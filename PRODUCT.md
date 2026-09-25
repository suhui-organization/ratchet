# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

> 本文件在 `impeccable init` 流程中写成。**访谈工具在本会话不可用**（`request_user_input`
> 报 "unavailable in Default mode"），所以带 `[推断]` 的条目来自仓库证据与用户明确的
> 口头要求，不是用户逐条确认过的事实。未确认的推测不写进来。

## Users

**[推断]** 两类人，同一份产物：

1. **交付方**：负责把 AI agent 接进业务系统的平台/安全工程师。他要回答的问题只有一个——
   "这些 agent 到底被允许碰什么，凭什么"（用户原话：企业客户不了解 agent 安全，
   我们还要负责培养认知）。
2. **收货方**：客户的安全负责人、审计员、或者上级甲方。他**不装任何东西**，
   拿到凭据就要能独立判断，且**不需要信任签发方**。

机器上还有第三种"使用者"：无人值守的传感器（CronJob），它按天采集并上报。

## Product Purpose

从 agent 的真实行为编译最小权限策略，产出**可被第三方离线独立验证**的凭据，
并用一个台账做持续验证、委派边界与遏制。[推断] 成功 = 收货方在不信任我们的前提下
也能得出结论；失败 = 产出一份"看起来很干净但没法核验"的报告。

## Positioning

**面向收货方的可验证性**：凭据 + 证据 + 事件流三者互相钉住，验证方离线重算即可判真伪。
配套的 ASAS 规范是公开的（CC-BY-4.0），出现 1 个外部实现 + 1 个审计方引用后移交中立组织。
同类产品都在卖"我们帮你看住了"，这里卖的是"**别人能自己验**"。

## Operating Context

**[推断/事实混合，来自仓库与本次会话]**

* CLI：`ratchet`（Go，单二进制，一条命令安装），命令 `scan / deliver / chain / reach / contain / policy / mcp ...`。
* 控制面：`asas-api`（Python 标准库），跑在 k8s 里，台账落 PVC（sqlite）。
* 传感器：容器镜像 + CronJob，采集 → 出证 → 上报 → 传证据 → 传事件 → 复验。
* 本机验证环境：kind 集群 `ns=asas`，地址 `http://127.0.0.1:30090`。
* 运维动作：`make deploy-local`、`make local-gateway`、`kubectl -n asas create job --from=cronjob/asas-sensor x`。
* 现场条件：内网、无外网（本机到 npm registry 不通），所以任何界面都不能依赖 CDN。

## Capabilities and Constraints

**能力**：`/healthz` `/attestations` `/verify` `/evidence` `/events` `/contain` `/reach` `/silence`。
九条验证规则（hashMatch / narrowingOnly / delegationNarrows / containmentCascades /
noUndeclaredUnknown / everyVerdictHasBasis / allPinnedOrExempt / silenceIsAuditable /
withinValidityWindow）。

**约束（都是硬约束）**：

* 控制面镜像**只用标准库**——一个自己需要一堆依赖的控制面，部署本身就是问题；
  因此控制台 UI **不能有构建步骤、不能引 CDN**，必须是随镜像一起走的静态资源。
* **控制面没有任何认证**：界面必须显式标注这一点，不能让人误以为它可以对外。
* 单租户、单副本、sqlite；没有备份与 HA。
* 判定逻辑只有一份：界面**绝不重新实现验证规则**，只调用同一份后端实现。

## Brand Commitments

* 名字：**Ratchet**。**公开站点中英双语**（`/` 英文，`/zh` 中文，路径前缀区分，
  两棵文案树同形状、由测试强制对齐）；内部文档与 CLI 输出为中文。
  法律条款页也有中文，但**是以译本的方式做的**：每份中文文档顶部有一段说明，
  写明如有歧义以英文版为准（`web/app/content/legal.ts` 的 `governingNote`）。
  条款的解释权只能有一个版本，两个语种各自为政时出问题无法判断以谁为准。
  译者能负责的边界是"忠实翻译 + 明确优先顺序"，法律意见仍然需要另找。
* **[用户明确要求]** 视觉上继承已上线的站点（`web/app/assets/css/main.css`）：
  浅色底、系统无衬线、代码用 JetBrains Mono、品牌色 `#4d6bfe`、`--color-ink:#202124`。
  新界面不另起一套视觉世界。
* 语气：直说事实与数字，不吹、不掩盖缺口。"看起来坏了就是坏了"——空状态、错误、
  未评估都要写清楚，不能让人误读成通过。

## Evidence on Hand

* 真机数据：本机 16 个 MCP server、12 个未定版（凭据里逐条带 `pinned:false`）。
* 跨语言一致性向量：`spec/testvectors/silence/events.jsonl`。
* 验收记录：`docs/DELIVERY-0.1[5-8].0.md`、人工验证清单 `docs/VERIFY-LOCAL.md`。
**不存在的**：客户案例、第三方评测、认证背书——界面里一律不得出现。

## Product Principles

1. **判定逻辑只有一份**；界面与报告都是它的消费者。
2. **三态而不是布尔**：pass / fail / not_evaluated。把"没检查"说成"通过"，
   比没有验证器更糟。
3. **不填假值**：拿不到的事实进 `unknown[]`，不猜一个看起来合理的数。
4. **默认值不许生产出失败**（实测踩过：默认委派到期比父晚几秒，一份默认生成的凭据直接违规）。
5. **沉默要被看见**：断流是事件，不是日志；界面必须把它放在显眼处。

## Roadmap：三级，别混着讲

故事混乱的根因是**把两个产品混在一起讲**。分开就不乱了：

| 级 | 是什么 | 今天到哪了 | 买家是谁 |
|---|---|---|---|
| **L1 证据** | 一台机器一条命令 → 一份**第三方可独立验证**的凭据（一次审计/一次交付） | ✅ 已可用 | 被客户/审计追着要证据的平台工程师 |
| **L2 台账** | 多台机器、多天 → 汇总、跨次比对、触达查询、遏制记录（持续） | ✅ 本机 k8s 已跑 | 要持续回答"现在谁还能碰什么"的安全负责人 |
| **L3 执行点** | 代理 / 网关 / SDK 拦截：把"能碰"变成"**只能**碰" | ❌ **没有** | 要强制执行最小权限的组织 |

**明确不做的**：多租户与计费、跨客户情报、内容安全与模型对齐。

**L1 与 L2 的关系**：L2 不是 L1 的升级版，而是 L1 在时间与机器数量上的展开——
收货方永远只需要 L1 的产物（凭据 + 证据），**不需要** L2 在线。
这条是护城河：客户不信任我们时，答案依然成立。
