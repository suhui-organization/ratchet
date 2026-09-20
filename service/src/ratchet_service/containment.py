"""遏制编排与"哪些 agent 曾能触达 X"（ASAS-8.3 / 8.5）。

两件事，都建立在同一张委派图上：

* **遏制级联**：吊销一个 agent 时，把它声明过的下级一起吊销。只吊销被点名的那一个，
  等于把最容易漏掉的那部分留着——子 agent 存在的意义之一就是绕过人工看管。
* **触达查询**：回答"谁**能**碰 X"（授权面）与"谁**被看到**碰过 X"（观测面）。
  两个都要给，因为二者不一致本身就是结论：能碰但从未用过（可回收的权限），
  或碰过但不在授权面里（有人绕过了配置）。

这里刻意不返回单一布尔值。审计问的是"依据是什么"，所以每条结果都带
`basis` 与它来自哪份凭据（`attestationId`）。
"""

from __future__ import annotations

from .graph import chain_to, delegation_edges, descendants


def graph_of(attestations: list[tuple[dict, str]]) -> list[dict]:
    return delegation_edges(attestations)


def plan_containment(agent: str, attestations: list[tuple[dict, str]]) -> list[dict]:
    """要吊销的名单：``agent`` 本身 + 它的全部下级（含孙，标注来自哪一条边）。

    返回的是**计划**，落库由调用方做。这样"吊销谁"这件事可以被单独测试、
    也可以在动手前先打印给人看（高危动作先看再执行）。
    """
    edges = graph_of(attestations)
    plan = [{"agent": agent, "via": "", "depth": 0, "cycle": False}]
    for item in descendants(agent, edges):
        plan.append(
            {
                "agent": item["child"],
                "via": item["parent"],
                "depth": item["depth"],
                "cycle": item["cycle"],
            }
        )
    return plan


def reach(subject: str, attestations: list[tuple[dict, str]], containment: list[dict]) -> dict:
    """谁曾能触达 ``subject``。

    ``subject`` 可以是资产名（server / tool）。判定口径：

    * ``granted``：某 agent 在 verdicts 里对 ``subject`` 有 allow/approve —— **能**碰；
    * ``observed``：observations 里出现该工具，或 verdict 的 basis 说明了观测次数 —— **碰过**；
    * ``contained``：该 agent 有遏制记录（含级联）。

    查不到不是"没有风险"，所以结果里带 ``unanswerable``：把"查不到"和"没有人能碰"
    分开写出来，是这份答案的一部分。
    """
    contained = {str(r.get("agent") or "") for r in containment}
    edges = graph_of(attestations)
    rows: list[dict] = []
    covered_orgs: set[str] = set()

    for att, ident in attestations:
        org = str((att.get("subject") or {}).get("org") or "")
        covered_orgs.add(org)
        verdicts = att.get("verdicts") or []
        for verdict in verdicts:
            if str(verdict.get("asset") or "") != subject:
                continue
            agent = str(verdict.get("agent") or "")
            decision = str(verdict.get("decision") or "")
            if decision not in ("allow", "approve", "deny"):
                continue
            rows.append(
                {
                    "org": org,
                    "agent": agent,
                    "verdict": decision,
                    "basis": str(verdict.get("basis") or ""),
                    "attestationId": ident,
                    "storedFor": str((att.get("subject") or {}).get("period", {}).get("to") or ""),
                    "contained": agent in contained,
                    "chain": [
                        {"parent": e["parent"], "child": e["child"], "expires": e["expires"]}
                        for e in chain_to(agent, edges)
                    ],
                }
            )

    observed = [
        {
            "org": str((att.get("subject") or {}).get("org") or ""),
            "tool": str(item.get("tool") or ""),
            "count": int(item.get("count") or 0),
            "attestationId": ident,
        }
        for att, ident in attestations
        for item in (att.get("observations") or [])
        if str(item.get("tool") or "") == subject
    ]

    answer = {
        "subject": subject,
        "granted": [r for r in rows if r["verdict"] in ("allow", "approve")],
        "denied": [r for r in rows if r["verdict"] == "deny"],
        "observed": observed,
        "contained": sorted(a for a in {r["agent"] for r in rows} if a in contained),
        "unanswerable": [],
    }
    if not rows:
        # 台账里没人提过这个 subject：既可能是"没人能碰"，也可能是"没有凭据覆盖它"。
        # 这两件事对应急响应是完全不同的结论，所以必须显式分开。
        answer["unanswerable"].append(
            f"台账里没有关于 {subject!r} 的任何判定——这**不**等于没人能碰它。"
            f"当前台账覆盖的组织：{sorted(covered_orgs) or '（空）'}"
        )
    return answer
