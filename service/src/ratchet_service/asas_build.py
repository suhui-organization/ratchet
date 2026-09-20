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
) -> dict:
    """policy.json（+ 可选的 inventory / 证据文件）→ ASAS-A 凭据。

    ``inventory`` 是 ``ratchet scan --introspect --out`` 的产物；没有它时工具面就是未知，
    会被显式声明而不是猜。
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
                content_unknown = f"取不到 {ref} 的制品（离线或包不存在），未计算哈希"
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
    """产出凭据并**立刻用同一套规则自查**——自己产的凭据先过自己的校验，不过就不算交付。"""
    attestation = build_attestation(policy, **kwargs)
    evidence = kwargs.get("evidence") or []
    report = asas.verify(
        attestation,
        files={name: data for name, data in evidence} or None,
    )
    return attestation, report


def write(attestation: dict, path: str | Path) -> Path:
    out = Path(path)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(attestation, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return out
