package hook

import "testing"

func TestDetectPicksTheRightHarness(t *testing.T) {
	cases := map[string]Harness{
		`{"tool_name":"Read","transcript_path":"/x"}`:      HarnessClaude,
		`{"tool_name":"Read","tool_use_id":"toolu_1"}`:     HarnessClaude,
		`{"tool_name":"Read","permission_mode":"default"}`: HarnessClaude,
		// tool_response 是 Claude 的独有字段：只看 tool_name 会把它误判成 Codex
		`{"tool_name":"Write","tool_response":{"is_error":true}}`:   HarnessClaude,
		`{"hook_event_name":"PermissionDenied","tool_name":"Bash"}`: HarnessClaude,
		`{"tool_name":"shell","model":"gpt-5","session_id":"s"}`:    HarnessCodex,
		`{"server":"filesystem","tool":"read_file"}`:                HarnessGeneric,
	}
	for raw, want := range cases {
		if got := detect([]byte(raw)); got != want {
			t.Errorf("%s → %s，期望 %s", raw, got, want)
		}
	}
}

func TestTranslateAutoRoutesCorrectly(t *testing.T) {
	claude := []byte(`{"tool_name":"mcp__github__create_issue","transcript_path":"/x"}`)
	call, ok := Translate(claude, HarnessAuto, "", "")
	if !ok || call.Server != "github" || call.Tool != "create_issue" {
		t.Fatalf("auto 没路由到 Claude：%+v ok=%v", call, ok)
	}
	codex := []byte(`{"tool_name":"shell"}`)
	call, ok = Translate(codex, HarnessAuto, "", "")
	if !ok || call.Server != "codex-tools" || call.Agent != "codex" {
		t.Fatalf("auto 没路由到 Codex：%+v ok=%v", call, ok)
	}
}

func TestFromGeneric(t *testing.T) {
	call, ok := FromGeneric([]byte(`{"server":"shell","tool":"bash","decision":"deny","outcome":"blocked"}`), "", "")
	if !ok || call.Server != "shell" || call.Tool != "bash" || call.Decision != "deny" {
		t.Fatalf("generic 翻译不对：%+v ok=%v", call, ok)
	}
	if _, ok := FromGeneric([]byte(`{"tool":"bash"}`), "", ""); ok {
		t.Fatal("缺 server 时不该记录——没有来源就无法编译成策略")
	}
	call, ok = FromGeneric([]byte(`{"tool":"mcp__fs__read_file"}`), "", "")
	if !ok || call.Server != "fs" || call.Tool != "read_file" {
		t.Fatalf("generic 没拆 MCP 名：%+v", call)
	}
}

func TestSplitMCPTool(t *testing.T) {
	cases := map[string][2]string{
		"mcp__filesystem__read_file": {"filesystem", "read_file"},
		"mcp__github__create_issue":  {"github", "create_issue"},
		"mcp__a__b__c":               {"a", "b__c"},
	}
	for in, want := range cases {
		s, n, ok := splitMCPTool(in)
		if !ok || s != want[0] || n != want[1] {
			t.Errorf("%s → %q/%q ok=%v，期望 %v", in, s, n, ok, want)
		}
	}
	for _, bad := range []string{"Read", "mcp__only", "mcp____x", "mcp__s__"} {
		if _, _, ok := splitMCPTool(bad); ok {
			t.Errorf("%s 不该被当成 MCP 工具名", bad)
		}
	}
}
