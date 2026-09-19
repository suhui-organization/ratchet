"""产物语言的词表。

范围：只覆盖**产物**（报告与合规映射）。CLI 自身的终端输出不在其中——
终端是操作者看的，产物是客户看的，两者受众不同。

默认 en-US：站点与目标用户群是英文，给客户的东西不该是中文。
"""
from __future__ import annotations

import os

DEFAULT_LOCALE = "en-US"

CATALOG: dict[str, dict[str, str]] = {
    "en-US": {
        # ── 报告骨架 ──
        "report.title": "# Agent privilege report",
        "report.meta.generated": "- Generated at: {ts}",
        "report.meta.agent": "- Agent: {agent}",
        "report.meta.default": "- Default decision: `{decision}` (anything unlisted is denied)",
        "report.meta.note": "- Note: {note}",
        "report.summary.heading": "## 1. Summary",
        "report.summary.body": (
            "This machine has **{servers}** server(s) and **{tools}** tool(s): "
            "**{allow}** allowed, **{approve}** require approval, **{deny}** denied."
        ),
        "report.summary.review": (
            "**{n}** tool(s) could not be classified from their name or description. "
            "They are set to *require approval* and listed below — **review that batch before you rely on this policy**."
        ),
        "report.table.heading": "## 2. Policy table",
        "report.rationale.heading": "## 3. Why each verdict was made",
        "report.rationale.body": (
            "Without this section a reader cannot judge whether the report is trustworthy:"
        ),
        "report.rationale.table_head": "| Tool | Reason |",
        "report.review.heading": "**Needs review (not enough evidence to decide automatically)**",
        "report.coverage.heading": "## 4. Coverage boundary (what this report cannot prove)",
        "report.coverage.1": "- It only covers configurations that were scanned; agents that were not found are not in it.",
        "report.coverage.2": "- A tool that was never observed may still have been used — \"not observed\" does not mean \"did not happen\".",
        "report.coverage.3": "- This report is not a security assessment and does not mean any compliance requirement is met.",
        # ── 合规映射 ──
        "compliance.heading": "## Compliance mapping (EU AI Act / ISO 42001)",
        "compliance.disclaimer": (
            "This section maps the evidence in this delivery to specific provisions. "
            "**It is an index, not legal advice, and not a declaration of compliance** — "
            "use it together with your compliance lead or counsel."
        ),
        "compliance.table_head": "| Provision | Artifact | What it can prove | What it cannot prove |",
        "compliance.closing": (
            "The mapping only says which provision an artifact can support; "
            "**it does not mean the provision is met**."
        ),
        "compliance.art12.req": "EU AI Act Art. 12 (automatic logging, tamper-evidence, retention)",
        "compliance.art12.art": "sha256 manifest plus the independent verification result",
        "compliance.art12.proves": "The delivery has not changed since it was generated, and the recipient computed that themselves.",
        "compliance.art12.cannot": "Only covers what was collected into the delivery; retention length is your storage policy, not enforced here.",
        "compliance.art14.req": "EU AI Act Art. 14 (human oversight)",
        "compliance.art14.art": "The `approve` state in the policy",
        "compliance.art14.proves": "High-risk calls have a human gate, and the decision is recorded.",
        "compliance.art14.cannot": "Only sees calls that go through this policy; approval quality depends on people.",
        "compliance.art11.req": "EU AI Act Art. 11 / 13 (technical documentation, transparency)",
        "compliance.art11.art": "Scope and method, the coverage boundary, and the reason behind every verdict",
        "compliance.art11.proves": "A deliverable description exists and each permission decision is traceable.",
        "compliance.art11.cannot": "Does not produce full technical documentation: model cards, data governance and training data are out of view.",
        "compliance.art15.req": "EU AI Act Art. 15 (robustness, cybersecurity)",
        "compliance.art15.art": "The least-privilege policy itself (read separated from write, destructive denied)",
        "compliance.art15.proves": "The capability surface is narrowed, and the narrowing is grounded in evidence.",
        "compliance.art15.cannot": "Not equivalent to passing a cybersecurity assessment; application-layer flaws are out of scope.",
        "compliance.art17.req": "EU AI Act Art. 17 (quality management system)",
        "compliance.art17.art": "A delivery that a third party can re-verify",
        "compliance.art17.proves": "The process itself is verifiable and can serve as process evidence.",
        "compliance.art17.cannot": "A QMS is an organisation-level system; this is only the logging and evidence input.",
        "compliance.iso.req": "ISO/IEC 42001 (AI management system)",
        "compliance.iso.art": "As above (policy rationale, manifest, independent verification)",
        "compliance.iso.proves": "Usable as evidence input for operational control and monitoring clauses.",
        "compliance.iso.cannot": "Not a certification and no statement of conformity under that standard.",
    },
    "zh-CN": {
        "report.title": "# Agent 权限收敛报告",
        "report.meta.generated": "- 生成时间：{ts}",
        "report.meta.agent": "- agent：{agent}",
        "report.meta.default": "- 默认决策：`{decision}`（未登记的一律拒绝）",
        "report.meta.note": "- 说明：{note}",
        "report.summary.heading": "## 1. 摘要",
        "report.summary.body": (
            "本机共梳理 **{servers}** 个 server / **{tools}** 个工具："
            "放行 **{allow}**、需人工审批 **{approve}**、拒绝 **{deny}**。"
        ),
        "report.summary.review": (
            "其中 **{n}** 个工具无法从名称与描述判定能力，已置为「需人工审批」并列入下方待确认"
            "——**请先核对这一批再用**。"
        ),
        "report.table.heading": "## 2. 权限表",
        "report.rationale.heading": "## 3. 判定依据",
        "report.rationale.body": "每条判定为什么这么判——没有这一节，读者无法判断这份报告是否可信：",
        "report.rationale.table_head": "| 工具 | 判定依据 |",
        "report.review.heading": "**待确认（依据不足，需人工判断）**",
        "report.coverage.heading": "## 4. 覆盖边界（这份报告不能证明什么）",
        "report.coverage.1": "- 只覆盖被扫描到的配置；扫描不到的 agent 不在其中（扫描结果里会列出未解析的配置）。",
        "report.coverage.2": "- 未观测到调用的工具仍可能被使用——「未观测」不等于「没发生」。",
        "report.coverage.3": "- 本报告不构成安全评估结论，也不代表系统已通过任何合规认证。",
        "compliance.heading": "## 合规条款映射（EU AI Act / ISO 42001）",
        "compliance.disclaimer": (
            "本节把交付物里的证据逐条对应到条款要求。"
            "**它是索引，不是法律意见，也不构成合规声明**——请与你的合规负责人或律师一起使用。"
        ),
        "compliance.table_head": "| 条款要求 | 本工具的产物 | 能证明什么 | 不能证明什么 |",
        "compliance.closing": "映射只描述「这个产物能支撑哪条要求」，**不代表该要求已经满足**。",
        "compliance.art12.req": "EU AI Act Art. 12（自动记录事件、防篡改、留存期）",
        "compliance.art12.art": "sha256 清单 + 独立校验结论",
        "compliance.art12.proves": "交付物自生成以来未被改动，且这一点由收货方自己算出",
        "compliance.art12.cannot": "只覆盖被采集进交付物的范围；留存期限由你自己的存储策略决定，工具不强制",
        "compliance.art14.req": "EU AI Act Art. 14（人类监督）",
        "compliance.art14.art": "策略里的 approve 三态",
        "compliance.art14.proves": "高危调用存在人工闸门，且审批结果被记录下来",
        "compliance.art14.cannot": "只看得到走该策略的调用；审批质量取决于人",
        "compliance.art11.req": "EU AI Act Art. 11 / 13（技术文档与透明度）",
        "compliance.art11.art": "报告的范围与方法、覆盖边界、以及每条判定的依据",
        "compliance.art11.proves": "有一份可交付的技术说明，且每条权限判定都能追回依据",
        "compliance.art11.cannot": "不生成完整技术文档：模型卡、数据治理、训练信息不在视野内",
        "compliance.art15.req": "EU AI Act Art. 15（鲁棒性与网络安全）",
        "compliance.art15.art": "最小权限策略本身（读写分离、破坏性操作默认拒绝）",
        "compliance.art15.proves": "agent 的能力面被显式收窄，且收窄依据可核查",
        "compliance.art15.cannot": "不等于系统通过了网络安全评估；应用层漏洞不在覆盖范围内",
        "compliance.art17.req": "EU AI Act Art. 17（质量管理体系）",
        "compliance.art17.art": "可被第三方独立复验的交付物",
        "compliance.art17.proves": "交付流程本身可验证，可作为过程证据的一部分",
        "compliance.art17.cannot": "QMS 是组织级体系；本工具只提供其中日志与可验证证据这一块的输入",
        "compliance.iso.req": "ISO/IEC 42001（AI 管理体系）",
        "compliance.iso.art": "同上（策略依据 + 清单 + 独立校验）",
        "compliance.iso.proves": "可作为运行控制与监视测量条款的证据输入",
        "compliance.iso.cannot": "不构成 42001 认证，也不代表该标准下的任何符合性声明",
    },
}


def resolve_locale(explicit: str | None = None) -> str:
    """优先级：显式参数 > RATCHET_LANG > LC_ALL/LANG > 默认 en-US。"""
    for raw in (explicit, os.environ.get("RATCHET_LANG"),
                os.environ.get("LC_ALL"), os.environ.get("LANG")):
        loc = _normalize(raw)
        if loc:
            return loc
    return DEFAULT_LOCALE


def _normalize(raw: str | None) -> str | None:
    if not raw:
        return None
    s = raw.strip().lower()
    if s.startswith("zh") or "chinese" in s:
        return "zh-CN"
    if s.startswith("en"):
        return "en-US"
    # "C" / "POSIX" 是"没有语言偏好"，不是一种语言。必须连 C.UTF-8 这类
    # 带上编码后缀的写法一起认——CI 与容器里默认就是 LC_ALL=C.UTF-8，
    # 只匹配裸的 "c" 会让这类环境漏到下一层 LANG（常见是 zh_CN.UTF-8），
    # 于是"默认英文产物"在别人机器上变成中文，而本机测试却是绿的。
    if s == "posix" or s.startswith("posix.") or s == "c" or s.startswith("c."):
        return None
    return None


def t(key: str, locale: str, **params) -> str:
    """取词条。缺词条时回落到英文，再回落成 key 本身——不静默给空串。"""
    table = CATALOG.get(locale) or CATALOG[DEFAULT_LOCALE]
    text = table.get(key) or CATALOG[DEFAULT_LOCALE].get(key) or key
    return text.format(**params) if params else text
