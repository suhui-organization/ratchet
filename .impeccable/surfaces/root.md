---
version: 1
slug: "root"
primary_target: "root"
related_targets: []
---

# Surface: 控制台（/）

- **Scope**：控制面自带的一页控制台，随镜像发布（`service/src/ratchet_service/console/`）。
- **Mode**：Operate。使用者是值班的工程师或做人工验收的人。
- **Target**：`/`（控制面根路径）。相关：`/ledger`、`/reach`、`/contain`、`/silence`、`/verify`。

## Audience / job

值班的人在事故里问的第一句是"**谁碰过 X**"。第二句是"凭什么这么说"。
所以这一页的主对象不是"凭据列表"，而是**以资产为中心的触达答案**，台账是它的证据面。

约束：内网可用（无 CDN、无构建步骤）、镜像只用标准库、控制面**没有认证**必须显著标注。

## Direction contract

**THESIS**：先答"谁碰过 X"，再让人顺着依据走到凭据与九条规则。
拒绝的做法：先铺一排大数字的指标卡（那是监控看板，不是这一次任务）。

**OWN-WORLD**：继承已上线站点（`web/app/assets/css/main.css`）：白底 `#fff`、面色 `#f7f8fa`、
墨 `#202124`、次要 `#8691a1`、线 `#e5e7eb`、品牌 `#4d6bfe`。等宽（JetBrains Mono 的
系统回退）只用于 id、哈希、计数、规则名——**不当"技术感"的装饰**。1px 线、4–6px 圆角、
不用投影堆叠；状态用自绘 SVG 圆点/环/短横表示，不用 emoji。

**STORY**：输入资产名 → 看到谁能碰、依据在哪条凭据 → 点开源凭据看九条规则与失败原因 →
对某个 agent 先"预演遏制"看清会连累谁。

**FIRST VIEWPORT**：顶部一行状态（探活 / 台账份数 / 事件链 / 断流数 / 无认证警告），
下面是一个整宽的查询条（占位符给本机真实资产名），再往下左侧是证据列表
（能 / 被拒 / 观测到用过 / 已遏制），右侧是所选凭据的九条规则与依据。

**FORM**：证据链视图（reach-centric console）。我自列 7 个结构的第 5 个。
seed key `9bf3af1a`（roll 降级运行：无 challenger、无 quality-bar board）。

**FINISH**：unreviewed and undocumented is unfinished; this build ends with the finish review,
the verdict, DESIGN.md, and every shipping raster carrying its provenance.
