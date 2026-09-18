# 交付记录 0.4.0 —— 采集侧

> 日期：2026-09-18。上一版见 [DELIVERY-0.3.0.md](DELIVERY-0.3.0.md)。

## 1. 闭环补齐

```
采集（本版）→ observe → policy draft --only-observed → 交付物 → 收货方独立验证
   ✅            ✅              ✅                    ✅            ✅
```

三端之前就通了，这一版补上最后缺的第一段。

| # | 产物 | 位置 | 状态 |
|---|---|---|---|
| 1 | 调用记录本地存放（JSONL，O_APPEND 原子追加） | `internal/store` | ✅ 测试通过 |
| 2 | Codex PostToolUse 事件翻译 | `internal/hook` | ✅ 测试通过 |
| 3 | `ratchet ingest`（从 stdin 读事件） | `cmd/ratchet` | ✅ |

## 2. 两条纪律

**① 绝不阻塞 agent。** 这个命令挂在交互式工具调用链上，它出错、超时、
或二进制不在，都不该让用户的工作流卡住。所以**任何异常都安静退出 0**；
写不进去时只往 stderr 说一句（hook 的 stderr 不进 agent 上下文）。

**② 默认不记录工具参数。** 事件里的 `tool_input` 常含文件内容、命令行、令牌，
而编译最小权限只需要 server + tool。测试里放了一个 `ghp_...` 做诱饵，
断言它不会出现在记录里——**多存一个字节就多一个泄露面**。

另外：已经过网关的 MCP 工具（`mcp__*` / `ratchet__*`）会被跳过，
避免同一次调用被记两遍。

## 3. 端到端实测（本机）

```console
$ printf '{"tool_name":"read_file","tool_input":{"command":"ghp_SHOULD_NOT_APPEAR"}}' | ratchet ingest
$ printf 'not json at all' | ratchet ingest ; echo $?
0                                    # 坏输入不阻塞
$ wc -l < $RATCHET_HOME/calls.jsonl
5                                    # 只有合法事件被记录
$ grep -c SHOULD_NOT_APPEAR $RATCHET_HOME/calls.jsonl
0                                    # 诱饵密钥没有落盘

$ ratchet observe --calls calls.jsonl --inventory inventory.json
  清单里      4 个工具
  被调用过    3
  从未被调用  1  ← 最小权限下这些应该被收掉

$ ratchet policy draft --from used.json --only-observed
  工具      3 → allow 1 · approve 2 · deny 0
  APPROVE  codex-tools/shell        名称命中「shell」；观测到 2 次调用
  APPROVE  codex-tools/write_file   名称命中「write」；观测到 1 次调用
  ALLOW    codex-tools/read_file    名称命中「read」；观测到 2 次调用
```

从未被调用过的 `delete_file` 不进策略，而未登记的工具默认被拒绝——等于被收掉了。

## 4. 怎么装（Codex）

把这一行加进 `~/.codex/hooks.json` 的 `PostToolUse`：

```json
{ "type": "command", "command": "/home/walden/Workspaces/ratchet/bin/ratchet ingest" }
```

> Codex 对非托管 hook 有信任闸门，未受信任的 hook 会被**静默跳过**。
> 信任步骤要在 Codex 侧完成后再依赖这条记录——否则会出现"装了但一条记录都没有"，
> 而没有任何报错。

记录默认落在 `$RATCHET_HOME/calls.jsonl`（默认 `~/.ratchet/calls.jsonl`），权限 0600。

## 5. 测试

```console
$ go test ./...
ok  internal/{discover,hook,mcp,observe,policy,store}
```

`store` 的并发用例同时起 40 个写入，断言**没有坏行**——hook 会被并发调用，
O_APPEND 保证每行原子落盘这件事必须被测住，否则并发下会产生读不回来的行。

## 6. 没做完的

| # | 事项 | 说明 |
|---|---|---|
| L1 | 其他 harness 的采集（Claude Code 等） | 目前只有 Codex；`hook` 包的结构已按可扩展写 |
| L2 | 交付目录里放验证器副本 / 报告渲染 | 未动 |
| L3 | 后端 API 与 30 天复验 | 未动 |
| L4 | 真机装上 hook 后长期观察 | 需要 Codex 侧的信任步骤，由使用者完成 |
