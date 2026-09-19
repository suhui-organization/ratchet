// Package hook 把各家 agent 的生命周期事件翻译成同一种调用记录。
//
// 目前接入三家：
//
//	codex        Codex 的 PostToolUse
//	claude-code  Claude Code 的 PostToolUse / PermissionDenied
//	generic      任何工具直接给 {server, tool} 的简单形状
//
// 共同纪律（每个翻译器都必须遵守）：
//  1. **不记录工具参数**——参数里常有文件内容、命令行、令牌，而编译最小权限不需要它；
//  2. **解析失败就返回 false**，绝不猜一个记录出来；
//  3. **绝不阻塞 agent**：这一层不做 IO，写盘失败由调用方静默处理。
package hook

import (
	"encoding/json"
	"strings"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// Harness 是事件来源的标识。
type Harness string

const (
	HarnessAuto    Harness = "auto"
	HarnessCodex   Harness = "codex"
	HarnessClaude  Harness = "claude-code"
	HarnessGeneric Harness = "generic"
)

// ParseHarness 归一化来源标识；认不出来的一律当 auto，由内容决定。
func ParseHarness(raw string) Harness {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "codex":
		return HarnessCodex
	case "claude", "claude-code", "claudecode":
		return HarnessClaude
	case "generic", "raw":
		return HarnessGeneric
	}
	return HarnessAuto
}

// Translate 把一条原始事件翻译成调用记录。
//
// server / agent 为空时用各家的默认值（调用方可用 --server / --agent 覆盖）。
func Translate(raw []byte, h Harness, server, agent string) (observe.Call, bool) {
	if h == HarnessAuto {
		h = detect(raw)
	}
	switch h {
	case HarnessClaude:
		return FromClaude(raw, server, agent)
	case HarnessGeneric:
		return FromGeneric(raw, server, agent)
	default:
		return FromCodex(raw, server, agent)
	}
}

// detect 从字段形状判断来源。
//
// 判据是各家**独有的**字段，不是靠猜。顺序很重要：
// 先判 Claude 的**全部**特征，最后才轮到"有 tool_name 就当 Codex"——
// 否则一个被裁剪过的 Claude payload（比如没有 transcript_path）会被
// 记成 Codex，多个 agent 混用时归因就错了。
//
// Claude Code 的特征：有会话记录路径、有工具调用 id、有权限模式、
// 回传 tool_response、或发出 PermissionDenied（Codex 不发这个事件）。
// Codex 的特征：payload 里带 model。
func detect(raw []byte) Harness {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return HarnessGeneric
	}
	for _, key := range []string{"transcript_path", "tool_use_id", "permission_mode", "tool_response"} {
		if _, ok := probe[key]; ok {
			return HarnessClaude
		}
	}
	if event, ok := probe["hook_event_name"]; ok {
		var name string
		if json.Unmarshal(event, &name) == nil && name == "PermissionDenied" {
			return HarnessClaude
		}
	}
	if _, ok := probe["model"]; ok {
		return HarnessCodex
	}
	if _, ok := probe["tool_name"]; ok {
		return HarnessCodex
	}
	return HarnessGeneric
}

// splitMCPTool 拆开 `mcp__<server>__<tool>`。
//
// 这是接入 Claude Code 最值钱的一点：它的 MCP 工具名里**自带来源**，
// 所以能直接记成 server/tool，而不是全塞进一个 "claude-code-tools" 桶里——
// 后者会让策略编译分不清"哪个 server 的哪个工具"。
func splitMCPTool(tool string) (server, name string, ok bool) {
	const prefix = "mcp__"
	if !strings.HasPrefix(tool, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(tool, prefix)
	idx := strings.Index(rest, "__")
	if idx <= 0 || idx+2 >= len(rest) {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
}
