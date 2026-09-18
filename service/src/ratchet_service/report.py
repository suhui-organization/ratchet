"""把策略渲染成一份可交付的报告。

这份报告要同时满足两类读者：

- **客户/老板**：一眼看懂"哪些能力被放行、哪些被收掉、为什么"；
- **审计方**：每条判定能追回依据，且知道这份报告**证明不了什么**。

所以固定包含四节：摘要、权限表、判定依据、覆盖边界。缺任何一节都会让
报告变成一份"看起来权威但其实什么也没说"的文档。
"""
from __future__ import annotations

from . import compliance


def _counts(servers: dict) -> tuple[int, int, int]:
    allow = approve = deny = 0
    for rules in servers.values():
        allow += len(rules.get("allow") or [])
        approve += len(rules.get("approve") or [])
        deny += len(rules.get("deny") or [])
    return allow, approve, deny


def render_report(policy: dict, *, generated_at: str = "", note: str = "") -> str:
    servers = policy.get("servers") or {}
    allow, approve, deny = _counts(servers)
    total = allow + approve + deny
    needs_review = policy.get("needsReview") or []
    rationale = policy.get("rationale") or {}

    lines: list[str] = []
    lines.append("# Agent 权限收敛报告")
    lines.append("")
    lines.append(f"- 生成时间：{generated_at or policy.get('generatedAt') or '—'}")
    lines.append(f"- agent：{policy.get('agent') or '—'}")
    lines.append(f"- 默认决策：`{policy.get('defaultDecision', 'deny')}`（未登记的一律拒绝）")
    if note:
        lines.append(f"- 说明：{note}")
    lines.append("")

    lines.append("## 1. 摘要")
    lines.append("")
    lines.append(
        f"本机共梳理 **{len(servers)}** 个 server / **{total}** 个工具："
        f"放行 **{allow}**、需人工审批 **{approve}**、拒绝 **{deny}**。"
    )
    lines.append("")
    if needs_review:
        lines.append(
            f"其中 **{len(needs_review)}** 个工具无法从名称与描述判定能力，"
            "已置为「需人工审批」并列入下方待确认——**请先核对这一批再用**。"
        )
        lines.append("")

    lines.append("## 2. 权限表")
    lines.append("")
    lines.append("| server | allow | approve | deny |")
    lines.append("|---|---|---|---|")
    for name in sorted(servers):
        rules = servers[name]
        fmt = lambda key: "、".join(f"`{t}`" for t in (rules.get(key) or [])) or "—"
        lines.append(f"| `{name}` | {fmt('allow')} | {fmt('approve')} | {fmt('deny')} |")
    lines.append("")

    if rationale:
        lines.append("## 3. 判定依据")
        lines.append("")
        lines.append("每条判定为什么这么判——没有这一节，读者无法判断这份报告是否可信：")
        lines.append("")
        lines.append("| 工具 | 判定依据 |")
        lines.append("|---|---|")
        for key in sorted(rationale):
            if key in needs_review:
                continue
            lines.append(f"| `{key}` | {rationale[key]} |")
        lines.append("")
        if needs_review:
            lines.append("**待确认（依据不足，需人工判断）**")
            lines.append("")
            for key in needs_review:
                lines.append(f"- `{key}`：{rationale.get(key, '—')}")
            lines.append("")

    lines.append("## 4. 覆盖边界（这份报告不能证明什么）")
    lines.append("")
    lines.append("- 只覆盖被扫描到的配置；扫描不到的 agent 不在其中（扫描结果里会列出未解析的配置）。")
    lines.append("- 未观测到调用的工具仍可能被使用——「未观测」不等于「没发生」。")
    lines.append("- 本报告不构成安全评估结论，也不代表系统已通过任何合规认证。")
    lines.append("")

    lines.append(compliance.render_markdown())
    lines.append("")
    return "\n".join(lines)
