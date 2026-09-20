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
            problems.append(f"{agent.get('id')}: 委派来源 {parent_id!r} 不在凭据里（未知委派链）")
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
        body = json.dumps({k: v for k, v in event.items() if k != "hash"}, sort_keys=True, separators=(",", ":"))
        computed = _sha256_bytes(body.encode())
        if event.get("hash") and event["hash"] != computed:
            problems.append(f"#{index}: 自身哈希不符")
        prev_hash = event.get("hash") or computed
        last_seq = seq
    return RuleResult("silenceIsAuditable", FAIL if problems else PASS, problems)


def verify(
    attestation: dict,
    *,
    files: dict[str, bytes] | None = None,
    events: list[dict] | None = None,
    now: datetime | None = None,
) -> Report:
    """验证一份 ASAS-A 凭据。三道输入都是独立可选的：

    - ``files``  证据文件（不给则 V1 未评估）
    - ``events`` 事件流（不给则 V6 未评估）
    """
    moment = now or datetime.now(timezone.utc)
    return Report(
        [
            rule_hash_match(attestation, files),
            rule_narrowing_only(attestation, moment),
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
