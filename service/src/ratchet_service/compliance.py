"""合规条款映射。

把交付物里的证据逐条对应到条款要求。**它是索引，不是法律意见，也不构成合规声明。**

写作纪律：每一条都必须写"不能证明什么"。合规场景里，"看起来全覆盖"
比"写清楚边界"危险得多——后者才是审计方能用的材料。

措辞按语言放在 i18n 词表里（同一个条款在不同语言下必须说同样的事，
不能各写一版，否则中英两份报告会给出不同的承诺）。
"""
from __future__ import annotations

from .i18n import resolve_locale, t

# 行的顺序即阅读顺序：先讲证据本身（Art.12），再讲人的介入，最后是文档与体系
ROWS = ("art12", "art14", "art11", "art15", "art17", "iso")
FIELDS = ("req", "art", "proves", "cannot")


def render_markdown(locale: str | None = None) -> str:
    loc = resolve_locale(locale)
    lines = [t("compliance.heading", loc), ""]
    lines.append(t("compliance.disclaimer", loc))
    lines.append("")
    lines.append(t("compliance.table_head", loc))
    lines.append("|---|---|---|---|")
    for row in ROWS:
        cells = [t(f"compliance.{row}.{f}", loc) for f in FIELDS]
        lines.append("| " + " | ".join(cells) + " |")
    lines.append("")
    lines.append(t("compliance.closing", loc))
    return "\n".join(lines)


def rows(locale: str | None = None) -> list[tuple[str, str, str, str]]:
    """结构化取用（测试与将来的图形化报告都用它，不必解析 Markdown）。"""
    loc = resolve_locale(locale)
    return [tuple(t(f"compliance.{r}.{f}", loc) for f in FIELDS) for r in ROWS]
