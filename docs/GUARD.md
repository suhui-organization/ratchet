# 执行点（拦截）策略

> 这份文件回答一个问题：**agent 要做一个错误动作时，这套东西凭什么能拦住它。**
> 代码在 `internal/guard`，入口是 `ratchet guard`。判定逻辑只有一份实现，
> 策略文件里写的就是执行点认的。

## 0. 一句话

**观测记录"发生过什么"，执行点在它发生之前把它停下。** 两者用同一份策略，
所以不存在"策略说拒、执行点不认"这种静默失效。

```
    agent 想调用一个工具
            │
            ▼
   ┌──────────────────┐    stdout: permissionDecision
   │  ratchet guard   │ ─────────────────────────────▶  allow / ask / deny
   │  (PreToolUse)    │
   └──────────────────┘
            │
            ├─▶ $RATCHET_HOME/guard.jsonl   拦截证据：依据、命中项、值指纹、策略指纹
            └─▶ $RATCHET_HOME/calls.jsonl   行为记录 → 自动进入哈希链事件流
```

## 1. 它和"审计"的分工

| | 审计（`policy` / `deliver`） | 执行点（`guard`） |
|---|---|---|
| 什么时候跑 | 编译期，看工具清单 | 运行期，看**这一次调用的真实参数** |
| 输入 | 工具名、描述、调用次数 | 工具名 + 参数值 |
| 产出 | 三态判定 + 依据 | allow / ask / deny + 可执行的动作 |
| 能不能阻止 | 不能。它是一份声明 | **能。这是它存在的全部理由** |

两边共用 `policy.Evaluate` 与 `policy.MatchGlob`——**不各写一份**。

## 2. 判定顺序

前面的规则优先。**规则 1、2 与工具本身的三态无关**：一个被标成 `allow` 的工具，
照样不能把受保护的东西写掉。

| # | 规则 | 条件 | 动作 |
|---|---|---|---|
| 1 | `guard.sensitive-target` | 参数值命中**凭据类**目标 | **拒绝**（读也拒） |
| 2 | `guard.protected-target` | 参数值命中**不可恢复**目标，且这次调用会写或删 | **拒绝** |
| 3 | `policy.*` | 策略的三态判定 | deny→拒绝 / approve→**人工确认** / allow→放行 |
| 4 | `guard.destructive` | 破坏性动作，且目标不在安全清单内 | **至少人工确认** |

### 2.1 为什么锚点是"目标"，不是"工具名"

同一个删除能力可以叫 `delete_file`，也可以叫 `rm`、`drop_table`，
还可以整句塞进一个 Bash 字符串里。禁工具名会被绕过；
而**被删掉的东西不会因为工具改名就变得可以恢复**。

所以规则 1、2 只看参数值里出现了什么目标。

### 2.2 为什么目标分成两类

这是整套策略里最容易做错的一处取舍：

| 类别 | 内容 | 何时拒 | 理由 |
|---|---|---|---|
| `sensitive` | `.env`、私钥、云凭证、kubeconfig | **任何操作** | 被读走与被删掉，代价一样，且读走不留痕 |
| `protected` | 备份、转储、`*.sql`、数据目录、生产目录、`.git`、系统路径 | **只在写或删时** | 读一份 `schema.sql` 是每天都在做的事 |

合成的代价是具体的：`**/*.sql` 一旦"任何操作都拒"，agent 连读迁移脚本都被拦，
客户当天就会关掉执行点——**保护强度归零，比有例外更糟**。
所以宁可多一层判断，也不把清单合成一张。

### 2.3 为什么"破坏性"要升级为人工确认

`Decide()` 对破坏性能力的默认映射是"拒绝"。执行点不能照抄这一条：
如果 agent 连删 `node_modules`、清 `build/` 都要人点确认，客户会关掉它。

执行点的做法是分档：

* 命中**不可恢复目标** → 拒绝（不可逆，没有商量余地）
* 破坏性 + 目标在 `safeTargets` 里 → 照策略走（通常放行）
* 破坏性 + 其它目标 → **人工确认**（有商量余地，但必须有人看一眼）

## 3. 破坏性动作藏在参数里

工具名经常完全中性。`shell`、`Bash`、`run_command` 会被分类成"执行"，
但它们要跑的那句可能正是 `rm -rf /srv/pgdata`。

所以执行点额外做一件事：**在参数值里找破坏性动词**。

* 只在看起来像命令/语句的值里找（`rm -rf /x`、`DROP TABLE users`）
* 只认纯字母的词，且复用策略侧同一份动词表（`policy.IsDestructiveToken`）

两条限制都是防误报：一个叫 `deleted_items.csv` 的文件名会被前缀匹配命中 `delete`，
那样的结果是每次读这个文件都弹确认——客户会直接关掉这个功能。

## 4. 失败模式（最重要的设计决定）

执行点自己出错时怎么办？两种选择都有代价：

| | 结果 |
|---|---|
| 全放 | 客户以为自己被保护着，其实没有 |
| 全拦 | 客户当场用不了自己的电脑 |

**默认选"放行，但让这件事可见"**：策略文件读不出来时，stderr 上一条明确的话，
`guard.jsonl` 里一条 `rule=guard.config-error` 的记录。沉默的失效才是不可接受的。

无人值守场景（CI、服务器）加 `--fail-closed`，那时配置故障直接拒绝。

## 5. 留痕与可验证性

每条判定都写进 `$RATCHET_HOME/guard.jsonl`：

```json
{"ts":"…","server":"filesystem","tool":"delete_file","decision":"deny",
 "rule":"guard.protected-target","reason":"拒绝：命中不可恢复的目标（…）",
 "matches":[{"argKey":"path","pattern":"**/backup/**","hash":"a0116a66a9fb9e05"}],
 "policyHash":"ee21aba935bba169"}
```

两个字段承担了这套东西的全部可信度：

* **`matches[].hash`** —— 值的 SHA-256 前 16 位。它证明"当时拦的就是这个值"，
  而不必把值写进日志。展示形式只有前 12 个字符。
* **`policyHash`** —— 策略文件原文的哈希。事后能回答"当时是规则松，
  还是执行点没生效"——这两件事的处理方式完全不同。

被拒的调用同时写进 `calls.jsonl`（`decision=deny, outcome=blocked`），
于是自动进入 `ratchet chain` 的哈希链事件流：**"某时某刻它挡下了一次删除"
是可验证的，不是一句转述。**

## 6. 安装

```bash
ratchet guard install --list                       # 看支持哪些 agent
ratchet guard install --harness codebuddy          # 打印要合并的配置（默认不替你写）
ratchet guard install --harness codebuddy --write  # 合并进那家的配置文件
ratchet guard install --all                        # 所有已知 agent 的片段一次打出
```

### 支持的 agent

| `--harness` | 产品 | 事件 | 配置文件 | 拒绝形状 |
|---|---|---|---|---|
| `claude-code` | Claude Code | `PreToolUse` | `~/.claude/settings.json` | `hookSpecificOutput.permissionDecision` |
| `codex` | Codex CLI | `PreToolUse` | `~/.codex/hooks.json` | 同上 |
| `codebuddy` | CodeBuddy（腾讯云） | `PreToolUse` | `~/.codebuddy/settings.json` | 同上 |
| `qwen-code` | Qwen Code（阿里） | `PreToolUse` | `~/.qwen/settings.json` | 同上 |
| `qoder` | Qoder / 通义灵码（阿里） | `PreToolUse` | `~/.qoder/settings.json` | 顶层 `{"decision":"deny"}` |
| `gemini-cli` | Gemini CLI（Google） | `BeforeTool` | `~/.gemini/settings.json` | `{"decision":"block"}` |
| `antigravity` | Antigravity CLI（Google） | `PreToolUse` | `~/.gemini/antigravity-cli/hooks.json` | `{"decision":"block"}` |
| `cursor` | Cursor | `preToolUse` | `~/.cursor/hooks.json` | `{"permission":"deny","user_message":…}` |
| `crush` | Crush（Charm） | `PreToolUse` | `~/.config/crush/crush.json` | 顶层 `{"decision":"deny"}` |
| `factory-droid` | Factory Droid | `PreToolUse` | `~/.factory/hooks.json` | `hookSpecificOutput.permissionDecision` |
| `cline` | Cline | `PreToolUse` | `.clinerules/hooks/PreToolUse`（**可执行文件**） | `{"cancel":true,"errorMessage":…}` |
| `opencode` | OpenCode | `tool.execute.before` | `.opencode/plugin/ratchet-guard.js`（**插件**） | 插件读到 deny 就抛错 |

十二家的路径都有官方文档佐证。

### 配置形状有五种，抄错一种就是静默不生效

同一样东西（"在这个事件上跑这条命令"）各家写法不同：

| 形状 | 长什么样 | 谁 |
|---|---|---|
| 标准 | `{"hooks":{"PreToolUse":[{"matcher":…,"hooks":[{…}]}]}}` | Claude Code、Codex、CodeBuddy、Qwen、Qoder、Gemini |
| 命名分组 | `{"ratchet-guard":{"enabled":true,"PreToolUse":[…]}}` | Antigravity |
| 无外层 | `{"PreToolUse":[{"matcher":…,"hooks":[…]}]}` | Factory Droid |
| 扁平条目 | `{"hooks":{"PreToolUse":[{"command":…,"matcher":…}]}}` | Crush |
| 带版本 + 小驼峰 | `{"version":1,"hooks":{"preToolUse":[{"command":…}]}}` | Cursor |

还有两家**根本不是配置**：

* **Cline** 的钩子是约定目录里一个叫 `PreToolUse` 的**可执行文件**，没有配置文件；
  而且必须在 Cline 设置里勾上「Enable Hooks」，否则它不跑。
* **OpenCode** 没有 shell 钩子，只有 JS/TS **插件**；装的是一个 `.js` 文件，
  它调用 `ratchet guard` 并在拒绝时抛错。

`guard install` 会按各家形状生成，别手抄。

### 装完之后还差一步

这几件事不做，钩子**不会运行**，而你看不出来：

* **Codex**：必须在 CLI 里跑一次 `/hooks`，审核并信任这条钩子。
  信任是按钩子内容哈希记的——以后改了命令要重新信任。
* **CodeBuddy**：配置不热加载。重启会话，并在 `/hooks` 面板审核一次。
* **Qwen Code**：项目级钩子要求该目录处于受信任状态。
* **Claude Code**：工作区信任对话框出现时选信任。
* **Qoder**：**不需要额外步骤**——官方明确说改完立即生效。

挂上之后先别急着信它，拿真实调用试一遍：

```bash
echo '{"server":"filesystem","tool":"delete_file","args":{"path":"/srv/backup/a.sql"}}' \
  | ratchet guard check
```

## 6.5 输出纪律：我们从不输出 `allow`

这是整套执行点里最容易被后来者无意破坏的一条，也是破坏之后**没有任何报错**的一条。

在 Claude Code 与 CodeBuddy 的文档里，`permissionDecision: "allow"` 的语义是
**"绕过权限系统直接执行"**。输出它等于替客户跳过了他自己的确认——
那不是"我没意见"，那是替人做决定。

而**沉默**（退出码 0、什么都不输出）才是真正的"没有意见"：
agent 的常规权限流程照常走。

所以：

| 判定 | 输出 |
|---|---|
| 拒绝 | 说出来（按各家形状） |
| 要求人工确认 | 该家支持就发 `ask`；不支持就**沉默**并往 stderr 说明 |
| 放行 | **什么都不输出** |

`TestAllowIsAlwaysSilent` 盯着这一条：它遍历所有 harness，断言输出里不出现 `allow`。

### 为什么 ask 在不支持的 agent 上降级成沉默，而不是拒绝

Codex 与 Gemini CLI 没有"交给人确认"这一档。把 ask 降级成拒绝听起来更安全，
但后果是具体的：策略把"写入"判为 `approve`，于是**每一次 `write_file` 都会变成硬失败**。
客户当天就会把执行点关掉——那时保护强度归零，比有例外更糟。

降级成沉默会让 agent 自己的权限流程接手，那是可接受的次优解。

### 各家协议的三个真实差异

这些不是文档洁癖，每一条错了的表现都是**静默不拦**：

| 差异 | 谁 | 错了会怎样 |
|---|---|---|
| 不支持 `ask` | Codex、Gemini CLI、Antigravity、Cursor、Crush、Cline、OpenCode | 带 `ask` 的输出被判成钩子失败，**然后继续执行工具** |
| 拒绝写在顶层 `decision` | Qoder、Gemini CLI、Antigravity、Crush | 发成 `hookSpecificOutput` 对方不认，等于没拦 |
| 退出码语义相反 | Gemini CLI 认退出码 2；**Antigravity 的非零退出码只写日志、不阻断** | 给 Gemini 写的"退出码 2"搬到 Antigravity 上就**拦不住** |
| 退出码 2 会丢弃 stdout | Qwen Code | 用退出码阻断会把拒绝理由一起丢掉 |
| `mcp_<server>_<tool>` 单下划线 | Gemini CLI、Antigravity | 拆不开，只能靠 `mcp_context.server_name` 取真实来源 |
| 返回 `allow` = 跳过用户确认 | Crush（官方原文明确）、Claude Code、CodeBuddy | 那不是"我没意见"，是替客户做了决定 |
| 理由要分两份 | Cursor（`user_message` + `agent_message`） | 只给一份：要么用户看不懂，要么 agent 不知道怎么改 |
| 参数在**顶层**不在 `tool_input` | Cursor 的 `beforeShellExecution` | 只看 `tool_input` 会拿到空参数，拦截形同虚设 |
| 参数值被**字符串化** | Cline 的 VS Code 形状 | `["/srv/backup"]` 整体当一个字符串，路径永远匹配不上 |

### 为什么统一不用退出码

上表第三行是踩出来的：Gemini CLI 与 Antigravity 在阻断机制上是**反的**。
但两家都认 stdout 上的 JSON（Gemini 官方把"退出码 0 + JSON"列为
"Preferred for all logic"），所以统一走 **退出码 0 + JSON**——
一份输出同时喂两家，而且避开"在另一家失效"这个坑。
`TestDenyNeverUsesExitCodes` 盯着这一条。

判定"现在是谁在调我们"的顺序是：**显式 `--harness` > 环境变量 > 载荷形状**。
环境变量排在形状前面，是因为 Claude / CodeBuddy / Qwen 三家的载荷几乎一模一样，
只靠形状会认错。各家钩子进程都继承了 agent 自己设的变量
（`CLAUDE_PROJECT_DIR` / `CODEBUDDY_PROJECT_DIR` / `QWEN_PROJECT_DIR`），那是最可靠的判据。

## 7. 它覆盖不到的地方（诚实清单）

* **走 agent 钩子的调用才拦得住。** 人自己敲的 `rm`、cron 脚本、
  别的 agent——不在范围内。
* **上面表里的十二家都有可阻断的钩子**，协议各有差异（见 6.5）。
  国内主流的"智能体平台"（扣子 Coze、阿里云百炼、腾讯元器/ADP、百度 AppBuilder、
  火山引擎 HiAgent…）走的是云端编排，**没有本地钩子**，要靠平台侧或 SDK 侧才能覆盖。
  这是另一条产品线，不在当前范围内。
* **不在表里的 agent 没有可靠拦截点**：月之暗面 Kimi CLI 目前还在 feature request
  阶段（没有钩子系统）；Aider 只有 git 钩子；Kilo Code 只有提示词级规则。
  它们只能观测，拦不住。**不要为了凑覆盖面假装支持**——一个装了却不生效的钩子，
  比明确说不支持更糟。
* **载荷认不出来时我们放行，但会明确告警。** 拦是错的（钩子认不出输入就阻断
  会把客户的 agent 卡死在无关的事情上），但静默返回 0 与"判定为放行"在日志里
  长得一模一样——而这两件事的含义完全不同。所以那条告警是必须的：
  它把"装了却从没生效过"这种最糟的失败形态变成可见的。
* **命令分析是启发式的。** base64 包一层、写个脚本再跑，
  都能绕过"在参数里找动词"这一层。目标规则（规则 1、2）拦的是直白形态，
  它拦不住精心构造的规避。
* **它不是沙箱。** 它拦的是它看得见的调用；看不见的事情它管不了。

## 8. 调参的地方

都在策略文件的 `guard` 段，改完重新 `policy draft` 或手改都行——
执行点读的就是这一份。

| 字段 | 默认 | 什么时候改 |
|---|---|---|
| `enabled` | `true` | 想先观察一段时间不拦，就先置 false（判定仍会记录） |
| `onApprove` | `ask` | 无人值守环境可改 `deny` |
| `protected` | 见 `policy.DefaultProtected` | 客户有自己的"不能碰"的目录 |
| `sensitive` | 见 `policy.SensitivePathDeny` | 与审计侧同一份口径，一般不用动 |
| `safeTargets` | **空** | 默认不让执行点替客户判断"什么算安全"；把 `policy.DefaultSafeTargets` 打开是一次明确的取舍 |

`safeTargets` 出厂为空是有意的：它是这套东西里唯一会**降低**拦截强度的开关，
该由客户自己按。
