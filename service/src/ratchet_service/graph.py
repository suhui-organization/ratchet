"""委派图：谁把权限给了谁。

这份图有**两个消费者**，所以必须只有一份实现：

* `asas.rule_containment_cascades`（V9）——判断遏制有没有沿委派链往下走；
* 控制面的遏制编排与 `哪些 agent 曾能触达 X`（ASAS-8.3/8.5）——决定吊销谁、回答谁。

如果两边各写一遍"父子关系"，代价不是重复代码，而是**两套答案**：吊销时算一套、
验证时算另一套，出事那天没人知道哪个对。

边的定义：子 agent 的 `delegation.from` 指向父 agent，且两者在同一份凭据里
（`subject.org` + agent id）。跨组织的委派不构成继承，不能用来放权。
"""

from __future__ import annotations

from collections import defaultdict


def delegation_edges(attestations: list[tuple[dict, str]]) -> list[dict]:
    """把一叠凭据摊成委派边：``[{parent, child, org, expires, scope, attestationId}]``。

    ``attestationId`` 用调用方给的 `_attestation_id` 结果；这里不认识存储层。
    """
    edges: list[dict] = []
    for att, ident in attestations:
        org = str((att.get("subject") or {}).get("org") or "")
        known = {str(a.get("id")) for a in att.get("agents") or []}
        for agent in att.get("agents") or []:
            delegation = agent.get("delegation") or {}
            parent = str(delegation.get("from") or "")
            if not parent:
                continue
            edges.append(
                {
                    "parent": parent,
                    "child": str(agent.get("id") or ""),
                    "org": org,
                    # 父不在同一份凭据里时，这条边是**声明**而不是已证明的关系；
                    # 标记出来，免得查询把声明当成事实。
                    "parentInDocument": parent in known,
                    "scope": [str(s) for s in (delegation.get("scope") or [])],
                    "expires": str(delegation.get("expires") or ""),
                    "attestationId": ident,
                }
            )
    return edges


def descendants(agent: str, edges: list[dict]) -> list[dict]:
    """``agent`` 的所有下级（广度优先，含层级）。

    环必须能收敛：一份被篡改的凭据完全可以声明 A→B→A。这里用 visited 集合兜住，
    并且**把环本身当成要报告的事实**（`cycle: True`），而不是悄悄吞掉。
    """
    children: dict[str, list[dict]] = defaultdict(list)
    for edge in edges:
        children[edge["parent"]].append(edge)

    found: list[dict] = []
    visited = {agent}
    queue = [(agent, 1)]
    while queue:
        current, depth = queue.pop(0)
        for edge in children.get(current, []):
            child = edge["child"]
            if child in visited:
                found.append({**edge, "depth": depth, "cycle": True})
                continue
            visited.add(child)
            found.append({**edge, "depth": depth, "cycle": False})
            queue.append((child, depth + 1))
    return found


def chain_to(agent: str, edges: list[dict], *, max_depth: int = 8) -> list[dict]:
    """从根到 ``agent`` 的委派链（用于回答"这个权限是谁给的"）。

    有环或超过 ``max_depth`` 就停下并标记，不假装链是完整的。
    """
    parents: dict[str, dict] = {}
    for edge in edges:
        parents.setdefault(edge["child"], edge)

    chain: list[dict] = []
    seen = {agent}
    current = agent
    while len(chain) < max_depth:
        edge = parents.get(current)
        if edge is None:
            break
        if edge["parent"] in seen:
            chain.append({**edge, "cycle": True})
            break
        seen.add(edge["parent"])
        chain.append({**edge, "cycle": False})
        current = edge["parent"]
    chain.reverse()
    return chain
