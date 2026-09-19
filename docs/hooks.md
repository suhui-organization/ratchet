# 把采集接到你的 agent 上

`ratchet ingest` 从 stdin 读一条事件，追加进本地调用记录。挂成 hook 之后，
每次工具调用都会记一行。它**解析失败就静默退出**，绝不打断你的 agent。

先确认命令能跑：

```bash
command -v ratchet          # 装到 ~/.local/bin 的话，这里会打出来
ratchet version
```

> 下面一律用**绝对路径**。hook 的执行环境不一定继承你的 `PATH`，
> 用相对名会在"命令找不到"和"记不上"之间反复折腾，而且没有任何报错。

---

## Claude Code

写进 `~/.claude/settings.json`：

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          { "type": "command", "command": "/home/你/.local/bin/ratchet ingest" }
        ]
      }
    ],
    "PermissionDenied": [
      {
        "hooks": [
          { "type": "command", "command": "/home/你/.local/bin/ratchet ingest" }
        ]
      }
    ]
  }
}
```

**`PermissionDenied` 也要挂**。它记的是"agent 想做、但被权限系统拦下的事"——
只看放行记录会低估它真正尝试过的能力面。

### 为什么 Claude Code 特别值得接

Claude Code 的 MCP 工具名是 `mcp__<server>__<tool>`，**名字里自带来源**。
所以记录会直接写成 `filesystem/read_file`，而不是全塞进一个桶里——
后者会让策略编译分不清"哪个 server 的哪个工具"。

内置工具（`Read`/`Write`/`Bash`/…）归到 `claude-code-tools` 下，
它们是能力面的一部分，只是不经过任何 MCP server。

---

## Codex

写进 `~/.codex/hooks.json`：

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "hooks": [
          { "type": "command", "command": "/home/你/.local/bin/ratchet ingest" }
        ]
      }
    ]
  }
}
```

> Codex 对非托管 hook 有信任闸门：未受信任的 hook 会被**静默跳过**。
> 表现是"装了但一条记录都没有"，而且没有任何报错。信任步骤要在 Codex 侧完成。

---

## 任何别的工具（generic）

只要能往 stdout 打一行 JSON，就能接进来：

```json
{"server": "my-tool", "tool": "do_thing", "decision": "allow", "outcome": "ok"}
```

```bash
echo '{"server":"my-tool","tool":"do_thing"}' | ratchet ingest --hook generic
```

`server` 是必需的——没有来源的调用记下来也没法编译成策略，脚本会拒绝它。

---

## 事件是怎么被认出来的

`--hook` 默认 `auto`，按 payload 的字段形状判断，不靠猜：

| 看到什么 | 判定 |
|---|---|
| `transcript_path` / `tool_use_id` / `permission_mode` / `tool_response` | Claude Code |
| `hook_event_name` = `PermissionDenied` | Claude Code（Codex 不发这个事件） |
| `model` | Codex |
| 只有 `tool_name` | Codex |
| 只有 `server` + `tool` | generic |

顺序是有意的：先判完 Claude 的**全部**特征，最后才轮到"有 `tool_name` 就当 Codex"。
否则一个被裁剪过的 Claude payload 会被记成 Codex，多 agent 混用时归因就错了。

要强制指定：`--hook claude-code` / `--hook codex` / `--hook generic`。

---

## 记录里有什么、没有什么

记：

| 字段 | 说明 |
|---|---|
| `server` / `tool` | 编译最小权限策略唯一必需的两项 |
| `agent` | 谁做的（多 agent 归因） |
| `decision` / `outcome` | allow·approve·deny / ok·error·blocked |
| `ts` | 时间戳（UTC） |

**不记**：工具参数。

参数里常有文件内容、命令行、令牌——而编译最小权限不需要它们。
少存一个字节就少一个泄露面。这一条有测试守着：用例里放了一个 `ghp_...` 诱饵，
断言它不会出现在记录的任何字段里。

记录默认落在 `~/.ratchet/calls.jsonl`（权限 0600，可用 `RATCHET_HOME` 或 `--store` 改）。

---

## 装完之后

```bash
ratchet scan --introspect --out inventory.json     # 这台机器上有什么
ratchet observe --calls ~/.ratchet/calls.jsonl --inventory inventory.json --out used.json
ratchet policy draft --from used.json --only-observed --out policy.json
```

用几天之后，`observe` 会告诉你哪些工具**从来没被调用过**——
那些就是可以安全收掉的部分。
