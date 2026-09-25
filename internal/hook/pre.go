package hook

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ArgValue 是一次调用里**一个具体的参数值**。
//
// Key 可能是 "path"，也可能是 "paths[0]"、"env.TOKEN"——展平后的路径，
// 这样嵌套结构里的值也能被逐个拿去匹配，而不是被压成一坨字符串后漏判。
type ArgValue struct {
	Key   string
	Value string
}

// PreCall 是一次**尚未执行**的调用。
//
// 与 observe.Call 的关键区别：这里**带参数值**。
//
// 为什么观测侧不读值而这里必须读：观测只需要知道"这个工具要传 path"就能编译策略；
// 而拦截必须知道"你准备删的是哪个目录"才能下判断。两者是不同的问题，
// 所以用不同的结构，不为了复用把观测侧那条纪律破掉。
//
// 值只用于**当场判定**：落盘时只写值的哈希与被命中的模式，不写值本身。
type PreCall struct {
	Agent  string
	Server string
	Tool   string
	// Args 是顶层参数（键 → 字符串化的值），供策略的参数约束判定使用。
	Args map[string]string
	// Values 是展平后的全部叶子值，供受保护目标扫描使用。
	Values []ArgValue
}

// TranslatePre 把一条 PreToolUse 事件翻译成待判定调用。
//
// server/agent 为空时用各家默认值；显式给 server 时覆盖（把多个 agent 归到同一策略下）。
func TranslatePre(raw []byte, h Harness, server, agent string) (PreCall, bool) {
	if h == HarnessAuto {
		h = detect(raw)
	}
	var (
		tool string
		in   json.RawMessage
	)
	switch h {
	case HarnessClaude, HarnessCodeBuddy, HarnessQwen, HarnessQoder, HarnessGemini,
		HarnessCrush, HarnessDroid:
		// 这五家的 PreToolUse/BeforeTool 输入是同一形状：
		// tool_name + tool_input（+ cwd / permission_mode）。
		// 复用一个结构体是有意的——协议一致的地方就不要写五份。
		// （Crush 用 event 而不是 hook_event_name 标明事件，但工具字段同形；
		//   Factory Droid 官方文档给的也是这套 snake_case 字段。）
		var p ClaudePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		tool, in = p.ToolName, p.ToolInput
	case HarnessCursor:
		// Cursor 有两个能拦的事件，**输入形状不一样**：
		//   preToolUse          → {tool_name, tool_input}（覆盖所有工具，但 ask 不被强制执行）
		//   beforeShellExecution→ {command, cwd, …}（参数在顶层，且支持 ask）
		//
		// beforeShellExecution 的命令在顶层 `command` 上，不在 tool_input 里——
		// 只认 tool_input 的解析器在这里会拿到空参数，于是"要删哪个目录"完全看不到，
		// 拦截形同虚设。
		var p struct {
			HookEventName string          `json:"hook_event_name"`
			ToolName      string          `json:"tool_name"`
			ToolInput     json.RawMessage `json:"tool_input"`
			Command       string          `json:"command"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		if p.ToolName != "" {
			tool, in = p.ToolName, p.ToolInput
		} else {
			// 把顶层 command 折成 tool_input 的形状，后面的逻辑就不必分叉
			tool = "Shell"
			in = mustJSONObject(map[string]string{"command": p.Command})
		}
	case HarnessCline:
		// Cline 有**两套互不相同的载荷**（官方示例脚本里明确两边都兼容）：
		//   VS Code 扩展：.preToolUse.toolName + .preToolUse.parameters（值被 JSON 字符串化）
		//   CLI / SDK：   .tool_call.name   + .tool_call.input
		var p struct {
			PreToolUse struct {
				ToolName   string          `json:"toolName"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"preToolUse"`
			ToolCall struct {
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"tool_call"`
			ToolName  string          `json:"tool_name"`
			ToolInput json.RawMessage `json:"tool_input"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		switch {
		case p.ToolCall.Name != "":
			tool, in = p.ToolCall.Name, p.ToolCall.Input
		case p.PreToolUse.ToolName != "":
			tool, in = p.PreToolUse.ToolName, p.PreToolUse.Parameters
		default:
			tool, in = p.ToolName, p.ToolInput
		}
	case HarnessOpenCode:
		// OpenCode 的载荷是我们自己的插件生成的（见 guard install --harness opencode），
		// 形状由我们定：就是 generic 那套 {server, tool, args}。
		var p struct {
			Server string            `json:"server"`
			Tool   string            `json:"tool"`
			Args   map[string]string `json:"args"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		if p.Tool == "" {
			return PreCall{}, false
		}
		call := PreCall{Agent: orDefault(agent, "opencode"), Server: orDefault(p.Server, "opencode-tools"), Tool: p.Tool}
		for k, v := range p.Args {
			call.Values = append(call.Values, ArgValue{Key: k, Value: v})
		}
		sort.Slice(call.Values, func(i, j int) bool { return call.Values[i].Key < call.Values[j].Key })
		call.Args = p.Args
		if server != "" {
			call.Server = server
		}
		return call, true
	case HarnessAntigravity:
		// Antigravity 用的是嵌套信封：
		//   {"toolCall":{"name":"run_command","args":{"CommandLine":"…"}},…}
		// 注意参数键是 **CommandLine**（大写），不是 command——
		// 所以拦截判定绝不能依赖参数名，只能依赖值（见 guard 的扫描逻辑）。
		var p struct {
			ToolCall struct {
				Name string          `json:"name"`
				Args json.RawMessage `json:"args"`
			} `json:"toolCall"`
			CWD       string          `json:"cwd"`
			ToolName  string          `json:"tool_name"`
			ToolInput json.RawMessage `json:"tool_input"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		if p.ToolCall.Name != "" {
			tool, in = p.ToolCall.Name, p.ToolCall.Args
		} else {
			tool, in = p.ToolName, p.ToolInput // 兼容 Claude 形状
		}
	case HarnessGeneric:
		var p struct {
			Server   string          `json:"server"`
			Tool     string          `json:"tool"`
			ToolName string          `json:"tool_name"`
			Args     json.RawMessage `json:"args"`
			Input    json.RawMessage `json:"tool_input"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		tool = strings.TrimSpace(p.Tool)
		if tool == "" {
			tool = strings.TrimSpace(p.ToolName)
		}
		server = orDefault(server, strings.TrimSpace(p.Server))
		in = p.Args
		if len(in) == 0 {
			in = p.Input
		}
	default: // Codex
		var p CodexPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return PreCall{}, false
		}
		tool, in = p.ToolName, p.ToolInput
	}

	tool = strings.TrimSpace(tool)
	if tool == "" {
		return PreCall{}, false
	}

	call := PreCall{Agent: orDefault(agent, defaultAgentFor(h))}
	if s, name, ok := splitMCPTool(tool); ok {
		call.Server, call.Tool = s, name
	} else {
		call.Server = orDefault(server, defaultServerFor(h))
		call.Tool = tool
	}
	// Gemini 系的 MCP 工具名是 mcp_<server>_<tool>（**单下划线**），
	// 和 Claude 系的 mcp__<server>__<tool> 不同，而且单下划线**无法可靠拆分**
	// （server 名里本身就可能带下划线）。所以能拿到 mcp_context 就用它，
	// 拿不到就保留整名——猜错的代价是策略里匹配不上，那等于没配。
	if h == HarnessGemini || h == HarnessAntigravity {
		if srv := geminiMCPServer(raw); srv != "" {
			call.Server = srv
		}
	}
	if server != "" {
		call.Server = server
	}
	call.Values = flattenArgs(in)
	call.Args = topLevelArgs(call.Values)
	return call, true
}

func defaultAgentFor(h Harness) string {
	switch h {
	case HarnessClaude:
		return "claude-code"
	case HarnessCodeBuddy:
		return "codebuddy"
	case HarnessQwen:
		return "qwen-code"
	case HarnessQoder:
		return "qoder"
	case HarnessGemini:
		return "gemini-cli"
	case HarnessAntigravity:
		return "antigravity"
	case HarnessCursor:
		return "cursor"
	case HarnessCrush:
		return "crush"
	case HarnessDroid:
		return "factory-droid"
	case HarnessCline:
		return "cline"
	case HarnessOpenCode:
		return "opencode"
	case HarnessGeneric:
		return "generic"
	}
	return "codex"
}

func defaultServerFor(h Harness) string {
	switch h {
	case HarnessClaude:
		return ClaudeBuiltinServer
	case HarnessCodeBuddy:
		// 内置工具（Read/Write/Bash/Task/Glob/Grep…）同样不经过 MCP server，
		// 给它们一个稳定的归属名才能在策略里被管起来。
		return "codebuddy-tools"
	case HarnessQwen:
		// Qwen/Gemini 系的工具 id 是 run_shell_command / write_file / read_file 这种。
		return "qwen-tools"
	case HarnessQoder:
		return "qoder-tools"
	case HarnessGemini:
		return "gemini-tools"
	case HarnessAntigravity:
		return "antigravity-tools"
	case HarnessCursor:
		return "cursor-tools"
	case HarnessCrush:
		return "crush-tools"
	case HarnessDroid:
		return "factory-tools"
	case HarnessCline:
		return "cline-tools"
	case HarnessOpenCode:
		return "opencode-tools"
	}
	return DefaultServer
}

// mustJSONObject 把 map 编成 JSON，失败时返回 nil。
// 只用于把"参数在顶层"的事件折成 tool_input 的形状。
func mustJSONObject(m map[string]string) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

// geminiMCPServer 从 mcp_context 里取出 MCP server 名。
//
// Gemini/反重力系的 MCP 工具名是 mcp_<server>_<tool>，单下划线无法可靠拆开，
// 所以只有这一条路能拿到真实来源；取不到就返回空，由调用方保留整名。
func geminiMCPServer(raw []byte) string {
	var p struct {
		MCPContext struct {
			ServerName string `json:"server_name"`
		} `json:"mcp_context"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return ""
	}
	return strings.TrimSpace(p.MCPContext.ServerName)
}

// flattenArgs 把任意形状的 tool_input 展平成"一个叶子值一条记录"。
//
// 为什么要展平而不是取顶层字符串：一次调用的危险目标可能藏在数组里
// （`{"paths":["/srv/app","/srv/backup"]}`）或嵌套对象里。压成 JSON 字符串再匹配，
// 会让 `/srv/backup` 这种精确目标因为和别的字符粘在一起而漏判——
// 漏判的代价是一次不可恢复的删除。
func flattenArgs(raw json.RawMessage) []ArgValue {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	var out []ArgValue
	walkArgs("", v, &out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func walkArgs(prefix string, v any, out *[]ArgValue) {
	switch t := v.(type) {
	case nil:
		return
	case string:
		// 有些 agent 会把嵌套参数**字符串化**再塞进来（Cline 的 VS Code 扩展就是这样：
		// parameters 里每个值都是 JSON 文本）。不展开的话，
		// `["/srv/backup"]` 会整体当一个字符串去匹配路径，永远匹配不上——
		// 而那次删除恰恰是最需要拦住的。
		//
		// 只在解析出**对象或数组**时才展开：一个普通字符串 "42" 也能被 json 解析，
		// 展开它没有意义还会改变键名。
		if len(t) > 1 && (t[0] == '{' || t[0] == '[') {
			var nested any
			if err := json.Unmarshal([]byte(t), &nested); err == nil {
				switch nested.(type) {
				case map[string]any, []any:
					walkArgs(prefix, nested, out)
					return
				}
			}
		}
		*out = append(*out, ArgValue{Key: prefix, Value: t})
	case bool:
		*out = append(*out, ArgValue{Key: prefix, Value: fmt.Sprintf("%t", t)})
	case float64:
		*out = append(*out, ArgValue{Key: prefix, Value: trimFloat(t)})
	case []any:
		for i, item := range t {
			walkArgs(fmt.Sprintf("%s[%d]", prefix, i), item, out)
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := k
			if prefix != "" {
				child = prefix + "." + k
			}
			walkArgs(child, t[k], out)
		}
	}
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	return s
}

// topLevelArgs 把展平值折回顶层键 → 一个字符串。
//
// 策略里的参数约束（CheckArgs）按顶层参数名判定（"这个工具要传 path"），
// 所以这里要把嵌套值重新聚到它的一级键上，并且多个值用换行连接——
// 换行不会出现在路径里，所以拼起来不会造出一个假的匹配。
func topLevelArgs(values []ArgValue) map[string]string {
	if len(values) == 0 {
		return nil
	}
	byKey := map[string][]string{}
	for _, v := range values {
		top := v.Key
		if i := strings.IndexAny(top, ".["); i >= 0 {
			top = top[:i]
		}
		if top == "" {
			continue
		}
		byKey[top] = append(byKey[top], v.Value)
	}
	out := make(map[string]string, len(byKey))
	for k, vs := range byKey {
		out[k] = strings.Join(vs, "\n")
	}
	return out
}
