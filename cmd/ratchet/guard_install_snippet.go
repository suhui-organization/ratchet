package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本文件把"在这个事件上跑 ratchet guard"翻译成九家 agent 各自的写法。
//
// 五种配置形状 + 两种非配置形态（丢文件 / 写插件）。抄错一种的后果不是报错，
// 而是**静默不生效**：客户以为装上了，其实没有。

// commandFor 是写进配置的那条命令。--harness 显式写进去，运行时就无需再猜是谁在调。
func commandFor(t installTarget, bin string) string {
	return bin + " guard --harness " + string(t.harness)
}

// clampExit 让钩子里的 shell 命令即便被包一层也不改语义。
// 保持简单：直接用命令本身。
func snippetJSON(t installTarget, bin, matcher string) map[string]any {
	cmd := commandFor(t, bin)
	inner := []any{
		map[string]any{"type": "command", "command": cmd, "timeout": 10},
	}
	switch t.shape {
	case shapeNamed:
		return map[string]any{
			hookSetName: map[string]any{
				"enabled": true,
				t.event: []any{
					map[string]any{"matcher": matcher, "hooks": inner},
				},
			},
		}
	case shapeUnwrapped:
		return map[string]any{
			t.event: []any{
				map[string]any{"matcher": matcher, "hooks": inner},
			},
		}
	case shapeCrushFlat:
		// 扁平条目：没有里面那层 {"hooks":[…]}
		return map[string]any{
			"hooks": map[string]any{
				t.event: []any{
					map[string]any{"matcher": matcher, "command": cmd, "timeout": 10},
				},
			},
		}
	case shapeCursor:
		// 多一个 version，事件名小驼峰，条目扁平
		return map[string]any{
			"version": 1,
			"hooks": map[string]any{
				t.event: []any{
					map[string]any{"matcher": matcher, "command": cmd},
				},
			},
		}
	default: // shapeHooksWrapped
		return map[string]any{
			"hooks": map[string]any{
				t.event: []any{
					map[string]any{"matcher": matcher, "hooks": inner},
				},
			},
		}
	}
}

// clineScript 是 Cline 要的可执行文件。
//
// Cline 没有配置文件——钩子就是约定目录里一个叫 PreToolUse 的可执行文件。
// 它的两套载荷（VS Code 扩展 / CLI）形状不同，但 ratchet guard 两边都认，
// 所以脚本只需要把 stdin 原样转过去。
//
// 为什么不用 `exec`：exec 会把后面的参数当命令，而我们要保留退出码语义。
// ratchet guard 从不靠退出码阻断（退出码在别的 agent 上会失效），
// 但这里仍然照常传递输出，让 Cline 自己去读 {"cancel":true}。
func clineScript(bin string) string {
	return `#!/bin/sh
# ratchet guard —— Cline 的 PreToolUse 钩子。
#
# Cline 没有配置文件：这个文件本身就是要装的钩子，文件名必须**正好**是 PreToolUse。
#   VS Code 扩展：<项目>/.clinerules/hooks/PreToolUse   （还要在设置里勾上 Enable Hooks）
#   CLI：        <项目>/.cline/hooks/PreToolUse.sh
#
# 载荷 stdin 直通 ratchet；ratchet 回 {"cancel":true,"errorMessage":…} 表示拦住。
` + bin + ` guard --harness cline
`
}

// openCodePlugin 是 OpenCode 要的插件文件。
//
// OpenCode 没有 shell 钩子，只有 JS/TS 插件。这里用 Node 内置的 execFileSync
// 而不是 OpenCode 的 `$` 助手：内置 API 的行为在所有版本上都一样，
// 而插件助手的确切签名会随版本变。
//
// 失败时**故意放行**（fail-open）：装坏一个插件不该让客户的 agent 完全不能用。
// 这与 ratchet guard 自己的失败策略一致——放行但把原因打到 stderr。
func openCodePlugin(bin string) string {
	return `// ratchet guard —— OpenCode 的 tool.execute.before 插件。
//
// OpenCode 没有 shell 钩子，所以拦截靠这个插件：工具执行前把调用交给 ratchet，
// ratchet 说拒绝就抛错。命令在下面的 RATCHET 里，换机器要改。
import { execFileSync } from "node:child_process"

const RATCHET = ` + "`" + bin + "`" + `

export const RatchetGuard = async () => ({
  "tool.execute.before": async (input, output) => {
    const payload = JSON.stringify({
      server: "opencode-tools",
      tool: input?.tool ?? "unknown",
      args: output?.args ?? {},
    })
    let raw = ""
    try {
      raw = execFileSync(RATCHET, ["guard", "--harness", "opencode"], {
        input: payload,
        encoding: "utf8",
      })
    } catch (err) {
      // 装坏了就放行，只告警——不能因为一个钩子把客户的 agent 整个锁死。
      console.error("ratchet guard 没能运行，本次未拦截：", err?.message ?? err)
      return
    }
    const line = raw.trim()
    if (!line) return
    let verdict
    try {
      verdict = JSON.parse(line)
    } catch {
      console.error("ratchet guard 的输出不是 JSON，本次未拦截：", line)
      return
    }
    if (verdict.decision === "deny") {
      throw new Error(verdict.reason || "已被 ratchet guard 拒绝")
    }
  },
})
`
}

// printSnippet 打印（默认不落盘）某一家的挂载方式。
func printSnippet(t installTarget, bin, matcher string) {
	home, _ := os.UserHomeDir()
	fmt.Printf("── %s（%s）%s\n", t.label, t.harness, verifiedMark(t))
	fmt.Printf("   目标文件：%s\n", filepath.Join(home, t.file))
	if t.trustNote != "" {
		fmt.Printf("   %s\n", t.trustNote)
	}
	fmt.Println()
	switch t.shape {
	case shapeFileDrop:
		fmt.Println(clineScript(bin))
	case shapePlugin:
		fmt.Println(openCodePlugin(bin))
	default:
		buf, err := json.MarshalIndent(snippetJSON(t, bin, matcher), "", "  ")
		if err != nil {
			return
		}
		fmt.Println(string(buf))
	}
	fmt.Println()
	fmt.Printf("   要我写：ratchet guard install --harness %s --write\n", t.harness)
}

func verifiedMark(t installTarget) string {
	if t.verified {
		return ""
	}
	return "  ⚠️ 配置路径待核实：只打印，不代写"
}

// writeSnippet 落盘。
//
// JSON 类形状走"合并"而不是覆盖：settings.json 里通常还有用户自己的其它配置，
// 覆盖掉别人的配置是不可接受的副作用。丢文件与插件两类则只写自己那一个文件。
func writeSnippet(t installTarget, targetPath, bin, matcher string) int {
	path := targetPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "找不到 HOME：%v\n", err)
			return 1
		}
		path = filepath.Join(home, t.file)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "建目录失败：%v\n", err)
		return 1
	}

	switch t.shape {
	case shapeFileDrop:
		// 钩子要是可执行的，否则一个字节都不会跑。
		if err := os.WriteFile(path, []byte(clineScript(bin)), 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败：%v\n", err)
			return 1
		}
	case shapePlugin:
		if err := os.WriteFile(path, []byte(openCodePlugin(bin)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败：%v\n", err)
			return 1
		}
	default:
		existing := map[string]any{}
		if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, &existing); err != nil {
				fmt.Fprintf(os.Stderr, "%s 已存在但不是合法 JSON，拒绝覆盖：%v\n", path, err)
				return 1
			}
		}
		fresh := snippetJSON(t, bin, matcher)
		switch t.shape {
		case shapeNamed:
			existing[hookSetName] = fresh[hookSetName]
		case shapeUnwrapped:
			existing[t.event] = fresh[t.event]
		default:
			hooks, _ := existing["hooks"].(map[string]any)
			if hooks == nil {
				hooks = map[string]any{}
			}
			hooks[t.event] = fresh["hooks"].(map[string]any)[t.event]
			existing["hooks"] = hooks
			if t.shape == shapeCursor {
				if _, ok := existing["version"]; !ok {
					existing["version"] = 1
				}
			}
		}
		out, err := json.MarshalIndent(existing, "", "  ")
		if err != nil {
			return 1
		}
		if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "写入失败：%v\n", err)
			return 1
		}
	}

	fmt.Printf("已写入 %s\n", path)
	if t.trustNote != "" {
		fmt.Println()
		fmt.Printf("还差一步：%s\n", strings.TrimSpace(t.trustNote))
	}
	return 0
}
