"""ASAS-3.4：比较两份 ASAS-A 凭据，说出**变了什么**。

为什么这是产品化的第一步：一份清单只回答"现在是什么"；每天再给一份清单，人不会看。
**"今天有什么变了"才是有人会看的那个东西**——而且它是唯一能让制品哈希产生价值的地方：
哈希本身没有意义，**两次哈希不一样**才有意义。

严重度不是拍脑袋定的，每条都对应一个真实攻击面：

- ``contentHash`` 变了 → **制品被替换**（供应链替换 / rug pull）→ critical
- ``descHash`` 变了 → 描述或工具面变了（工具投毒 / 影子工具）→ high
- ``pinned`` true→false → 从锁定退回浮动（下次启动就可能换东西）→ high
- ``version`` 变了且原本已定版 → 定版本该阻止这件事 → high
- 判定放宽（deny → approve/allow）→ 权限面变大 → high
- 新增资产 → 攻击面变大 → medium；新增 unknown → 覆盖变小 → medium
"""

from __future__ import annotations

CRITICAL, HIGH, MEDIUM, INFO = "critical", "high", "medium", "info"
_ORDER = {CRITICAL: 0, HIGH: 1, MEDIUM: 2, INFO: 3}


def _assets(att: dict) -> dict[str, dict]:
    return {str(a.get("name")): a for a in att.get("assets") or []}


def _verdicts(att: dict) -> dict[tuple, str]:
    return {
        (str(v.get("agent")), str(v.get("asset"))): str(v.get("decision"))
        for v in att.get("verdicts") or []
    }


def _change(kind, subject, before, after, severity, why):
    return {"kind": kind, "subject": subject, "before": before, "after": after,
            "severity": severity, "why": why}


def diff(old: dict, new: dict) -> list[dict]:
    """返回按严重度排序的变化列表。没有变化就是空列表——**空列表是好消息**。"""
    out: list[dict] = []
    old_assets, new_assets = _assets(old), _assets(new)

    for name in sorted(set(new_assets) - set(old_assets)):
        out.append(_change("asset_added", name, None, new_assets[name].get("version"),
                           MEDIUM, "新增可达组件：攻击面变大"))
    for name in sorted(set(old_assets) - set(new_assets)):
        out.append(_change("asset_removed", name, old_assets[name].get("version"), None,
                           INFO, "组件不再可达"))

    for name in sorted(set(old_assets) & set(new_assets)):
        o, n = old_assets[name], new_assets[name]
        if o.get("contentHash") and n.get("contentHash") and o["contentHash"] != n["contentHash"]:
            out.append(_change("content_changed", name, o["contentHash"][:22] + "…",
                               n["contentHash"][:22] + "…", CRITICAL,
                               "制品哈希变了：同一个名字解析到了不同的产物"))
        if o.get("descHash") and n.get("descHash") and o["descHash"] != n["descHash"]:
            out.append(_change("description_changed", name, o["descHash"][:22] + "…",
                               n["descHash"][:22] + "…", HIGH,
                               "描述或工具面变了：工具投毒/影子工具的观测点"))
        if o.get("version") != n.get("version"):
            # 消掉一类假阳性：上一轮只有引用（@latest），这一轮解析出了真实版本（1.9.0）——
            # 那是"解析出了版本"，不是"版本变了"。实测跑真实凭据时这类假变化占了一半。
            unresolved_before = o.get("version") == o.get("declaredRef")
            resolved_after = n.get("version") != n.get("declaredRef")
            if unresolved_before and resolved_after and o.get("declaredRef") == n.get("declaredRef"):
                out.append(_change("version_resolved", name, o.get("declaredRef"),
                                   n.get("version"), INFO,
                                   "从引用解析出真实版本：不是变化，是信息变多了"))
            else:
                severity = HIGH if o.get("pinned") else MEDIUM
                out.append(_change("version_changed", name, o.get("version"), n.get("version"),
                                   severity,
                                   "已定版组件不该变版本" if o.get("pinned") else "浮动引用的解析结果变了"))
        if o.get("pinned") is True and n.get("pinned") is False:
            out.append(_change("pin_removed", name, True, False, HIGH,
                               "从已定版退回未定版：下次启动可能换成别的产物"))
        if o.get("exemptUntil") != n.get("exemptUntil"):
            out.append(_change("exemption_changed", name, o.get("exemptUntil"),
                               n.get("exemptUntil"), INFO, "书面豁免变了"))

    old_v, new_v = _verdicts(old), _verdicts(new)
    rank = {"deny": 0, "approve": 1, "allow": 2}
    for key in sorted(set(new_v) - set(old_v)):
        out.append(_change("verdict_added", f"{key[0]}/{key[1]}", None, new_v[key],
                           MEDIUM, "新增授权"))
    for key in sorted(set(old_v) - set(new_v)):
        out.append(_change("verdict_removed", f"{key[0]}/{key[1]}", old_v[key], None,
                           INFO, "授权被撤销"))
    for key in sorted(set(old_v) & set(new_v)):
        if old_v[key] != new_v[key]:
            widened = rank.get(new_v[key], 9) > rank.get(old_v[key], 9)
            out.append(_change("verdict_changed", f"{key[0]}/{key[1]}", old_v[key], new_v[key],
                               HIGH if widened else INFO,
                               "判定放宽：权限面变大" if widened else "判定收紧"))

    old_unknown = {str(u.get("what")) for u in old.get("unknown") or []}
    new_unknown = {str(u.get("what")) for u in new.get("unknown") or []}
    for what in sorted(new_unknown - old_unknown):
        out.append(_change("unknown_added", what, None, None, MEDIUM,
                           "新增未知：凭据的覆盖面变小"))
    for what in sorted(old_unknown - new_unknown):
        out.append(_change("unknown_resolved", what, None, None, INFO, "未知已被确定"))

    return sorted(out, key=lambda c: (_ORDER.get(c["severity"], 9), c["subject"]))


def summarize(changes: list[dict]) -> dict:
    counts: dict[str, int] = {}
    for c in changes:
        counts[c["severity"]] = counts.get(c["severity"], 0) + 1
    return {"total": len(changes), "bySeverity": counts,
            "hasCritical": bool(counts.get(CRITICAL))}
