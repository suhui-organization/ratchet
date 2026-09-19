package hook

import (
	"encoding/json"
	"strings"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// ClaudeBuiltinServer 是 Claude Code 内置工具（Read/Write/Bash/…）的归属名。
// 它们不经过任何 MCP server，但同样是能力面的一部分。
const ClaudeBuiltinServer = "claude-code-tools"

// ClaudePayload 是 Claude Code 的 hook 输入（只取需要的字段）。
//
// 不读 tool_input：参数里常有文件内容与令牌，编译最小权限用不上。
type ClaudePayload struct {
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`
	SessionID      string          `json:"session_id"`
	CWD            string          `json:"cwd"`
	PermissionMode string          `json:"permission_mode"`
}

// FromClaude 翻译 Claude Code 的事件。
//
// 两类事件都收：
//   - PostToolUse      → decision=allow（它已经执行完了）
//   - PermissionDenied → decision=deny、outcome=blocked
//
// 收 deny 很关键：它记录的是"agent **想做**但被拦下的事"。
// 只看放行记录会低估它真实尝试过的能力面。
func FromClaude(raw []byte, server, agent string) (observe.Call, bool) {
	var p ClaudePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return observe.Call{}, false
	}
	tool := strings.TrimSpace(p.ToolName)
	if tool == "" {
		return observe.Call{}, false
	}

	call := observe.Call{Agent: orDefault(agent, "claude-code")}
	if s, name, ok := splitMCPTool(tool); ok {
		// MCP 工具：名字里就带着真实来源
		call.Server, call.Tool = s, name
	} else {
		call.Server, call.Tool = orDefault(server, ClaudeBuiltinServer), tool
	}
	// 显式给了 --server 时，覆盖 MCP 解析出来的 server（用于把多个 agent 归到同一策略下）
	if server != "" {
		call.Server = server
	}

	switch p.HookEventName {
	case "PermissionDenied":
		call.Decision, call.Outcome = "deny", "blocked"
	default:
		call.Decision = "allow"
		call.Outcome = outcomeOf(p.ToolResponse)
	}
	return call, true
}

// outcomeOf 从 tool_response 里判断这次调用是否出错。
//
// 只认结构里的显式错误标记，不去解析文本——猜错会把一次失败记成成功，
// 而"失败率"是后面判断某个工具该不该收掉的重要信号。
func outcomeOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "ok"
	}
	var probe struct {
		IsError  *bool   `json:"is_error"`
		IsError2 *bool   `json:"isError"`
		Error    *string `json:"error"`
		Success  *bool   `json:"success"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "ok"
	}
	if probe.IsError != nil && *probe.IsError {
		return "error"
	}
	if probe.IsError2 != nil && *probe.IsError2 {
		return "error"
	}
	if probe.Error != nil && *probe.Error != "" {
		return "error"
	}
	if probe.Success != nil && !*probe.Success {
		return "error"
	}
	return "ok"
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
