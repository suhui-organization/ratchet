package hook

import "testing"

// 别名要认全：用户在 --harness 里会写各种叫法。
func TestParseHarnessNameAliases(t *testing.T) {
	cases := map[string]Harness{
		"claude-code": HarnessClaude,
		"claude":      HarnessClaude,
		"codex":       HarnessCodex,
		"codebuddy":   HarnessCodeBuddy,
		"tencent":     HarnessCodeBuddy,
		"qwen-code":   HarnessQwen,
		"qwen":        HarnessQwen,
		"qoder":       HarnessQoder,
		"lingma":      HarnessQoder, // 通义灵码
		"tongyi":      HarnessQoder,
		"gemini-cli":  HarnessGemini,
		"gemini":      HarnessGemini,
		// 反重力是独立一档：它的阻断机制和 Gemini CLI 是反的
		"antigravity": HarnessAntigravity,
		"agy":         HarnessAntigravity,
		"cursor":      HarnessCursor,
		"crush":       HarnessCrush,
		"droid":       HarnessDroid,
		"factory":     HarnessDroid,
		"cline":       HarnessCline,
		"opencode":    HarnessOpenCode,
		"generic":     HarnessGeneric,
		"":            HarnessAuto,
		"auto":        HarnessAuto,
		"???":         HarnessAuto,
	}
	for in, want := range cases {
		if got := ParseHarnessName(in); got != want {
			t.Fatalf("ParseHarnessName(%q) = %q，想要 %q", in, got, want)
		}
	}
}

// 环境变量是最可靠的判据——三家的载荷形状几乎一样，只靠形状会认错。
func TestDetectFromEnv(t *testing.T) {
	cases := []struct {
		env  []string
		want Harness
	}{
		{[]string{"CODEBUDDY_PROJECT_DIR=/x"}, HarnessCodeBuddy},
		{[]string{"CODEBUDDY_PLUGIN_ROOT=/x"}, HarnessCodeBuddy},
		{[]string{"QWEN_PROJECT_DIR=/x"}, HarnessQwen},
		{[]string{"QODER_PROJECT_DIR=/x"}, HarnessQoder},
		{[]string{"QODER_PLUGIN_ROOT=/x"}, HarnessQoder},
		{[]string{"ANTIGRAVITY_CONVERSATION_ID=abc"}, HarnessAntigravity},
		{[]string{"CRUSH=1"}, HarnessCrush},
		{[]string{"FACTORY_PROJECT_DIR=/x"}, HarnessDroid},
		{[]string{"CLAUDE_PROJECT_DIR=/x"}, HarnessClaude},
		{[]string{"PATH=/usr/bin"}, HarnessAuto},
		// Codex 的插件钩子同时设 PLUGIN_ROOT 与 CLAUDE_PLUGIN_ROOT，
		// 要按 Codex 认——它俩的输出协议不同。
		{[]string{"PLUGIN_ROOT=/x", "CLAUDE_PLUGIN_ROOT=/x"}, HarnessCodex},
	}
	for _, c := range cases {
		if got := DetectFromEnv(c.env); got != c.want {
			t.Fatalf("DetectFromEnv(%v) = %q，想要 %q", c.env, got, c.want)
		}
	}
}

// 形状兜底：认得出各家独有的字段。
func TestDetectByShape(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want Harness
	}{
		{"Gemini 用 BeforeTool", `{"hook_event_name":"BeforeTool","tool_name":"run_shell_command"}`, HarnessGemini},
		{"反重力的 toolCall 信封", `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls"}},"conversationId":"c1"}`, HarnessAntigravity},
		{"Codex 有 turn_id", `{"turn_id":"t1","tool_name":"Bash"}`, HarnessCodex},
		{"CodeBuddy 有 generation_id", `{"generation_id":"g1","tool_name":"Write"}`, HarnessCodeBuddy},
		{"CodeBuddy 的会话路径", `{"transcript_path":"/u/.codebuddy/projects/a.jsonl","tool_name":"Write"}`, HarnessCodeBuddy},
		{"Qwen 的会话路径", `{"transcript_path":"/u/.qwen/projects/a.jsonl","tool_name":"write_file"}`, HarnessQwen},
		{"Claude 的 tool_use_id", `{"tool_use_id":"toolu_1","tool_name":"Read"}`, HarnessClaude},
		{"Claude 的 PermissionDenied", `{"hook_event_name":"PermissionDenied","tool_name":"Bash"}`, HarnessClaude},
		{"认不出来的当 generic", `{"hello":"world"}`, HarnessGeneric},
	}
	for _, c := range cases {
		if got := Detect([]byte(c.raw)); got != c.want {
			t.Fatalf("%s：Detect = %q，想要 %q", c.name, got, c.want)
		}
	}
}

// 协议能力表本身是决策依据，写错了会静默 fail-open，所以钉住它。
func TestProtocolCapabilities(t *testing.T) {
	if ProtocolOf(HarnessCodex).SupportsAsk {
		t.Fatal("Codex 不支持 ask——带 ask 的输出会被判成钩子失败然后继续执行")
	}
	if ProtocolOf(HarnessGemini).SupportsAsk {
		t.Fatal("Gemini 的 BeforeTool 没有 ask 这一档")
	}
	if ProtocolOf(HarnessAntigravity).SupportsAsk {
		t.Fatal("反重力的钩子没有 ask 这一档")
	}
	// Cursor 的 preToolUse 里 ask 不被强制执行；Crush 的 decision 只有 allow/deny/沉默；
	// Cline 只有 cancel 布尔；OpenCode 的插件只能抛错。
	for _, h := range []Harness{HarnessCursor, HarnessCrush, HarnessCline, HarnessOpenCode} {
		if ProtocolOf(h).SupportsAsk {
			t.Fatalf("%s 没有可靠的人工确认档，协议表写错了会让 ask 变成静默 fail-open", h)
		}
	}
	if !ProtocolOf(HarnessDroid).SupportsAsk {
		t.Fatal("Factory Droid 的输出协议与 Claude 一致，是支持 ask 的")
	}
	for _, h := range []Harness{HarnessClaude, HarnessCodeBuddy, HarnessQwen, HarnessQoder} {
		if !ProtocolOf(h).SupportsAsk {
			t.Fatalf("%s 支持 ask，协议表里写错了会白白丢掉人工确认这一档", h)
		}
	}
	if ProtocolOf(HarnessQoder).DenyField != DenyTopLevelDecision {
		t.Fatal("Qoder 的拒绝写在顶层 decision 上")
	}
	if ProtocolOf(HarnessGemini).DenyField != DenyDecisionBlock {
		t.Fatal("Gemini 的拒绝形状是顶层 decision:block")
	}
	if ProtocolOf(HarnessAntigravity).DenyField != DenyDecisionBlock {
		t.Fatal("反重力的拒绝形状也是顶层 decision:block")
	}
	// 这几家都同构：hookSpecificOutput.permissionDecision
	for _, h := range []Harness{HarnessClaude, HarnessCodeBuddy, HarnessQwen, HarnessCodex} {
		if ProtocolOf(h).DenyField != DenyPermissionDecision {
			t.Fatalf("%s 应当用 hookSpecificOutput.permissionDecision", h)
		}
	}
}

// 反重力的载荷是嵌套信封，而且参数键是 CommandLine（大写）——
// 拦截判定绝不能依赖参数名，只能依赖值。
func TestTranslatePreFromAntigravity(t *testing.T) {
	raw := []byte(`{"toolCall":{"name":"run_command","args":{"CommandLine":"rm -rf /srv/backup"}},
		"conversationId":"c1","stepIdx":3}`)
	call, ok := TranslatePre(raw, HarnessAuto, "", "")
	if !ok {
		t.Fatal("应当能翻译反重力的信封")
	}
	if call.Tool != "run_command" {
		t.Fatalf("工具名应当取自 toolCall.name，得到 %q", call.Tool)
	}
	if call.Server != "antigravity-tools" {
		t.Fatalf("内置工具应当归到 antigravity-tools，得到 %q", call.Server)
	}
	if len(call.Values) != 1 || call.Values[0].Value != "rm -rf /srv/backup" {
		t.Fatalf("必须拿到参数**值**才能判危险目标，得到 %+v", call.Values)
	}
	if call.Values[0].Key != "CommandLine" {
		t.Fatalf("参数键原样保留（不假设叫 command），得到 %q", call.Values[0].Key)
	}
}

// Gemini 系的 MCP 工具名是 mcp_<server>_<tool>（单下划线），拆不开；
// 只有 mcp_context.server_name 能给出真实来源。
func TestGeminiMCPServerComesFromMCPContext(t *testing.T) {
	raw := []byte(`{"hook_event_name":"BeforeTool","tool_name":"mcp_filesystem_read_file",
		"tool_input":{"path":"/x"},"mcp_context":{"server_name":"filesystem"}}`)
	call, ok := TranslatePre(raw, HarnessGemini, "", "")
	if !ok {
		t.Fatal("应当能翻译")
	}
	if call.Server != "filesystem" {
		t.Fatalf("应当从 mcp_context 取 server，得到 %q", call.Server)
	}
	// 没有 mcp_context 时保留整名，不能瞎拆
	raw2 := []byte(`{"hook_event_name":"BeforeTool","tool_name":"mcp_filesystem_read_file","tool_input":{}}`)
	call2, ok := TranslatePre(raw2, HarnessGemini, "", "")
	if !ok {
		t.Fatal("应当能翻译")
	}
	if call2.Tool != "mcp_filesystem_read_file" {
		t.Fatalf("没有 mcp_context 时应当保留整名（单下划线拆不开），得到 %q", call2.Tool)
	}
}

// Cursor 的 beforeShellExecution 把命令放在**顶层** command 上，不在 tool_input 里。
// 只认 tool_input 的解析器在这里会拿到空参数——拦截形同虚设。
func TestCursorBeforeShellExecutionReadsTopLevelCommand(t *testing.T) {
	raw := []byte(`{"hook_event_name":"beforeShellExecution","command":"rm -rf /srv/backup",
		"cwd":"/srv","sandbox":false}`)
	call, ok := TranslatePre(raw, HarnessCursor, "", "")
	if !ok {
		t.Fatal("应当能翻译 beforeShellExecution")
	}
	if len(call.Values) != 1 || call.Values[0].Value != "rm -rf /srv/backup" {
		t.Fatalf("顶层 command 必须被读到，否则拦不住：%+v", call.Values)
	}
	if call.Tool != "Shell" {
		t.Fatalf("beforeShellExecution 应当归成 Shell 工具，得到 %q", call.Tool)
	}
}

// Cline 有两套载荷。VS Code 扩展那套会把参数**字符串化**——
// 不展开的话 ["/srv/backup"] 会整体当一个字符串去匹配路径，永远匹配不上。
func TestClineBothPayloadShapes(t *testing.T) {
	// CLI / SDK 形状
	cli := []byte(`{"tool_call":{"name":"run_commands","input":{"commands":["rm -rf /srv/backup"]}}}`)
	call, ok := TranslatePre(cli, HarnessCline, "", "")
	if !ok {
		t.Fatal("CLI 形状应当能翻译")
	}
	if call.Tool != "run_commands" {
		t.Fatalf("工具名应当取自 tool_call.name，得到 %q", call.Tool)
	}
	if !hasValue(call.Values, "rm -rf /srv/backup") {
		t.Fatalf("必须拿到参数值，得到 %+v", call.Values)
	}

	// VS Code 扩展形状：parameters 里的值是 JSON 文本
	vsc := []byte(`{"preToolUse":{"toolName":"read_files","parameters":"{\"files\":\"[\\\"/srv/backup\\\"]\"}"},"workspaceRoots":["/srv"]}`)
	call2, ok := TranslatePre(vsc, HarnessCline, "", "")
	if !ok {
		t.Fatal("VS Code 形状应当能翻译")
	}
	if call2.Tool != "read_files" {
		t.Fatalf("工具名应当取自 preToolUse.toolName，得到 %q", call2.Tool)
	}
	if !hasValue(call2.Values, "/srv/backup") {
		t.Fatalf("字符串化的嵌套值必须被展开，否则路径匹配不上：%+v", call2.Values)
	}
}

func hasValue(vs []ArgValue, want string) bool {
	for _, v := range vs {
		if v.Value == want {
			return true
		}
	}
	return false
}

// Crush / Droid 的工具字段和其它家同形，但事件键不同（Crush 用 event）。
func TestCrushAndDroidPayloads(t *testing.T) {
	crush := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/x","tool_name":"bash","tool_input":{"command":"rm -rf /srv/backup"}}`)
	call, ok := TranslatePre(crush, HarnessAuto, "", "")
	if !ok {
		t.Fatal("Crush 载荷应当能翻译")
	}
	if call.Tool != "bash" || call.Server != "crush-tools" {
		t.Fatalf("Crush 解析错了：%s/%s", call.Server, call.Tool)
	}
	if !hasValue(call.Values, "rm -rf /srv/backup") {
		t.Fatalf("Crush 必须拿到参数值：%+v", call.Values)
	}

	// Droid 的会话记录落在 .factory/ 下，这是它与 Claude 的唯一形状差异
	droid := []byte(`{"session_id":"s","transcript_path":"/home/u/.factory/projects/x/session.jsonl",
		"hook_event_name":"PreToolUse","tool_name":"Execute","tool_input":{"command":"rm -rf /srv/backup"}}`)
	call2, ok := TranslatePre(droid, HarnessAuto, "", "")
	if !ok {
		t.Fatal("Droid 载荷应当能翻译")
	}
	if call2.Server != "factory-tools" {
		t.Fatalf("Droid 内置工具应当归到 factory-tools，得到 %q", call2.Server)
	}
}

// 五家的 PreToolUse / BeforeTool 输入同形，解析器要都能吃下。
func TestTranslatePreAcrossHarnesses(t *testing.T) {
	payload := `{"hook_event_name":"PreToolUse","tool_name":"mcp__filesystem__delete_file",
		"tool_input":{"path":"/srv/backup/a.sql"},"permission_mode":"default"}`
	for _, h := range []Harness{
		HarnessClaude, HarnessCodeBuddy, HarnessQwen, HarnessQoder, HarnessAuto,
	} {
		call, ok := TranslatePre([]byte(payload), h, "", "")
		if !ok {
			t.Fatalf("%s 解析失败", h)
		}
		if call.Server != "filesystem" || call.Tool != "delete_file" {
			t.Fatalf("%s：MCP 工具名没拆对，得到 %s/%s", h, call.Server, call.Tool)
		}
		if len(call.Values) != 1 || call.Values[0].Value != "/srv/backup/a.sql" {
			t.Fatalf("%s：没拿到参数值，得到 %+v", h, call.Values)
		}
	}
}

// 国内 agent 的内置工具名和国外不一样（run_shell_command vs Bash），
// 但归属的 server 名要各自稳定，否则策略里认不出来。
func TestBuiltinToolNamesLandInPerAgentServers(t *testing.T) {
	cases := map[Harness]struct{ payload, server string }{
		HarnessQwen:      {`{"tool_name":"run_shell_command","tool_input":{"command":"ls"}}`, "qwen-tools"},
		HarnessCodeBuddy: {`{"tool_name":"Bash","tool_input":{"command":"ls"}}`, "codebuddy-tools"},
		HarnessQoder:     {`{"tool_name":"Bash","tool_input":{"command":"ls"}}`, "qoder-tools"},
		HarnessGemini:    {`{"tool_name":"run_shell_command","tool_input":{"command":"ls"}}`, "gemini-tools"},
	}
	for h, c := range cases {
		call, ok := TranslatePre([]byte(c.payload), h, "", "")
		if !ok {
			t.Fatalf("%s 解析失败", h)
		}
		if call.Server != c.server {
			t.Fatalf("%s 的内置工具应当归到 %s，得到 %s", h, c.server, call.Server)
		}
	}
}
