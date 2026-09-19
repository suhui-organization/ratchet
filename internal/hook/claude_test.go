package hook

import (
	"strings"
	"testing"
)

func TestFromClaudeBuiltinTool(t *testing.T) {
	raw := []byte(`{
	  "session_id": "s1", "transcript_path": "/x/y.jsonl", "cwd": "/tmp",
	  "permission_mode": "default", "hook_event_name": "PostToolUse",
	  "tool_name": "Write", "tool_input": {"file_path": "/tmp/a", "content": "ghp_SECRET"},
	  "tool_response": {"filePath": "/tmp/a"}
	}`)
	call, ok := FromClaude(raw, "", "")
	if !ok {
		t.Fatal("内置工具应被记录")
	}
	if call.Server != "claude-code-tools" || call.Tool != "Write" || call.Agent != "claude-code" {
		t.Fatalf("翻译结果 = %+v", call)
	}
	if call.Decision != "allow" || call.Outcome != "ok" {
		t.Fatalf("状态 = %s/%s", call.Decision, call.Outcome)
	}
	joined := call.Server + call.Tool + call.Agent + call.Decision + call.Outcome
	if strings.Contains(joined, "ghp_SECRET") {
		t.Fatal("工具参数被记录下来了")
	}
}

func TestFromClaudeMCPToolIsSplit(t *testing.T) {
	// 这是接入 Claude Code 最值钱的一点：MCP 工具名自带真实来源
	raw := []byte(`{"hook_event_name":"PostToolUse","tool_name":"mcp__filesystem__read_file","tool_response":{}}`)
	call, ok := FromClaude(raw, "", "")
	if !ok {
		t.Fatal("MCP 工具应被记录")
	}
	if call.Server != "filesystem" || call.Tool != "read_file" {
		t.Fatalf("没有拆出真实来源：%+v", call)
	}
}

func TestFromClaudePermissionDeniedIsRecorded(t *testing.T) {
	// deny 记录的是"agent 想做但被拦下的事"——只看放行记录会低估真实意图
	raw := []byte(`{"hook_event_name":"PermissionDenied","tool_name":"Bash"}`)
	call, ok := FromClaude(raw, "", "")
	if !ok {
		t.Fatal("被拒绝的调用也要记")
	}
	if call.Decision != "deny" || call.Outcome != "blocked" {
		t.Fatalf("状态 = %s/%s", call.Decision, call.Outcome)
	}
}

func TestFromClaudeErrorOutcome(t *testing.T) {
	failed := []string{
		`{"tool_name":"Read","tool_response":{"is_error":true}}`,
		`{"tool_name":"Read","tool_response":{"isError":true}}`,
		`{"tool_name":"Read","tool_response":{"error":"boom"}}`,
		`{"tool_name":"Read","tool_response":{"success":false}}`,
	}
	for _, raw := range failed {
		call, ok := FromClaude([]byte(raw), "", "")
		if !ok || call.Outcome != "error" {
			t.Errorf("没识别出失败：%s → %+v", raw, call)
		}
	}
}

func TestFromClaudeDoesNotGuessOutcome(t *testing.T) {
	// 没有显式错误标记时不能瞎猜——猜错会把失败记成成功，
	// 而失败率是后面判断某个工具该不该收掉的重要信号
	call, _ := FromClaude([]byte(`{"tool_name":"Read","tool_response":{"text":"..."}}`), "", "")
	if call.Outcome != "ok" {
		t.Errorf("无错误标记时应为 ok，实际 %s", call.Outcome)
	}
}

func TestFromClaudeBadInput(t *testing.T) {
	for _, raw := range []string{`not json`, `{"tool_name":""}`, `{}`} {
		if _, ok := FromClaude([]byte(raw), "", ""); ok {
			t.Errorf("不该记录：%s", raw)
		}
	}
}

func TestFromClaudeServerOverride(t *testing.T) {
	call, _ := FromClaude([]byte(`{"tool_name":"mcp__filesystem__read_file"}`), "shared", "")
	if call.Server != "shared" {
		t.Fatalf("覆盖没生效：%+v", call)
	}
}
