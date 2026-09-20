"""把 ratchet 的交付产物翻译成 ASAS-A 凭据。

设计上只做"翻译"，不做判断：判定逻辑在 Go 侧（policy draft），校验逻辑在 asas.py。
这里唯一带判断的地方是一条**诚实规则**：

    凡是这台机器上拿不到的事实（没有哈希、不确定是否定版、枚举失败），
    一律写进 ``unknown[]``，**绝不填一个看起来合理的默认值**。

原因见 spec/ASAS-v0.1.md §2：隐瞒未知等于审计欺诈。第一版凭据里 unknown 可能很长，
那不是缺陷——那是这份凭据在说实话。
"""

from __future__ import annotations

import hashlib
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

from . import asas

DEFAULT_VALIDITY_DAYS = 30


def _delegation_block(
    delegation: dict | None, verdicts: list[dict], agent_id: str, period_to
) -> dict:
    """把"这个 agent 是替谁干活的"写进凭据（ASAS-2.4/2.5）。

    作用域 = 它**实际**被授予的资产，不是调用方声称的一个列表：
    要拿去和父比的是实际拿到的东西；声明一个比实际更小的集合，验证会通过，
    但凭据就在撒谎——那正是这份标准要防的事。

    没给到期时间时取 ``min(本凭据有效期, 父的到期)``。**父的到期必须参与**：
    只取自己的有效期会让父子两份凭据相隔几秒生成时出现"子比父多活 3 秒"，
    于是默认生成的委派就是违规的——本机实测踩过。默认值不该生产出失败。
    """
    if not delegation:
        return {}
    parent = str(delegation.get("from") or "").strip()
    if not parent:
        return {}
    granted = sorted(
        {
            str(v.get("asset"))
            for v in verdicts
            if v.get("agent") == agent_id and v.get("decision") in ("allow", "approve") and v.get("asset")
        }
    )
    if not granted:
        # schema 要求 scope 至少一项。一个什么都没被授予的 agent 没有可继承的权限，
        # 写一条空委派既不合 schema，也没有信息量——如实不写。
        return {}
    return {
        "delegation": {
            "from": parent,
            "scope": granted,
            "expires": str(delegation.get("expires") or _default_expiry(delegation, period_to)),
        }
    }


def _default_expiry(delegation: dict, period_to) -> str:
    """默认到期 = 自己有效期与父到期的较早者。"""
    bound = delegation.get("parentNotAfter")
    if bound:
        try:
            parent_to = datetime.fromisoformat(str(bound).replace("Z", "+00:00"))
        except ValueError:
            parent_to = None
        if parent_to is not None:
            if parent_to.tzinfo is None:
                parent_to = parent_to.replace(tzinfo=timezone.utc)
            return min(period_to, parent_to).isoformat()
    return period_to.isoformat()


def _hash_unknown_reason(ref: str, hasher_supplied: bool) -> str:
    """没算出资质哈希时，``unknown[].why`` 该怎么写。

    "没试"和"试了没成"是两种事实：前者是我们的选择（离线 / 预算用尽），
    后者是环境不给（包不存在、网络不通）。凭据里混成一句"取不到"，
    审计的人就没法区分"这份凭据的能力边界"和"这次运气不好"。
    """
    from . import artifacts

    reason = artifacts.skip_reason() if hasher_supplied else None
    if reason == "offline":
        return f"已设为离线（RATCHET_OFFLINE=1），未出网取 {ref} 的制品；据此声明 unknown"
    if reason == "budget":
        return f"出网预算已用尽（RATCHET_HTTP_BUDGET 秒），未取 {ref} 的制品；据此声明 unknown"
    return f"取不到 {ref} 的制品（离线或包不存在），未计算哈希"


def _sha256_text(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode()).hexdigest()


def _sorted_union(*groups) -> list[str]:
    out: set[str] = set()
    for group in groups:
        for item in group or []:
            if item:
                out.add(str(item))
    return sorted(out)


def _server_facts(inventory: dict | list | None) -> dict[str, dict]:
    """从扫描产物里取 server 级事实。

    接受两种形状：完整 inventory（带 ``servers`` 字段）或裸的 facts 数组
    （``ratchet deliver`` 落盘的就是后者）。**只取元数据**：名字、被声明的包引用、定版状态。
    环境变量的值从来不在这条链路上。
    """
    if inventory is None:
        return {}
    items = inventory.get("servers") if isinstance(inventory, dict) else inventory
    out: dict[str, dict] = {}
    for item in items or []:
        if isinstance(item, dict) and item.get("name"):
            out[str(item["name"])] = item
    return out


def build_attestation(
    policy: dict,
    *,
    org: str,
    environment: str | None = None,
    scope: list[str] | None = None,
    owner: str = "unassigned",
    runtime_kind: str | None = None,
    inventory: dict | None = None,
    evidence: list[tuple[str, bytes]] | None = None,
    generated_at: datetime | None = None,
    validity_days: int = DEFAULT_VALIDITY_DAYS,
    exemptions: dict | None = None,
    artifact_hasher=None,
    delegation: dict | None = None,
) -> dict:
    """policy.json（+ 可选的 inventory / 证据文件）→ ASAS-A 凭据。

    ``inventory`` 是 ``ratchet scan --introspect --out`` 的产物；没有它时工具面就是未知，
    会被显式声明而不是猜。

    ``delegation`` = ``{"from": 父 agent id, "expires": ISO 时间}``：这份凭据代表的
    agent 是**替别人干活**的（ASAS-2.4/2.5）。作用域取它实际被授予的资产，
    因为"我实际拿了什么"才是要拿去和父比的东西——写成"我声称我只拿了什么"没有意义。
    """
    moment = generated_at or datetime.now(timezone.utc)
    agent_id = str(policy.get("agent") or "agent")
    servers: dict = policy.get("servers") or {}
    rationale: dict = policy.get("rationale") or {}

    # 工具面：从 policy 里取（allow/approve/deny 的并集）
    tools_by_server: dict[str, set[str]] = {}
    for server, buckets in servers.items():
        tools_by_server.setdefault(str(server), set())
        for bucket in ("allow", "approve", "deny"):
            for tool in (buckets or {}).get(bucket) or []:
                tools_by_server[str(server)].add(str(tool))

    # 枚举结果（有 inventory 才能说"这个 server 有 N 个工具"）
    reachable: dict[str, int] = {}
    # inventory 有两种形状：完整 inventory（dict，带 tools/servers）或裸 facts 数组（list）。
    tools = inventory.get("tools") if isinstance(inventory, dict) else None
    for item in (tools or []):
        server = str(item.get("server") or "")
        if server:
            reachable[server] = reachable.get(server, 0) + 1

    facts = _server_facts(inventory)

    unknown: list[dict] = []
    assets: list[dict] = []
    exemptions = exemptions or {}
    # artifact_hasher 默认是 None = **不联网**。取制品哈希要出网，因此必须由调用方
    # 显式开启（CLI 会传真实的 hash_npm_package）——库的默认行为不该偷偷发请求，
    # 否则单元测试会变成网络测试，离线环境也无法使用。
    for server in sorted(tools_by_server):
        tools = sorted(tools_by_server[server])
        # 制品哈希：能算就算（ASAS-3.2）；算不出来就如实声明，不填假值。
        ref = str(facts.get(server, {}).get("ref") or "")
        resolved_version, content_hash = None, None
        content_unknown = "本机扫描拿不到制品本体，无法计算产物哈希（不填假值）"
        if ref:
            got = artifact_hasher(ref) if artifact_hasher else None
            if got:
                resolved_version, content_hash = got
            else:
                content_unknown = _hash_unknown_reason(ref, artifact_hasher is not None)
        # 描述哈希取自我们真正看到的东西：该 server 下工具名与依据的规范文本。
        # 它不是产物哈希——产物哈希需要制品本体，本机扫描拿不到。
        descriptor = json.dumps(
            {"server": server, "tools": tools, "rationale": {f"{server}/{t}": rationale.get(f"{server}/{t}", "") for t in tools}},
            sort_keys=True,
            ensure_ascii=False,
        )
        assets.append(
            {
                "kind": "server",
                "name": server,
                # 定版状态与版本引用是**配置里看得到的事实**，能拿到就必须写进来，
                # 不能一律退化成 unknown——那会浪费这份凭据的信息量。
                # 优先写**解析到的真实版本**（比配置里写的引用更有信息量）；
                # 拿不到时退回被声明的引用，再拿不到才是 null。
                "version": resolved_version or (facts.get(server, {}).get("ref") or None),
                # 只在真有引用时输出：schema 里它是可选的字符串，
                # 塞一个 null 进去等于用一个"空值"表达"没有"，没必要。
                **({"declaredRef": ref} if ref else {}),
                "pinned": facts.get(server, {}).get("pinned"),
                # 书面豁免写进凭据本身（ASAS-3.1）
                **({"exemptUntil": exemptions[server]} if server in exemptions else {}),
                "contentHash": content_hash,
                "descHash": _sha256_text(descriptor),
                "reachableTools": reachable.get(server),
            }
        )
        unknown.append({
            "what": f"{server}: contentHash",
            "why": content_unknown,
            "discoveredAt": moment.isoformat(),
        }) if content_hash is None else None
        if facts.get(server, {}).get("pinned") is None:
            unknown.append({
                "what": f"{server}: pinned",
                "why": "这条定义没有包引用（例如远程 URL server），无法判断定版状态",
                "discoveredAt": moment.isoformat(),
            })
        if not facts.get(server, {}).get("ref"):
            unknown.append({
                "what": f"{server}: version",
                "why": "没有包引用可记录版本",
                "discoveredAt": moment.isoformat(),
            })
        if reachable.get(server) is None:
            unknown.append({
                "what": f"{server}: reachableTools",
                "why": "未运行 --introspect，工具面未知",
                "discoveredAt": moment.isoformat(),
            })

    verdicts = []
    for server, buckets in servers.items():
        for decision, key in (("allow", "allow"), ("approve", "approve"), ("deny", "deny")):
            for tool in (buckets or {}).get(key) or []:
                verdicts.append({
                    "agent": agent_id,
                    "asset": str(server),
                    "decision": decision,
                    "basis": str(rationale.get(f"{server}/{tool}") or "").strip() or "no basis recorded",
                })

    files = [(name, data) for name, data in (evidence or [])]
    manifest_evidence = [
        {"file": name, "sha256": hashlib.sha256(data).hexdigest()} for name, data in files
    ]
    if not manifest_evidence:
        # 没有证据文件时也必须给 manifest 一个条目，否则凭证本身就是空的
        manifest_evidence = [{"file": "policy.json", "sha256": hashlib.sha256(
            json.dumps(policy, sort_keys=True, ensure_ascii=False).encode()).hexdigest()}]

    period_to = moment + timedelta(days=validity_days)
    return {
        "asas": "0.1",
        "subject": {
            "org": org,
            **({"environment": environment} if environment else {}),
            "period": {
                "from": str(policy.get("generatedAt") or moment.isoformat()),
                "to": period_to.isoformat(),
            },
            **({"scope": scope} if scope else {}),
        },
        "agents": [{
            "id": agent_id,
            "owner": owner,
            "registered": True,
            "runtime": {"kind": runtime_kind or agent_id, "version": str(policy.get("generator") or "unknown")},
            "identity": {"type": "unknown", "shared": False},
            **_delegation_block(delegation, verdicts, agent_id, period_to),
        }],
        "assets": assets,
        "verdicts": verdicts,
        "unknown": unknown,
        "observations": [],
        # 我们是观察者，不拦调用：只能声明 readonly（ASAS-4.1 明确允许，且要求必须声明）
        "boundaries": {"mode": "readonly", "revocationMinutes": 0},
        "manifest": {
            "evidence": manifest_evidence,
            "generatedAt": moment.isoformat(),
            "generator": str(policy.get("generator") or "ratchet"),
        },
    }


def build_and_verify(policy: dict, **kwargs) -> tuple[dict, asas.Report]:
    """产出凭据并**立刻用同一套规则自查**——自己产的凭据先过自己的校验，不过就不算交付。

    `parents` / `containment` / `graph` / `events` 是**验证输入**（不参与构建），所以在这里摘出来：
    它们决定了自查能评到几条规则，不该被塞进 build_attestation 的参数里。
    """
    parents = kwargs.pop("parents", None)
    containment = kwargs.pop("containment", None)
    graph = kwargs.pop("graph", None)
    events = kwargs.pop("events", None)
    attestation = build_attestation(policy, **kwargs)
    evidence = kwargs.get("evidence") or []
    report = asas.verify(
        attestation,
        files={name: data for name, data in evidence} or None,
        parents=parents,
        containment=containment,
        graph=graph,
        events=events,
    )
    return attestation, report


def write(attestation: dict, path: str | Path) -> Path:
    out = Path(path)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(attestation, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return out
