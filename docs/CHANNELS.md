# 推广渠道总表

> 一张表管住所有渠道，避免重复劳动或漏掉。最后更新：2026-09-20。
> 详细经过见 [DELIVERY-0.14.0.md](DELIVERY-0.14.0.md)。

## 状态图例

`已收录` 长期有效 · `待审` 已提交等人审 · `阻塞` 有明确外部原因 · `待做` 还没动 · `不做` 已论证不划算

## 一、目录 / 市场（发现入口）

| 渠道 | 状态 | 说明 / 下一步 |
|---|---|---|
| 官方 MCP Registry | 已收录 | `io.github.iversonwuwei/ratchet` 0.12.0，级联 Smithery / PulseMCP / GitHub Registry |
| Glama | 待审 | 2026-09-19 提交，通过后把分数徽章补进 punkpeye 的 PR |
| Docker 官方 MCP Catalog | 待审 | [PR #5164](https://github.com/docker/mcp-registry/pull/5164)，Docker Desktop 用户可见 |
| punkpeye/awesome-mcp-servers | 待审 | [PR #14700](https://github.com/punkpeye/awesome-mcp-servers/pull/14700)，机器人要求先有 Glama 徽章 |
| awesome-ai-security-tools | 待审 | [PR #117](https://github.com/scadastrangelove/awesome-ai-security-tools/pull/117)，按规则进观察名单 |
| Smithery | 暂缓（判定不适合） | server 记录已建（`iverson-wuwei/ratchet`），但**控制台只收公网 URL**（页面字段：Namespace / Server ID / MCP Server URL），没有 MCPB 上传入口；API 那条路要的请求体没有公开 schema。ratchet 是本地 server，为它改成托管服务会背离产品定位 → 不值得 |

> **MCPB 包的更好去处（2026-09-20 发现）**：MCPB 就是 **Claude Desktop 一键安装本地 server 的格式**。
> 我们为 Smithery 打的 `dist/ratchet-0.12.0.mcpb` 可以直接作为 release 附件发出去，
> 让 Claude Desktop 用户一键装上（不需要 Smithery 这个中间人）。这比登进 Smithery 划算得多。
| PulseMCP | 阻塞 | `/submit` 用 curl 能取到 200，但**内置浏览器导航超时**（bot 防护）。它抓官方 Registry，我们已在那上面，大概率自然收录 |
| mcp.so | 阻塞 | `/submit` 返回 307 跳登录，需要账号态 |
| mcpmarket.com | 阻塞 | 同上：curl 200，但**内置浏览器导航超时**（2026-09-20 实测） |
| wong2/awesome-mcp-servers | 阻塞 | 仓库**禁用 PR 与 issue**，零入口；条目已在 fork 分支备好 |
| npm 包名占位（`npx ratchet-mcp`） | 待做 | 需要 npm token；MCP 圈子的安装直觉是 npx |

## 二、内容与社交

| 渠道 | 状态 | 说明 |
|---|---|---|
| X（@WaldenWuwei） | 已发 | 4 条线程 + 6 条点对点回复；由定时任务每天三轮继续 |
| dev.to（waldenwuwei） | 已发 | 长文，canonical 指向 `podcloud.dlszjr.com` |
| Reddit | 阻塞 | 出口 IP 被网络策略拦；登录后发帖框不渲染提交按钮；API 应用创建被判"账号太新"。需要养号 |
| Hacker News | 阻塞 | Show HN 被临时限制；普通提交判 `toonew`。需要养号 |
| LinkedIn | 已发 | 2026-09-20 发布中文长动态（账号「吴嵬 / 大连素辉软件科技有限公司」）：五步方法论 + 本机实测数字，不带链接 |
| TikTok | 不做 | 人群/通道/形式三条都不匹配，见 DELIVERY-0.14.0 §13 |

## 三、生态集成（把工具放进别人已经在用的地方）

| 渠道 | 状态 | 说明 |
|---|---|---|
| Homebrew tap | 已收录 | `suhui-organization/tap/ratchet`，README 已补说明 |
| GHCR 镜像 | 已发布 | `ghcr.io/suhui-organization/ratchet`，公开可拉 |
| GitHub Releases | 已发布 | v0.12.0，四平台二进制 + install.sh + SHA256SUMS |
| Cursor / VS Code / Claude Code 的 MCP 市场 | 待做 | 各自的收录方式不同，需要逐个摸 |
| Continue.dev / Zed / Windsurf 目录 | 待做 | 同上 |

## 四、社区与媒体（长周期，换可信度）

| 渠道 | 状态 | 说明 |
|---|---|---|
| OWASP GenAI Security 社区 | 待做 | 我们已写过 ASI04 的映射，去工作组正常参与是最短路径 |
| MCP 相关 Discord（Glama / Smithery / 官方） | 待做 | showcase 频道发一次项目介绍是被欢迎的 |
| 安全类 Newsletter（tl;dr sec 等） | 待做 | 写好 pitch 邮件由你发送 |
| 技术媒体投稿（The New Stack / Help Net Security） | 待做 | 把 dev.to 那篇改写投稿 |
| 会议 CFP（BSides / DevSecCon / KubeCon） | 待做 | 周期长，可信度最高 |
| YouTube / Demo 外联 | 待做 | 找 3–5 个做 MCP 测评的小频道 |

## 纪律（对所有渠道生效）

> **目录层的结论（2026-09-20）**：能提交的都已提交（官方 Registry / Glama / Docker Catalog /
> 两个 awesome 清单）；剩下的三个目录（PulseMCP、mcp.so、mcpmarket）**全部卡在反爬或账号态**上，
> 内置浏览器连页面都打不开。继续在这一层投入的边际收益很低——它需要的是**人工在普通浏览器里
> 逐个填表**，或者干脆等官方 Registry 的级联。

1. 只回**自己说出痛点**的人，不追着产品推广帖回复；
2. 回复里放事实与数字，**不带链接、不提产品名**；
3. 一条帖子只回一次，不复制粘贴同一句话；
4. **不做刷量**（不批量点赞/投票）——被判定操纵会连账号一起失去；
5. 平台给的原话要逐字记进交付记录，否则下次还会重复踩。
