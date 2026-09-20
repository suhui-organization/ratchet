"""ASAS-V：ASAS v0.1 的离线验证器。

规则定义见 spec/ASAS-v0.1.md §6。这里只做一件事：**在不联网、不联系签发方的条件下**，
判断一份 ASAS-A 凭据是否合规。

三个刻意的设计决定：

1. **只用标准库**。验证器如果自己需要装一堆依赖，收货方就不会去验——那就违背了
   ASAS-6.2（第三方离线、零安装可验证）。json/hashlib 就够了。
2. **三态而不是布尔**：pass / fail / not_evaluated。有些规则需要输入（比如 V1 需要证据
   文件、V6 需要事件流），拿不到输入时必须报"未评估"，**不能假装通过**——
   一个会把"没检查"说成"通过"的验证器，比没有验证器更糟。
3. **V2 的"收窄"是集合包含**：子 agent 的作用域必须是父 agent 作用域的子集。父不在凭据里
   = 未知委派链 = 不合规（ASAS-2.5）。
"""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

PASS = "pass"
FAIL = "fail"
NOT_EVALUATED = "not_evaluated"

# basis 里出现这些，等于没写依据（ASAS-2.2）
_EMPTY_BASIS = {"", "-", "n/a", "na", "none", "todo"}


@dataclass
class RuleResult:
    rule: str
    status: str
    details: list[str] = field(default_factory=list)

    def as_dict(self) -> dict:
        return {"rule": self.rule, "status": self.status, "details": self.details}


@dataclass
class Report:
    results: list[RuleResult]

    @property
    def ok(self) -> bool:
        """合规 = 没有任何一条 fail。未评估不算通过，但也不单独判失败。"""
        return all(r.status != FAIL for r in self.results)

    @property
    def evaluated(self) -> list[str]:
        return [r.rule for r in self.results if r.status != NOT_EVALUATED]

    @property
    def not_evaluated(self) -> list[str]:
        return [r.rule for r in self.results if r.status == NOT_EVALUATED]

    def as_dict(self) -> dict:
        return {
            "ok": self.ok,
            "evaluated": self.evaluated,
            "notEvaluated": self.not_evaluated,
            "results": [r.as_dict() for r in self.results],
        }


# ── V7 凭据在有效期内 ──────────────────────────────────────────────────────────
def rule_within_validity_window(att: dict, now: datetime) -> RuleResult:
    """ASAS-6.5：过期凭据视为无效。

    没有这一条，"变更触发重证"就无从落地——凭据只要能一直拿出来用，
    就没有任何人会去重新出证。
    """
    period = ((att.get("subject") or {}).get("period") or {})
    expires = _parse_ts(str(period.get("to") or ""))
    if expires is None:
        return RuleResult("withinValidityWindow", FAIL, ["凭据没有可解析的有效期（subject.period.to）"])
    if expires <= now:
        return RuleResult("withinValidityWindow", FAIL,
                          [f"凭据已于 {period.get('to')} 过期：必须重新出证"])
    return RuleResult("withinValidityWindow", PASS, [])


def _parse_ts(value: str) -> datetime | None:
    if not value:
        return None
    try:
        text = value.replace("Z", "+00:00")
        dt = datetime.fromisoformat(text)
    except ValueError:
        return None
    return dt if dt.tzinfo else dt.replace(tzinfo=timezone.utc)


def _sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


# ── V1 哈希一致 ────────────────────────────────────────────────────────────────
def rule_hash_match(att: dict, files: dict[str, bytes] | None) -> RuleResult:
    evidence = (att.get("manifest") or {}).get("evidence") or []
    if files is None:
        return RuleResult("hashMatch", NOT_EVALUATED, ["没有提供证据文件，无法重算哈希"])
    bad: list[str] = []
    for item in evidence:
        name, expected = item.get("file"), (item.get("sha256") or "").lower()
        if name not in files:
            bad.append(f"{name}: 文件缺失")
            continue
        actual = _sha256_bytes(files[name])
        if actual != expected:
            bad.append(f"{name}: 期望 {expected[:12]}… 实际 {actual[:12]}…")
    return RuleResult("hashMatch", FAIL if bad else PASS, bad)


# ── V2 委派只可收窄 ────────────────────────────────────────────────────────────
def _scope_of(att: dict, agent_id: str) -> set[str] | None:
    """父 agent 的作用域 = 它在 verdicts 里被授予（allow/approve）的资产名。"""
    granted = {
        v.get("asset")
        for v in att.get("verdicts") or []
        if v.get("agent") == agent_id and v.get("decision") in ("allow", "approve")
    }
    if not granted:
        return None
    return {a for a in granted if a}


def rule_narrowing_only(att: dict, now: datetime) -> RuleResult:
    agents = {a.get("id"): a for a in att.get("agents") or []}
    problems: list[str] = []
    for agent in att.get("agents") or []:
        delegation = agent.get("delegation")
        if not delegation:
            continue
        parent_id = delegation.get("from")
        child_scope = {s for s in (delegation.get("scope") or []) if s}
        if parent_id not in agents:
            # 父不在**同一份**凭据里：这不是缺陷，而是分工——跨凭据的委派归 V8
            # （它要拿父凭据来比）。V2 在这里判失败会把合法架构判成违规，
            # 也会让 V8 永远轮不到。
            continue
        parent_scope = _scope_of(att, parent_id)
        if parent_scope is None:
            problems.append(f"{agent.get('id')}: 父 {parent_id} 在凭据里没有任何被授予的资产，无法证明收窄")
            continue
        widened = child_scope - parent_scope
        if widened:
            problems.append(f"{agent.get('id')}: 委派放大 {sorted(widened)}（父作用域之外）")
        expires = _parse_ts(delegation.get("expires") or "")
        if expires is None:
            problems.append(f"{agent.get('id')}: 委派缺少可解析的到期时间")
        elif expires <= now:
            problems.append(f"{agent.get('id')}: 委派已过期（{delegation.get('expires')}）")
    return RuleResult("narrowingOnly", FAIL if problems else PASS, problems)


# ── V8 跨凭据的委派收窄 ────────────────────────────────────────────────────────
def _denied_of(att: dict, agent_id: str) -> set[str]:
    """某 agent 在这份凭据里被明确拒绝的资产（ASAS-2.4 的 deny 半边）。"""
    return {
        str(v.get("asset"))
        for v in att.get("verdicts") or []
        if v.get("agent") == agent_id and v.get("decision") == "deny" and v.get("asset")
    }


def rule_delegation_narrows(
    att: dict, parents: dict[str, dict] | None, now: datetime
) -> RuleResult:
    """V8：子 ≠ 父所在的那份凭据时，用**父的凭据**来证明收窄。

    V2 只能看一份凭据内部。真实世界里父子常是两个主体、两份凭据——只看子凭据，
    能证明的只是"它自己声称收窄了"。所以这里要三件事之一：

    * 父在本凭据内 → 交给 V2，本规则跳过（不重复判定）；
    * 父有凭据可用 → 逐条比：作用域 ⊆、deny ⊇、有效期不超出父；
    * 父凭据不可得 → **失败**（ASAS-2.5：未知委派链视为越权）。

    第三种是这条规则的立场：想用委派要权限，就得把父链条一起交出来。
    """
    own_agents = {a.get("id") for a in att.get("agents") or []}
    delegations = [
        (str(a.get("id") or ""), a.get("delegation") or {})
        for a in att.get("agents") or []
        if a.get("delegation")
    ]
    if not delegations:
        # 没有声明任何委派 = 输入已被读过、里面没有委派。这与"没有输入"不同，
        # 所以是 pass 而不是 not_evaluated（和 unknown[] 为空时的处理一致）。
        return RuleResult("delegationNarrows", PASS, [])

    parents = parents or {}
    problems: list[str] = []
    for child, delegation in delegations:
        parent_id = str(delegation.get("from") or "")
        if parent_id in own_agents:
            continue  # V2 的职责
        parent_att = parents.get(parent_id)
        if parent_att is None:
            problems.append(
                f"{child}: 父 {parent_id!r} 的凭据不可得——未知委派链视为越权（ASAS-2.5）"
            )
            continue
        problems.extend(_compare_delegation(att, child, parent_id, delegation, parent_att, now))
    return RuleResult("delegationNarrows", FAIL if problems else PASS, problems)


def _compare_delegation(
    child_att: dict,
    child: str,
    parent_id: str,
    delegation: dict,
    parent_att: dict,
    now: datetime,
) -> list[str]:
    problems: list[str] = []
    child_scope = {str(s) for s in (delegation.get("scope") or []) if s}
    parent_scope = _scope_of(parent_att, parent_id) or set()
    widened = sorted(child_scope - parent_scope)
    if widened:
        problems.append(f"{child}: 委派放大 {widened}（不在父 {parent_id} 的作用域内）")

    # deny 只增不减：父凭据里写下来的拒绝，子凭据必须也写着。
    # 注意这不是"有没有被挡住"的问题（默认就是 deny），而是**可见性**：
    # 丢掉一条，审阅子的凭据的人就再也看不到这条限制，只看到一片沉默。
    dropped = sorted(_denied_of(parent_att, parent_id) - _denied_of(child_att, child))
    if dropped:
        problems.append(
            f"{child}: 丢掉了父 {parent_id} 的拒绝项 {dropped}（deny 只增不减，ASAS-2.4）"
        )

    parent_to = _parse_ts(str((parent_att.get("subject") or {}).get("period", {}).get("to") or ""))
    expires = _parse_ts(str(delegation.get("expires") or ""))
    if parent_to is None:
        problems.append(f"{child}: 父 {parent_id} 的凭据没有可解析的有效期，无法证明子不超出父")
    elif parent_to <= now:
        problems.append(f"{child}: 父 {parent_id} 的凭据已过期（{parent_to.isoformat()}），继承来的权限随之失效")
    elif expires is None:
        problems.append(f"{child}: 委派缺少可解析的到期时间")
    elif expires > parent_to:
        problems.append(
            f"{child}: 委派到期 {expires.isoformat()} 晚于父凭据有效期 {parent_to.isoformat()}——子不得比父活得久"
        )
    return problems


# ── V9 遏制沿委派链级联 ────────────────────────────────────────────────────────
def rule_containment_cascades(
    att: dict,
    containment: list[dict] | None,
    graph: list[dict] | None,
    parents: dict[str, dict] | None,
) -> RuleResult:
    """V9：被遏制的 agent，它的下级不能还留着继承来的权限（ASAS-8.5）。

    这条规则检查的是**台账层面**的性质，不是单份凭据的性质——"吊销有没有往下走"
    要看整个委派图。所以它有两份输入：遏制记录 + 委派边。

    **图完整不完整必须说清**：

    * `graph` 给全了（控制面把台账摊开、或导出文件里带 `graph`）→ 某个被遏制的
      agent 没有下级，就是"真的没有下级"，判 pass；
    * `graph` 没给（只有手上这几份凭据）→ 看不见下级**不等于**没有下级。
      这种情况下如果记录里有遏制对象，就如实报 not_evaluated。

    这一条不是洁癖：把"我没看见"当成"不存在"，正是被遏制的 agent 还能继续跑的原因。
    """
    if containment is None:
        return RuleResult("containmentCascades", NOT_EVALUATED, ["没有提供遏制记录，无法检查级联"])

    from .graph import delegation_edges

    complete = graph is not None
    if graph is None:
        sources = [(att, "self")] + [(p, "parent") for p in (parents or {}).values()]
        graph = delegation_edges(sources)

    org = str((att.get("subject") or {}).get("org") or "")
    org_edges = [e for e in graph if e["org"] in (org, "")]
    contained = {str(r.get("agent") or "") for r in containment}
    problems: list[str] = []
    unseen: list[str] = []
    for agent in sorted(contained):
        if not agent:
            continue
        children = [e for e in org_edges if e["parent"] == agent]
        for edge in children:
            if edge["child"] not in contained:
                problems.append(
                    f"{edge['child']}: 父 {agent} 已被遏制，但下级仍在（既不遏制也不放行——按 ASAS-8.5 视为未遏制）"
                )
        if not children and not complete:
            unseen.append(agent)
    if problems:
        return RuleResult("containmentCascades", FAIL, problems)
    if unseen:
        return RuleResult(
            "containmentCascades",
            NOT_EVALUATED,
            [f"{a}: 手上这份委派图不完整，无法确认它有没有下级（看不见 ≠ 没有）" for a in unseen],
        )
    return RuleResult("containmentCascades", PASS, [])


# ── V3 未知必须声明 ────────────────────────────────────────────────────────────
def rule_no_undeclared_unknown(att: dict) -> RuleResult:
    """枚举失败（没有工具面）或描述状态未知（没有 descHash）MUST 出现在 unknown[] 里。

    这是 ASAS-3.5 的可执行版本：拿不到的东西不写出来，等同于宣称自己看全了。
    """
    declared = " ".join(
        str(u.get("what", "")) + " " + str(u.get("why", "")) for u in att.get("unknown") or []
    )
    for item in att.get("unknown") or []:
        if not str(item.get("why") or "").strip():
            return RuleResult("noUndeclaredUnknown", FAIL, [f"unknown 条目缺少 why：{item.get('what')!r}"])
    problems: list[str] = []
    for asset in att.get("assets") or []:
        name = str(asset.get("name") or "")
        if asset.get("kind") in ("server", "skill"):
            if asset.get("reachableTools") is None and name not in declared:
                problems.append(f"{name}: 工具面未枚举，但未在 unknown[] 声明")
            if not asset.get("descHash") and name not in declared:
                problems.append(f"{name}: 缺少描述哈希（描述状态未知），但未在 unknown[] 声明")
        # contentHash 允许为 null，但"没算出哈希"同样是未知，必须声明（ASAS-3.2 + 3.5）
        if not asset.get("contentHash") and name not in declared:
            problems.append(f"{name}: 没有内容哈希（无法确定产物状态），但未在 unknown[] 声明")
        # pinned 允许为 null，但"不知道是否定版"必须声明，否则等于默认声称已定版
        if not isinstance(asset.get("pinned"), bool) and name not in declared:
            problems.append(f"{name}: 定版状态未知，但未在 unknown[] 声明")
        if not asset.get("version") and name not in declared:
            problems.append(f"{name}: 版本未知，但未在 unknown[] 声明")
    return RuleResult("noUndeclaredUnknown", FAIL if problems else PASS, problems)


# ── V4 每条判定都有依据 ────────────────────────────────────────────────────────
def rule_every_verdict_has_basis(att: dict) -> RuleResult:
    problems: list[str] = []
    for verdict in att.get("verdicts") or []:
        basis = str(verdict.get("basis") or "").strip().lower()
        if basis in _EMPTY_BASIS:
            problems.append(
                f"{verdict.get('agent')} / {verdict.get('asset')}: 判定为 {verdict.get('decision')} 但没有依据"
            )
    return RuleResult("everyVerdictHasBasis", FAIL if problems else PASS, problems)


# ── V5 定版或有未过期豁免 ──────────────────────────────────────────────────────
def rule_all_pinned_or_exempt(att: dict, now: datetime) -> RuleResult:
    problems: list[str] = []
    for asset in att.get("assets") or []:
        pinned = asset.get("pinned")
        if pinned is True:
            continue
        if pinned is None:
            # "不知道是否定版"由 V3 负责（必须在 unknown[] 里声明）；
            # 这里只处理**已知未定版**，那种情况必须有未过期的书面豁免。
            continue
        name = asset.get("name")
        exempt_until = _parse_ts(asset.get("exemptUntil") or "")
        if exempt_until is None:
            problems.append(f"{name}: 未定版且没有豁免期限")
        elif exempt_until <= now:
            problems.append(f"{name}: 豁免已过期（{asset.get('exemptUntil')}）")
    return RuleResult("allPinnedOrExempt", FAIL if problems else PASS, problems)


# ── V6 沉默可被审计 ────────────────────────────────────────────────────────────
def rule_silence_auditable(events: list[dict] | None) -> RuleResult:
    """事件流：序号单调 + 前序哈希相符。

    没有这一条，整个监控层不可证伪——谁能改传感器，谁就能伪造"一切正常"。
    """
    if events is None:
        return RuleResult("silenceIsAuditable", NOT_EVALUATED, ["没有提供事件流，无法检查断流"])
    problems: list[str] = []
    prev_hash = ""
    last_seq = -1
    for index, event in enumerate(events):
        seq = event.get("seq")
        if not isinstance(seq, int):
            problems.append(f"#{index}: seq 不是整数")
            continue
        if seq != last_seq + 1:
            problems.append(f"#{index}: 序号断裂（期望 {last_seq + 1}，实际 {seq}）")
        if event.get("prevHash", "") != prev_hash:
            problems.append(f"#{index}: 前序哈希不符")
        computed = event_digest(event)
        if event.get("hash") and event["hash"] != computed:
            problems.append(f"#{index}: 自身哈希不符")
        prev_hash = event.get("hash") or computed
        last_seq = seq
    return RuleResult("silenceIsAuditable", FAIL if problems else PASS, problems)


def event_digest(event: dict) -> str:
    """一条事件的哈希：去掉 `hash` 字段后按键排序序列化，再取 sha256。

    **这个口径必须只有一份**，因为有两个地方在用：

    * 这条规则（收货方重算，检查事件有没有被改）；
    * 控制面跨次比对（把**重算值**与上次存下的重算值比，而不是比事件里自称的 hash）。

    差别很要紧：只比自称的 hash，改内容、留着旧 hash 就骗过去了——
    `test_editing_a_middle_event_is_caught_too` 说的就是这种情况。
    """
    body = json.dumps(
        {k: v for k, v in event.items() if k != "hash"}, sort_keys=True, separators=(",", ":")
    )
    return _sha256_bytes(body.encode())


def verify(
    attestation: dict,
    *,
    files: dict[str, bytes] | None = None,
    events: list[dict] | None = None,
    parents: dict[str, dict] | None = None,
    containment: list[dict] | None = None,
    graph: list[dict] | None = None,
    now: datetime | None = None,
) -> Report:
    """验证一份 ASAS-A 凭据。五道输入都是独立可选的：

    - ``files``       证据文件（不给则 V1 未评估）
    - ``events``      事件流（不给则 V6 未评估）
    - ``parents``     委派父凭据 ``{agent id: 凭据}``（不给则 V8 报"未知委派链"）
    - ``containment`` 遏制记录（不给则 V9 未评估）
    - ``graph``       委派边全集（不给则 V9 只能用手上这几份凭据拼一张残图，
                      并且会如实说明"看不见 ≠ 没有"）

    "不给"和"给了但是空的"是两件事：前者未评估，后者是"查过了，没有"。
    """
    moment = now or datetime.now(timezone.utc)
    return Report(
        [
            rule_hash_match(attestation, files),
            rule_narrowing_only(attestation, moment),
            rule_delegation_narrows(attestation, parents, moment),
            rule_containment_cascades(attestation, containment, graph, parents),
            rule_no_undeclared_unknown(attestation),
            rule_every_verdict_has_basis(attestation),
            rule_all_pinned_or_exempt(attestation, moment),
            rule_silence_auditable(events),
            rule_within_validity_window(attestation, moment),
        ]
    )


def load_files(directory: str | Path) -> dict[str, bytes]:
    """把证据目录读成 ``{文件名: 字节}``，文件名与 manifest 中的 file 字段对应。"""
    root = Path(directory)
    return {p.name: p.read_bytes() for p in sorted(root.rglob("*")) if p.is_file()}
