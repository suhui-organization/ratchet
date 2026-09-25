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
	HarnessAuto        Harness = "auto"
	HarnessCodex       Harness = "codex"
	HarnessClaude      Harness = "claude-code"
	HarnessCodeBuddy   Harness = "codebuddy"
	HarnessQwen        Harness = "qwen-code"
	HarnessQoder       Harness = "qoder"
	HarnessGemini      Harness = "gemini-cli"
	HarnessAntigravity Harness = "antigravity"
	HarnessCursor      Harness = "cursor"
	HarnessCrush       Harness = "crush"
	HarnessDroid       Harness = "factory-droid"
	HarnessCline       Harness = "cline"
	HarnessOpenCode    Harness = "opencode"
	HarnessGeneric     Harness = "generic"
)

// ParseHarness 归一化来源标识；认不出来的一律当 auto，由内容决定。
func ParseHarness(raw string) Harness { return ParseHarnessName(raw) }

// Detect 从载荷形状判断来源（包装内部实现，供 CLI 在解析与输出之间用同一个答案）。
func Detect(raw []byte) Harness { return detect(raw) }

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
// 先判各家的**独有**特征，最后才轮到"有 tool_name 就当 Codex"——
// 否则一个被裁剪过的 Claude payload（比如没有 transcript_path）会被
// 记成 Codex，多个 agent 混用时归因就错了。
//
// 注意：**光靠形状分不开 Claude / CodeBuddy / Qwen**——三家的 payload 几乎一样。
// 所以可靠的判据是环境变量（见 DetectFromEnv），形状只作为兜底。
// 好消息是这三家的输出协议也同构，认错在这一层不会造成"拒绝发错格式"。
//
// 各家独有特征：
//
//	Gemini   hook_event_name == "BeforeTool"（别家叫 PreToolUse）
//	Codex    turn_id / model（Codex 的两个扩展字段）
//	CodeBuddy generation_id；transcript_path 里有 .codebuddy/
//	Qwen     transcript_path 里有 .qwen/；base schema 带 timestamp
//	Claude   transcript_path / tool_use_id / permission_mode / tool_response /
//	         PermissionDenied 事件（Codex 不发这个）
func detect(raw []byte) Harness {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return HarnessGeneric
	}
	if name, ok := probe["hook_event_name"]; ok {
		var s string
		if json.Unmarshal(name, &s) == nil && s == "BeforeTool" {
			return HarnessGemini
		}
	}
	if _, ok := probe["toolCall"]; ok {
		// 反重力系的独有信封：{"toolCall":{"name":…,"args":…},"conversationId":…}
		return HarnessAntigravity
	}
	if _, ok := probe["conversationId"]; ok {
		return HarnessAntigravity
	}
	// Crush 用 `event` 标事件（别家用 hook_event_name），这是它的独有字段。
	if ev, ok := probe["event"]; ok {
		var s string
		if json.Unmarshal(ev, &s) == nil && (s == "PreToolUse" || s == "PostToolUse") {
			return HarnessCrush
		}
	}
	// Cline 两套载荷各有独有键
	if _, ok := probe["preToolUse"]; ok {
		return HarnessCline
	}
	if _, ok := probe["tool_call"]; ok {
		return HarnessCline
	}
	// Cursor 的 beforeShellExecution 独有 workspace_roots
	if _, ok := probe["workspace_roots"]; ok {
		return HarnessCursor
	}
	// Cursor 的 preToolUse 独有 model_params（别家不带模型的参数列表）
	if _, ok := probe["model_params"]; ok {
		return HarnessCursor
	}
	if _, ok := probe["turn_id"]; ok {
		return HarnessCodex
	}
	if _, ok := probe["generation_id"]; ok {
		return HarnessCodeBuddy
	}
	if tp, ok := probe["transcript_path"]; ok {
		var s string
		if json.Unmarshal(tp, &s) == nil {
			switch {
			case strings.Contains(s, "/.codebuddy/"):
				return HarnessCodeBuddy
			case strings.Contains(s, "/.qwen/"):
				return HarnessQwen
			case strings.Contains(s, "/.factory/"):
				// Factory Droid 的会话记录落在 .factory/projects/ 下
				return HarnessDroid
			}
		}
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

// argKeysOf 取工具输入的**键名**，值一律不读。
//
// 这是参数级约束的前提：知道"这个工具要传 path"，才谈得上限制它能传哪些路径。
// 而值（文件内容、命令行、令牌）一个字节都不留。
func argKeysOf(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
