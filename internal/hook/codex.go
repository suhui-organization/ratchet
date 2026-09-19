// Package hook 把 agent 的生命周期事件翻译成调用记录。
//
// 第一个接入的是 Codex 的 PostToolUse：它在每次工具调用结束后触发。
package hook

import (
	"encoding/json"
	"strings"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// DefaultServer 是 Codex 内置工具在记录里归属的 "server" 名。
//
// Codex 的内置 shell/exec 不经过任何 MCP server，但它们同样是能力面的一部分；
// 给它们一个稳定的归属名，才能在策略里被管起来。
const DefaultServer = "codex-tools"

// CodexPayload 是 PostToolUse 的输入（只取需要的字段）。
type CodexPayload struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	Model         string          `json:"model"`
}

// FromCodex 把一次 hook 事件翻译成调用记录。
//
// **刻意不记录 tool_input。** 参数里经常包含文件内容、命令行、令牌；
// 编译最小权限只需要 server + tool，多存一个字节就多一个泄露面。
// 需要取证时由调用方显式开启。
//
// 返回 ok=false 表示这条事件不该被记录（解析不出来，或已经过网关的重复调用）。
func FromCodex(raw []byte, server, agent string) (observe.Call, bool) {
	var p CodexPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return observe.Call{}, false
	}
	tool := strings.TrimSpace(p.ToolName)
	if tool == "" {
		return observe.Call{}, false
	}
	// 只有我们自己网关记过的调用才跳过（否则会重复计一次）
	if strings.HasPrefix(tool, "ratchet__") {
		return observe.Call{}, false
	}

	call := observe.Call{Agent: orDefault(agent, "codex")}
	if s, name, ok := splitMCPTool(tool); ok {
		// MCP 工具：名字里带着真实来源，直接拆开记。
		// （早先这里是把 mcp__ 全跳过的——那是假定网关会记一份；
		//   但 ratchet 现在没有网关，跳过等于把这些调用整段丢掉。）
		call.Server, call.Tool = s, name
	} else {
		call.Server, call.Tool = orDefault(server, DefaultServer), tool
	}
	if server != "" {
		call.Server = server // 显式覆盖：把多个 agent 归到同一策略下
	}
	call.Decision = "allow"
	call.Outcome = "ok"
	return call, true
}
