# 项目现状（单一事实来源）

> 更新：2026-09-25。**任何"现在做到哪了"的问题，以这份文件为准**；
> 其它文档要么是实现细节，要么是历史记录（见文末索引）。
> 每条都写证据（命令、文件、可复现的输出），不写"基本完成"这种话。

## 一句话

**既能拦住，也说得清。**

拦的那一半：agent 要把备份删掉之前把它停下（`ratchet guard`，L3）。
说得清的那一半：被客户/审计问"这些 agent 到底被允许碰什么、你凭什么这么说"时，
**交得出东西**——一份别人不用信你、自己就能验的凭据（L1/L2）。

两半共用同一份策略，所以不会出现"策略里写着拒、执行点却不认"这种静默失效。
这是本产品批评别人的那件事，自己不能犯。

| 级 | 是什么 | 状态 |
|---|---|---|
| **L1 证据** | 一台机器一条命令 → 可被第三方独立验证的凭据（+ 证据哈希） | ✅ **可用** |
| **L2 台账** | 多机多天汇总、复算验证、跨次比对、触达查询、遏制记录 | ✅ **本机 k8s 在跑**（单租户、无认证） |
| **L3 执行点** | PreToolUse 拦截：把"能碰"变成"只能碰" | ✅ **可用**（`ratchet guard`，走 hook 不走代理；见 [GUARD.md](GUARD.md)） |

## 一、现在能直接用（照抄即可）

```bash
# Agent 侧：一条命令出一份凭据（读本机真实配置，离线也能跑）
ratchet deliver --out /tmp/v --home "$HOME" --client 客户名 \
  --report-cmd "python3 -m ratchet_service.cli"
```

| 东西 | 地址 / 命令 | 现在可用性 |
|---|---|---|
| 控制面（server）控制台 | http://127.0.0.1:30090 | ✅ 活（版本 `0.1.0-pitch3`） |
| Agent 侧 CLI | 装：`curl -fsSL https://github.com/suhui-organization/ratchet/releases/latest/download/install.sh \| sh` | ✅ 已发布 v0.13.0，下载验证过 `reach/contain/chain` 都在 |
| 收货方验证页（零安装） | https://podcloud.dlszjr.com/verify | ✅ 活 |
| 本机部署 | `make deploy-local` + `make local-gateway` | ✅ 幂等，可重来 |
| 人工验收清单 | [VERIFY-LOCAL.md](VERIFY-LOCAL.md) 九步 | ✅ 我逐条实跑过 |
| 十分钟演示脚本 | [DEMO-10MIN.md](DEMO-10MIN.md) | ✅ 含"不要说的话" |

## 二、组件状态

| 组件 | 代码 | 测试 | 部署 | 发布 |
|---|---|---|---|---|
| `ratchet` CLI（Agent 侧） | ✅ 12 条命令 | ✅ Go 11 包 | — （本地二进制 / Release） | ✅ v0.13.0 |
| 传感器（Agent 侧的无人值守形态） | ✅ 脚本 + 容器 | ✅ 表面用例锁住行为 | ✅ kind CronJob（每天 09:00） | ✅ 镜像进 SWR，按 digest 钉 |
| 控制面 `asas-api`（server） | ✅ 12 个端点 | ✅ 22 条 HTTP 用例（真起服务打真请求） | ✅ kind `ns=asas`，台账落 PVC | ✅ 镜像进 SWR |
| 控制台页面 | ✅ 单文件，无构建、无 CDN | ✅ 页面断言 + Playwright 度量 | ✅ 由控制面自己提供 | 随镜像 |
| 规范 ASAS v0.3 | ✅ 8 控制族 / V1–V9 | ✅ 跨语言向量 | — | 公开评审稿 |
| 发布流水线 | ✅ 打 tag 自动编译 | ✅ 新增 CI（每次 push 跑） | — | ⚠️ **有红灯**（见 §6） |

## 三、已完成（按能力，不按轮次）

**Agent 侧**

* 盘点：读真实 harness 配置列出 MCP server；`--introspect` 要显式 `--allow-exec` 才执行配置里的命令。
* 策略：从真实调用编译最小权限策略，**每条判定带依据**；三态（allow/approve/deny）。
* 凭据：产出 ASAS-A 并**当场自查**，不过自查不算交付；未定版必须书面豁免（带到期日）。
* 事件流：调用记录 → 带哈希链的事件流（`seq` + 前序哈希），删行/改行/砍尾巴都会露馅。
* 传感器：采集 → 出证 → 上报 → 传证据 → 传事件 → 复验，一条命令走完；默认**离线**（出网有预算）。
* 委派：声明"替谁干活"，作用域收窄、deny 只增不减、子不得比父活得久。

**执行点（guard）**—— 见 [GUARD.md](GUARD.md)

* 拦截：`ratchet guard` 挂 PreToolUse，在工具**执行之前**给出 allow / ask / deny。
* 覆盖十二家 agent（配置路径全部有官方文档佐证）：Claude Code、Codex CLI、
  CodeBuddy（腾讯云）、Qwen Code（阿里）、Qoder/通义灵码（阿里）、
  Gemini CLI、Antigravity CLI、Cursor、Crush、Factory Droid、Cline、OpenCode。
  五套配置形状 + 七套输出协议，抄错一种就是静默不拦。
* Cline 与 OpenCode 不是配置型：前者要往约定目录丢一个可执行文件，后者要装一个 JS 插件。
* **不用退出码阻断**：Gemini CLI 认退出码 2，而 Antigravity 的非零退出码只写日志不阻断；
  统一走"退出码 0 + JSON"，一份输出喂两家。
* **从不输出 `allow`**：在多数 agent 上它的语义是"绕过权限系统直接执行"，
  输出它等于替客户跳过他自己的确认。放行时保持沉默。

**公开站点（web/）**

* **中英双语**：`/` 英文、`/zh` 中文，路径前缀区分（可直接分享，搜索引擎分别收录）。
  文案两棵树都在 `web/app/site.ts`，字段结构由 `web/tests/i18n.test.ts` 强制对齐——
  漏翻会让测试红，而不是页面上悄悄少一句话。
* 营销页 `/guard` 把卖点、机制、覆盖范围、**风险**与建议都放进正文（风险不是免责声明）。
* 可访问性实测（2026-09-25）：8 条路由 × 390px 与 1440px，横向溢出 0、越界元素 0、
  小于 24px 的点击目标 0；全站文字对比度 0 处低于 WCAG AA。
  同一次实测修掉了两处文档与实现的漂移：正文/元信息色低于 AA、主按钮白字只有 4.33:1。
* 目标优先：锚点是"碰的是什么东西"而不是"用的哪个工具"——被标成 allow 的工具照样不能写受保护目标。
* 两类目标分开：凭据类（读也拒）与不可恢复类（只在写或删时拒），合成一张会把 agent 废掉。
* 破坏性藏在参数里也能认出：`shell` + `rm -rf /srv/pgdata` 会被拦。
* 留痕可验证：每条判定记依据、命中项、**值指纹**与**策略指纹**；被拒调用进哈希链事件流。
* 失败模式默认"放行但可见"（不把客户锁在机器外），`--fail-closed` 可改。

**控制面（server）**

* 台账 + 复算验证：**同一份规则实现**（与 CLI 逐字一致，实测 diff 为空）。
* 证据上传：`hashMatch` 从"未评估"变成"通过"（9/9 规则全部可评估）。
* 事件流跨次比对：比对**重算值**（不是事件自称的 hash），改中间一条也能点到 seq。
* 遏制编排：级联到下级、记 `via`、`dryRun` 只预告不写库。
* 触达查询：授权面 / 观测面 / 拒绝 / 遏制分列，每条带依据与委派链；查不到会明说"答不了"。
* 控制台：定位 + 两步接入指引（地址自动填）+ 查询条 + 凭据面板（九条规则）+ 断流区。

**交付与验收**

* 离线验证：`python3 verify.py <交付目录>`，第三方零安装。
* 规范：v0.3，含规范化哈希口径（互操作必需）与跨交付比对；测试向量 27 条。
* 一致性：本地 `asas.verify` 与控制面 `/verify` 输出**逐字一致**（有验收记录）。

## 四、未完成

### A. 明确缺口（知道要做，还没做）

| # | 缺口 | 影响 | 判据（做完怎么算） |
|---|---|---|---|
| A1 | 控制面**没有任何认证** | 只能本机/内网用，不能给客户 | 有 token/OIDC 鉴权，未授权请求被拒 |
| A2 | **没有公网部署** | 客户访问不到控制面 | 服务器 192.168.66.8 上跑起来 + 域名 + TLS |
| A3 | 日志**轮转语义**未定义 | 归档后重开链会被记成"断流" | 有 `rotation` 声明，合法轮转不误判、改写仍然失败 |
| A4 | 事件流**全量上传** | 忙的机器上文件持续增长 | 分段上传 + 锚点续接，控制面仍能比对前缀 |
| A5 | `mcpb` 资产缺失 | 网站 `site.ts` 指向 `0.13.0.mcpb`，一旦重新部署就是 404 | `make release` 产出 mcpb，或用脚本补到 release |
| A6 | Release 流水线最后一个红灯 | 每次发版都红（**不影响安装**，资产与镜像已发出） | MCP Registry 那步绿 |
| A7 | 跨组织/联合身份的委派 | 目前按"同组织 + agent 名"解析，跨组织判未知链 | 要么支持，要么在规范里明确排除 |
| A8 | 控制台不会自动刷新 | 长开需要手动刷新 | 轮询或 SSE，且不打扰阅读 |
| A9 | 多租户与计费 | 一套台账、一个组织 | **明确不在 v1**（见 C） |

### B. 待你决定（产品决定，我不替你拍）

| # | 决定 | 选项 | 影响 |
|---|---|---|---|
| B1 | ~~要不要做 L3 执行点~~ **已做**（2026-09-25） | 选了第四条路：**不做代理/网关，做 PreToolUse 钩子**。工程最小、复用现成的判定引擎，且不进入亚信/360 那 13 家的平台战场 | 剩下的决定是**覆盖到哪**：(a) 只维护 Claude Code；(b) 再补 Codex 的包装脚本；(c) 才考虑 MCP 代理 |
| B2 | L1 定价与交付形态 | 按次审计 / 订阅 / 只卖给集成商 | 影响 GTM 与网站文案（现在写的是 `$1,500 按次`） |
| B3 | 要不要把控制面做成 SaaS | 自托管 / 托管 / 两者 | 与 A1+A9 是同一件事的不同深度 |

### C. 明确不做（写在这里，免得反复讨论）

模型对齐、提示词护栏、内容安全、跨客户情报、多租户计费、Windows 传感器。

## 五、测试与验收现状

| 项 | 现状 |
|---|---|
| Python | **151 项通过**（`cd service && python3 -m pytest -q`） |
| Go | **11 个包通过**（`go test ./...`） |
| CI | 每次 push / PR：`go vet` + `go test` + Python 测试 + **交付流水线冒烟**（要真产出 `events.jsonl`） |
| 跨语言口径 | `spec/testvectors/silence/events.jsonl`：Go 造、Python 验，两边各有一条"必须逐字相同" |
| UI 度量 | Playwright：桌面 1440 / 移动 390 无横向溢出；对比度全部 ≥4.5（正文 16.1 · 主按钮 8.79） |
| 设计检测器 | `impeccable detect` 清零 |
| 人工验收 | [VERIFY-LOCAL.md](VERIFY-LOCAL.md) 九步，步骤 2/4/4b/5/7/8 我实跑过并贴了输出 |

## 六、发布与部署现状（含红灯）

| 项 | 状态 |
|---|---|
| CLI 发布 | ✅ `v0.13.0`，11 个资产（四平台包 + sha256 + `install.sh`），下载验证过内含 `reach/contain/chain` |
| 容器镜像 | ✅ `asas-api` / `asas-sensor` 在华为 SWR，按 **digest** 钉版本；`values-*.yaml` 已同步 |
| 本机集群 | ✅ kind `ns=asas`：`asas-api` 1/1 Running，NodePort `8080:30090`，PVC `asas-data` |
| 控制台地址 | ✅ http://127.0.0.1:30090 （由 `asas-local-gateway` 容器接出来，重启机器仍在） |
| 服务器部署 | ❌ 未做（192.168.66.8 现在 SSH 都连不上，VPN 掉了） |
| Release 流水线 | ⚠️ 红在最后一步「Publish to the official MCP Registry」；日志要 admin 权限我读不到 |
| 网站 | ⚠️ 线上页面还是 `0.12.1` 且指向 `ratchet-0.12.1.mcpb`（该资产 404）；仓库里已改到 0.13.0，但**站点没重新部署** |

## 七、风险表

| 风险 | 触发信号 | 动作 |
|---|---|---|
| 无认证的控制面被暴露 | 有人把它放到公网 | 立刻下线；先做 A1 |
| 规范没人认 | 3 次外部评审都拿不到"可以当验收标准" | 降级为产品格式，不再自称标准 |
| 需求不存在 | 60 天 0 付费 | 按 GTM 止损线，拆成开源组件 |
| 演示翻车 | 现场控制面不通 | 演示前跑 [DEMO-10MIN.md](DEMO-10MIN.md) 末尾的三条检查 |

## 七之二、渠道可用性（推广侧，2026-09-23 记录）

> 为什么写在这里：`docs/DELIVERY-0.14.0.md` 里从 09-22 15:30 起连续多档记录"0 条"。
> 那不是"没有值得回的帖"，而是**本机没有给被墙站点用的出口**——两件事必须分开，
> 否则读文档的人会误判成"内容运营停了"。

| 渠道 | 状态 | 证据 |
|---|---|---|
| X（点对点回复） | ❌ 不可达 | 页面报 `Something went wrong. Try reloading.`；控制台 `x.com/i/api/*` → `HTTP-0 codes:[1004]`；宿主机 `curl https://x.com` → `HTTP 000`（10s 超时） |
| Reddit（old.reddit.com 路径） | ❌ 不可达 | `curl https://old.reddit.com` → `HTTP 000`；浏览器导航挂住直到工具超时 |
| HN（评论） | ❌ 不可达 | `curl https://news.ycombinator.com` → `HTTP 000` |
| GitHub（推送代码/文档） | ⚠️ 时通时断 | `curl https://github.com` 有时 200、有时 000；推送偶尔需要重试 |

**根因（2026-09-23 本机诊断）**：DNS 正常（`x.com → 172.66.0.227`），但 TCP connect 超时
（`Trying 172.66.0.227:443...` → `timed out`）；`ss -ltnp` 里 1080/7890/8080/8118/10808/10809/20171
**全无进程**；`gsettings org.gnome.system.proxy mode` = `'none'`；环境变量无 `http_proxy/https_proxy`；
EasyConnect 的 `tun0 = 2.0.1.1/24` 只承载公司网段，`ip route get 172.66.0.227` → `via 192.168.88.2 dev ens33`。
即：**当前没有任何通往被墙站点的代理/隧道**。恢复方式由运维决定（启动原来的代理程序，或切换
EasyConnect 的模式）；恢复后推广档照常，不需要改任何产品代码。

**自检命令（出问题时先跑这四条，30 秒能定位）**：

```bash
for u in https://x.com https://old.reddit.com https://news.ycombinator.com https://github.com; do
  printf '%-32s ' "$u"; curl -sS -o /dev/null --max-time 10 -w 'HTTP %{http_code}\n' "$u"; done
ss -ltn | grep -E ':(1080|7890|8080|8118|10808|10809|20171)\b' || echo '没有任何本地代理在监听'
gsettings get org.gnome.system.proxy mode          # 期望不是 'none'
ip route get 1.1.1.1 | head -1                      # 看是不是走了 tun0；走 ens33 说明没走隧道
```

判读：四个站点**全部 000** → 出口没了（本档情形）；**只有被墙站点 000、GitHub 200** →
普通外网正常、缺的是代理/隧道；**全部 200** → 渠道恢复，照常跑下一档。

## 八、下一步建议（按对"能交付"的影响排序）

1. **A1 认证**——不做这个，server 只能自己用，B1/B3 也都无从谈起。
2. **A2 服务器部署**——VPN 恢复后 `make deploy-local` 的同构流程 + 域名 + TLS。
3. **A5 + A6 发布收口**——mcpb 资产 + Registry 那步，让发版全绿（现在每次发版都留一个红叉）。
4. **A3 轮转语义**——它挡着"断流能不能直接影响合规判定"。
5. **执行点的下一步**——B1 已定（走 hook，不做代理）。接着要定的是覆盖到哪：
   只维护 Claude Code、还是再补 Codex 的包装脚本；见 B1 那一行的三个选项。

## 九、文档索引（哪个问题看哪份）

| 你想知道 | 看这份 |
|---|---|
| **现在做到哪了** | 本文件 |
| 三个角色分别是什么、术语对照 | [ROLES.md](ROLES.md) |
| 十分钟怎么演示给客户/投资人 | [DEMO-10MIN.md](DEMO-10MIN.md) |
| 怎么在本机 k8s 人工验一遍 | [VERIFY-LOCAL.md](VERIFY-LOCAL.md) |
| 开发流程与阶段判据 | [DEVELOPMENT-PLAN.md](DEVELOPMENT-PLAN.md) |
| 规范全文 / 给评审人 | [ASAS-v0.1.md](../spec/ASAS-v0.1.md) / [ASAS-SPEC-for-review.md](ASAS-SPEC-for-review.md) |
| 视觉世界与令牌 | [DESIGN.md](../DESIGN.md)、[PRODUCT.md](../PRODUCT.md) |
| 每一轮具体做了什么 | `docs/DELIVERY-*.md`（最新 0.18.0） |
| 渠道与推广 | [CHANNELS.md](CHANNELS.md)、[GTM.md](GTM.md) |
