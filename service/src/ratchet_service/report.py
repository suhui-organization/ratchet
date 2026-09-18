"""把策略渲染成一份可交付的报告。

这份报告要同时满足两类读者：

- **客户/老板**：一眼看懂"哪些能力被放行、哪些被收掉、为什么"；
- **审计方**：每条判定能追回依据，且知道这份报告**证明不了什么**。

所以固定包含四节：摘要、权限表、判定依据、覆盖边界。缺任何一节都会让
报告变成一份"看起来权威但其实什么也没说"的文档。

语言：默认 en-US（产物是给客户看的）。文案在 i18n 词表里，这里不写死句子。
"""
from __future__ import annotations

from . import compliance
from .i18n import resolve_locale, t


def _counts(servers: dict) -> tuple[int, int, int]:
    allow = approve = deny = 0
    for rules in servers.values():
        allow += len(rules.get("allow") or [])
        approve += len(rules.get("approve") or [])
        deny += len(rules.get("deny") or [])
    return allow, approve, deny


def render_report(
    policy: dict, *, generated_at: str = "", note: str = "", locale: str | None = None
) -> str:
    loc = resolve_locale(locale)
    servers = policy.get("servers") or {}
    allow, approve, deny = _counts(servers)
    total = allow + approve + deny
    needs_review = policy.get("needsReview") or []
    rationale = policy.get("rationale") or {}

    lines: list[str] = [t("report.title", loc), ""]
    stamp = generated_at or policy.get("generatedAt") or "—"
    lines.append(t("report.meta.generated", loc, ts=stamp))
    lines.append(t("report.meta.agent", loc, agent=policy.get("agent") or "—"))
    lines.append(t("report.meta.default", loc, decision=policy.get("defaultDecision", "deny")))
    if note:
        lines.append(t("report.meta.note", loc, note=note))
    lines.append("")

    lines.append(t("report.summary.heading", loc))
    lines.append("")
    lines.append(t("report.summary.body", loc, servers=len(servers), tools=total,
                   allow=allow, approve=approve, deny=deny))
    lines.append("")
    if needs_review:
        lines.append(t("report.summary.review", loc, n=len(needs_review)))
        lines.append("")

    lines.append(t("report.table.heading", loc))
    lines.append("")
    lines.append("| server | allow | approve | deny |")
    lines.append("|---|---|---|---|")
    for name in sorted(servers):
        rules = servers[name]
        fmt = lambda key: ", ".join(f"`{x}`" for x in (rules.get(key) or [])) or "—"
        lines.append(f"| `{name}` | {fmt('allow')} | {fmt('approve')} | {fmt('deny')} |")
    lines.append("")

    if rationale:
        lines.append(t("report.rationale.heading", loc))
        lines.append("")
        lines.append(t("report.rationale.body", loc))
        lines.append("")
        lines.append(t("report.rationale.table_head", loc))
        lines.append("|---|---|")
        for key in sorted(rationale):
            if key in needs_review:
                continue
            lines.append(f"| `{key}` | {rationale[key]} |")
        lines.append("")
        if needs_review:
            lines.append(t("report.review.heading", loc))
            lines.append("")
            for key in needs_review:
                lines.append(f"- `{key}`: {rationale.get(key, '—')}")
            lines.append("")

    lines.append(t("report.coverage.heading", loc))
    lines.append("")
    for i in (1, 2, 3):
        lines.append(t(f"report.coverage.{i}", loc))
    lines.append("")

    lines.append(compliance.render_markdown(loc))
    lines.append("")
    return "\n".join(lines)
