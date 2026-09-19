# 交付记录 0.14.0 —— 开始营销：收款接上、漏斗补上、内容到位

> 日期：2026-09-19。上一版见 [DELIVERY-0.13.0.md](DELIVERY-0.13.0.md)。
> 对应 [GTM.md](GTM.md) 里的"第一节之后、第二节开始"。

## 1. 这一版做了什么（第二节四件事）

| # | 计划里的事 | 状态 | 落点 |
|---|---|---|---|
| 2.1 | Registry 重发到 0.12.0 | **已发布**（见 §7） | `server.json`（version 0.12.0 / identifier `v0.12.0`） |
| 2.2 | 定价页接上真实收款 | **已上线**（见 §4 验收） | `web/app/pages/pricing.vue`、`web/app/utils/paddle.ts`、`web/app/pages/thanks.vue` |
| 2.3 | 内容资产补到 4 篇 | 已完成 02/03/04 | `docs/content/02-pinning-reality.en.md`、`03-tool-surface.en.md`、`04-answer-the-question.en.md` |
| 2.4 | 首页补"10 秒出数字"钩子 | **已上线** | `web/app/pages/index.vue` 的 `#after` 段 |

## 2. 收款：Paddle（真实接入，不是占位）

站点是纯静态的，所以走 **Paddle.js overlay**：前端用公开的 client token 加 price id
直接开收银台，**不需要自己的后端**——也就不需要保存任何订单号、邮箱或支付数据。
这与产品对外的说法一致：付款这件事整个交给 Paddle。

| 项 | 值 | 说明 |
|---|---|---|
| 商品 | `pro_01m2wn5wesc49jqy73ymndx4hx` — Agent permission audit (one engagement) | 新建，status=active |
| 价格 | `pri_01m2wn60hxk03jmrdd2h0bx489` — USD 1500.00，一次性 | 取 GTM 价格带（$1,500–8,000）的最低位 |
| client token | `live_297463a10ad89aa3315d67a53d0` | **公开值**，本来就跑在浏览器里；写进 `site.ts` 没有风险 |
| api key | 不落仓库 | 只用于建商品/价格，留在服务器的 Secret 里 |
| 域名 | `podcloud.dlszjr.com` | 沿用 Paddle 已认证域名，所以没有重走审核 |

页面与收银台的一致性靠一条规矩守住：`site.ts` 的 `auditPrice` 必须与 Paddle 后台那个
price 的金额一致。页面上写 1500、收银台收 2000，那是合规问题不是文案问题。

配套改动：

- `deploy/docker/security-headers.inc`：CSP 放行 Paddle——`script-src cdn.paddle.com`、
  `frame-src *.paddle.com`、`connect-src *.paddle.com`、`form-action *.paddle.com`。
  **少一条，收银台就静默打不开**，而且只在浏览器控制台报错。
- `web/app/pages/thanks.vue`：付完之后去哪。写清三件事——什么时候有人联系、交付物是什么、
  对方怎么用它自己的浏览器验证。买完落回首页是最容易让人后悔的一步。

## 3. 内容：三篇都用本机实测数字

不编数据。这三篇里每个数字都来自 2026-09-19 在本机的真实扫描：

| 数字 | 值 |
|---|---|
| MCP server | 16（1 个 harness：Codex） |
| 未锁版本 | 12（其中 4 个写了 `@latest`，8 个连版本串都没有） |
| 工具总数 | 164（12 个 server 答话，4 个枚举不出来） |
| 策略三态 | allow 43 · approve 106 · deny 15（另有 24 条"判不出来"） |

第 03 篇特意保留了两条**误判**（`resolve-library-id` 因描述里的 "format" 被判 deny、
`sequentialthinking` 因 "clear" 被判 deny）。删掉它们，这篇就从"实测"变成"宣传"。

## 4. 首页漏斗

安装命令之后补了一段 `#after`：`scan` → `policy draft` → `deliver`，结尾两个入口
（去验证页 / 让人替你跑一遍）。原来落地页在"装完了，然后呢"这里断掉——
曝光能变成安装，但安装之后没有下一步。

## 5. 测试

```console
$ make test
python 34 passed · web 15 passed（新增 tests/paddle.test.ts 7 条）· Go 全包 ok
```

`web/app/utils/paddle.ts` 被特意做成纯函数：配置判空、environment 归一化、
successUrl 拼接都能在 node 环境直接测，不需要 Nuxt 运行时，也不会在 prerender 期间碰 window。

## 6. 上线与验收

镜像 `swr.ap-southeast-3.myhuaweicloud.com/digital-finance/ratchet-web:0.12.0-54ba377`
已发布到 `ratchet` 命名空间（2/2 Running，`check-drift.sh` 退出码 0），出口域名
`https://podcloud.dlszjr.com` 上可以看到：

| 验收项 | 结果 |
|---|---|
| 首页三步漏斗 | `Ten seconds after install…` + `Verify a delivery` 出现 |
| 定价页 | `$1,500 USD` + `Buy the audit` + `Paddle` 说明 |
| 付完之后的页面 | `/thanks` → 200 |
| CSP 放行收银台 | `script-src https://cdn.paddle.com; frame-src https://*.paddle.com` 已生效 |

途中新增 `deploy/swr/release.sh`：把"同步部署文件 → 服务器 helm upgrade → 漂移检查"
合成一条命令。这次发布正好证明它有用——VPN 中间断过一次，重连后一条命令补完，
不用再手敲三次 ssh。

## 7. Registry 已重发到 0.12.0

官方 Registry 上现在是最新版本，第三方目录（Smithery / PulseMCP / GitHub Registry）随之级联：

```
$ mcp-publisher publish
✓ Server io.github.iversonwuwei/ratchet version 0.12.0

$ curl .../v0/servers/io.github.iversonwuwei%2Fratchet/versions
0.12.0 | ghcr.io/suhui-organization/ratchet:v0.12.0 | 2026-09-19T11:20:53Z
0.11.0 | ghcr.io/suhui-organization/ratchet:v0.11.0 | ...
```

途中两个真实故障，都记下来了：

1. **设备授权要连点两次**：第一次授权成功后，轮询被网络重置（这台机器到 GitHub 一直不稳），
   token 没落盘、码作废。第二次把 Go 的 HTTP/2 关掉（`GODEBUG=http2client=0`）才走完。
   再遇到 `TLS handshake timeout` / `connection reset`，先试这个开关，然后直接重试 publish——
   它幂等。
2. **公开性判据写错过**：GHCR 对未带 token 的 manifest 请求一律先回 401（正常握手），
   旧文档却把它当成"包是私有的"。已改成用匿名 token 端点判断，见
   [publishing-to-mcp-registry.md](publishing-to-mcp-registry.md)。

## 8. 未完成

到这一版为止，第二节四件事全部完成。剩下的是**发出去**，以及只有你能做的动作：

| # | 事项 | 谁做 |
|---|---|---|
| 1 | 在线上点一次 `Buy the audit`，确认 Paddle overlay 能弹出 | 你（需要真实浏览器） |
| 2 | 按渠道清单发第一批内容（目录已就位，顺序：先目录后社交） | 你 / 我出文案 |
| 3 | 轮换对话里出现过的 Vercel token 与服务器密码 | 你 |

## 9. 第一波：目录收录（GTM 第 3 节，先目录后社交）

| 渠道 | 动作 | 结果 |
|---|---|---|
| **awesome-mcp-servers**（95k star） | PR 加进 🔒 Security 段，标题带 `🤖🤖🤖`（该清单给自动化提交留了快速通道） | [PR #14700](https://github.com/punkpeye/awesome-mcp-servers/pull/14700)，open / mergeable |
| **awesome-ai-security-tools** | 按他们的门槛（零 star、仓库太新的先不放主列表）提交到 WATCHLIST | [PR #117](https://github.com/scadastrangelove/awesome-ai-security-tools/pull/117)，open / mergeable |
| **Glama** | 加 `glama.json`（`maintainers: ["iversonwuwei"]`，schema 从 Glama 官方端点取的） | 待提交收录，见下 |
| **Homebrew tap** | 补 README：装什么、数字什么意思、**它不是什么** | 已推 `suhui-organization/homebrew-tap` |

### 途中修掉的两个真问题

1. **仓库没有 LICENSE 文件，但站点页脚与镜像 label 都写着 Apache-2.0。**
   等于对外声明了一件没有对应授权文件的事。已补 Apache-2.0 全文（`LICENSE`，202 行）。
   这一条是被 awesome-ai-security-tools 的收录门槛照出来的：他们明确要求"有清晰的根许可证"。
2. **README 的"状态"还停在 0.1.0**，写着"真实环境扫描尚未实现"——和刚跑出来的实测
   （16 server / 12 未锁 / 164 工具）直接矛盾。已按 0.12.0 的实况重写，并把边界写清楚。

### Glama 这一步卡在哪

awesome-mcp-servers 的机器人要求：先用 Dockerfile 在 Glama 上把 server 跑通、通过
introspection 检查，再把 Glama 分数徽章加到条目里。现状：

- 徽章地址能取到，但内容是 `This MCP server is not listed on Glama` —— 即还没收录；
- 提交入口 `https://glama.ai/mcp/servers` 需要以 GitHub 账号登录（我这边没有你的会话）；
- 我们的 `Dockerfile` 默认 `CMD ["mcp"]`，起来就是一个 stdio MCP server，符合
  "能启动并响应 introspection" 这条要求，不需要额外改造。

所以这一步只差一次人工提交，提交完我把徽章补进 PR（机器人会重新检查）。

## 10. 第一轮社交：X 线程已发（2026-09-19）

发布方式不是 API key，而是**浏览器自动化**——用 Codex 浏览器在已登录的会话里操作
（这也是之前那批推广实际用的方式：`js` 在会话里被调用了 218 次）。所以"要不要账号密码"
这个前提本身就是错的，账号在浏览器里。

已发布（账号 `@WaldenWuwei`），四条串成一条线程，按纪律**不带链接**：

| # | 内容 | 链接 |
|---|---|---|
| 1 | 16 个 MCP server、12 个未锁版本 | https://x.com/WaldenWuwei/status/2101278269282853245 |
| 2 | 下次启动就重新解析；谁能改那个 npm 名字，谁就决定我机器上跑什么 | https://x.com/WaldenWuwei/status/2101278515132051520 |
| 3 | 4 个写 `@latest`、8 个连版本串都没有，后者反而更像"稳定依赖" | https://x.com/WaldenWuwei/status/2101278619847012368 |
| 4 | 16 个里有 4 个根本枚举不出来——未知不等于安全 | https://x.com/WaldenWuwei/status/2101279631815127292 |

数字与 `ratchet scan` 在 2026-09-19 的实测一致（16 / 12 / 4+8 / 164 工具）。

### Glama：已提交（2026-09-19）

登录后 "Add Server" 直接给的就是提交表单（不是在注册墙后面），已按表单要求提交：

| 字段 | 值 |
|---|---|
| Name | `Ratchet` |
| Description | 一句话说明：只读 CLI + stdio MCP server，列 server/工具 → 编译三态策略（每条带依据）→ 出可自验的交付物 |
| GitHub Repository URL | `https://github.com/suhui-organization/ratchet` |

结果：页面弹出 **"Your server has been submitted for review"**。
表单里写明：审核通过后会**发邮件要求提供 Dockerfile** 做自动化安全与质量检查，
只有通过检查的 server 才会被索引。我们的 `Dockerfile` 默认 `CMD ["mcp"]`，
起来就是一个 stdio MCP server，符合"能启动并响应 introspection"这条要求。

**踩坑记录**（下一次省时间）：

1. 第一次点 GitHub 登录让内置浏览器**整页崩了**（`This page crashed`）；
2. 登录态就绪后弹窗是提交表单，但内置浏览器的 AX 快照只回**增量 diff**，
   第二次取快照会返回 "no change"，拿到的是过期元素号——所以每取一次快照就要用完再取下一次；
3. 最后是靠 `tab.playwright.locator('button:has-text("Submit for Review")')` 点掉的：
   **有 DOM 接口就别猜元素号**。

审核通过、拿到徽章地址后，把徽章补进 awesome-mcp-servers 的 PR（机器人会重新检查）。

## 11. 第二轮社交：dev.to 已发、Reddit 被网络策略拦住

### dev.to（已发布）

https://dev.to/waldenwuwei/i-counted-what-my-ai-agents-can-actually-touch-16-mcp-servers-164-tools-15-denials-1lmj

- 正文用 `01` / `03` 两篇内容包的真实数字（16 server / 12 未锁 / 164 工具 / allow 43 · approve 106 · deny 15），
  并把两条**误判**写进去（`resolve-library-id` 命中 "format"、`sequentialthinking` 命中 "clear"）；
- **canonical_url 已设为 `https://podcloud.dlszjr.com`**（已用 dev.to API 复核：
  `canonical: https://podcloud.dlszjr.com`）——不设的话 SEO 权重会全给 dev.to；
- 发布用的方式是 Playwright 接口（`#article-form-title` / `#article_body_markdown` / `#canonicalUrl`），
  比猜 AX 元素号稳得多；"Publish" 要点两次（第一次被编辑器里的状态抢走了）；
- 遗留：tags 没加上（`#tag-input` 需要 type+Enter 而不是 fill），下次发时补。

### Reddit r/mcp（发不出去，有证据）

从这台机器的出口 IP（59.46.235.173）访问 Reddit 被**整站网络策略拦截**，新旧界面都一样：

- `https://www.reddit.com/r/mcp/submit` → `You've been blocked by network security.`
- `https://old.reddit.com/r/mcp/submit` → `Your request has been blocked due to a network policy.`

这不是登录问题（拦截页在登录之前就返回了），也不是浏览器的问题——是这个网段被 Reddit 挡了。
绕过的办法只有两条：换网络（手机热点/境外节点）发，或走 Reddit 的 developer token。
文案已经备好（[content/01](content/01-scan-report.en.md) 的 "Reddit — r/mcp" 段），
在手机浏览器里贴一次即可，不需要改动。
## 8. 这一轮之后的判据

按 GTM 的止损线看，接下来两周要盯的是三个数，不是 star：

1. 定价页上"Buy the audit"被点了几次（Paddle 后台能看到 checkout 打开数）
2. 有没有人走完 `scan` 之后接着跑 `policy draft`
3. 第一个付费单——若 60 天没有，按 GTM 的结论停下来，不继续改产品
