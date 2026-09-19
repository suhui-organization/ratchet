package hook

import (
	"encoding/json"
	"strings"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// FromGeneric 接受最简单的形状：{"server": "...", "tool": "..."}。
//
// 存在的意义：不是每个 agent 都有官方 hook。任何能往 stdout 打一行 JSON 的包装脚本
// 都能接进来，不必等我们为它写一个翻译器。
func FromGeneric(raw []byte, server, agent string) (observe.Call, bool) {
	var c struct {
		Server   string `json:"server"`
		Tool     string `json:"tool"`
		Agent    string `json:"agent"`
		Decision string `json:"decision"`
		Outcome  string `json:"outcome"`
		// 兼容 Codex/Claude 风格的字段名，方便直接透传
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return observe.Call{}, false
	}
	tool := strings.TrimSpace(c.Tool)
	if tool == "" {
		tool = strings.TrimSpace(c.ToolName)
	}
	if tool == "" {
		return observe.Call{}, false
	}
	// 若名字里带 mcp__server__tool 形状，同样拆开
	srv := strings.TrimSpace(c.Server)
	if s, name, ok := splitMCPTool(tool); ok {
		srv, tool = s, name
	}
	if srv == "" {
		srv = server
	}
	if srv == "" {
		return observe.Call{}, false // 没有来源的调用记下来也无法编译成策略
	}
	out := observe.Call{
		Server:   srv,
		Tool:     tool,
		Agent:    orDefault(c.Agent, orDefault(agent, "generic")),
		Decision: c.Decision,
		Outcome:  c.Outcome,
	}
	if out.Decision == "" {
		out.Decision = "allow"
	}
	if out.Outcome == "" {
		out.Outcome = "ok"
	}
	return out, true
}
