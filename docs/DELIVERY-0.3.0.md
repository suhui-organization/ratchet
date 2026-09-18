# 交付记录 0.3.0 —— 观测与最小权限

> 日期：2026-09-18。上一版见 [DELIVERY-0.2.0.md](DELIVERY-0.2.0.md)。

## 1. 这一版加了什么

把**真实发生过的调用**接进来，让策略能区分"它确实在用"和"只是挂在那里"。

| # | 产物 | 位置 | 状态 |
|---|---|---|---|
| 1 | 调用记录读取与聚合（JSONL 契约） | `internal/observe` | ✅ 测试通过 |
| 2 | 能力面 vs 使用面的比对 | `internal/observe` | ✅ |
| 3 | `ratchet observe` 命令 | `cmd/ratchet` | ✅ |
| 4 | `policy draft --only-observed` | `internal/policy` | ✅ |

## 2. 调用记录的契约

一行一次调用，**只有 `server` 与 `tool` 是必需的**：

```jsonl
{"ts":"2026-09-18T10:00:00Z","agent":"codex","server":"filesystem","tool":"read_file"}
```

契约刻意做得窄：越窄越容易从各种来源喂进来（hook、网关日志、厂商导出）。
坏行**计入 `skipped` 并报出来**，不静默当成"没有调用"——
全部无法解析时直接报错，避免"读错文件"被当成"这个 server 很干净"。

## 3. 它说出的话

```console
$ ratchet observe --calls calls.jsonl --inventory inventory.json --out inventory-used.json
  调用记录  6 条

  清单里      5 个工具
  被调用过    3
  从未被调用  2  ← 最小权限下这些应该被收掉
  清单之外    2  ⚠ 被调用过但不在清单里：要么扫描漏了，要么有人绕过了配置

从未被调用的工具（按最小权限应当移除）
  filesystem/delete_file
  github/get_issue

清单之外的调用（先查清来源，再决定是补清单还是堵绕过）
  github/create_pull_request         1 次
  ghost/mystery_tool                 1 次
```

**"清单之外的调用"是这一版最重要的信号**：配置里没有、却真实发生过。
它只可能有两个原因——扫描不全，或者有人绕过了配置。两种都值得立刻查。

## 4. 最小权限的落点

```console
$ ratchet policy draft --from inventory-used.json --only-observed
  工具      3 → allow 2 · approve 1 · deny 0
  APPROVE  filesystem/write_file       名称命中「write」；观测到 1 次调用
  ALLOW    filesystem/list_directory   名称命中「list」；观测到 1 次调用
  ALLOW    filesystem/read_file        名称命中「read」；观测到 2 次调用
```

`--only-observed` 把 5 个工具收成 3 个：从未被调用过的 `delete_file` 与
`get_issue` 不进策略，而未登记的工具默认被拒绝——**等于被收掉了**。

这就是产品对外那句话的字面实现：**不授予没有观察到的能力**。

## 5. 测试

```console
$ go test ./...   →  discover / mcp / observe / policy 全绿
$ python3 -m pytest -q  →  14 passed
$ npm run test          →   8 passed
```

观测层覆盖：坏行计入 skipped、清单之外被识别、`Apply` 保留元信息、
空文件与全坏文件都必须报错。

## 6. 没做完的

| # | 事项 | 说明 |
|---|---|---|
| L1 | **采集侧**：hook / 网关产生调用记录 | 现在只消费 JSONL，不生产；这是下一个大件 |
| L2 | 更多 harness 方言 + 远程 server 的 introspect | 同 0.2.0 |
| L3 | 报告渲染 / 后端 API / 30 天复验 | 未动 |

**L1 是关键缺口**：没有采集侧，用户得自己弄一份调用记录出来——
而那正是"5 分钟拿到一份策略"这个演示最容易被卡住的地方。
