package hook

import (
	"strings"
	"testing"
)

func TestFromCodexMapsBasicEvent(t *testing.T) {
	raw := []byte(`{
	  "hook_event_name": "PostToolUse",
	  "tool_name": "shell",
	  "tool_input": {"command": "cat ~/.ssh/id_rsa"},
	  "session_id": "sess-1",
	  "cwd": "/tmp",
	  "model": "gpt"
	}`)
	call, ok := FromCodex(raw, "", "")
	if !ok {
		t.Fatal("正常事件应被记录")
	}
	if call.Server != "codex-tools" || call.Tool != "shell" || call.Agent != "codex" {
		t.Fatalf("翻译结果 = %+v", call)
	}
}

func TestFromCodexNeverKeepsToolInput(t *testing.T) {
	// 参数里可能是私钥、令牌、文件内容。最小权限编译不需要它，
	// 所以它连结构体都不该进——这里用一个明显的诱饵做断言。
	raw := []byte(`{"tool_name":"shell","tool_input":{"command":"ghp_secretShouldNotBeStored"}}`)
	call, ok := FromCodex(raw, "", "")
	if !ok {
		t.Fatal("事件应被记录")
	}
	encoded := strings.Join([]string{call.TS, call.Agent, call.Server, call.Tool, call.Decision, call.Outcome}, "|")
	if strings.Contains(encoded, "ghp_secretShouldNotBeStored") {
		t.Fatal("参数被记录下来了")
	}
}

func TestFromCodexSkipsGatewayToolsAndBadInput(t *testing.T) {
	for _, raw := range []string{
		`{"tool_name":"mcp__filesystem__read_file"}`,
		`{"tool_name":"ratchet__read_file"}`,
		`{"tool_name":""}`,
		`{"hook_event_name":"PostToolUse"}`,
		`not json at all`,
	} {
		if _, ok := FromCodex([]byte(raw), "", ""); ok {
			t.Errorf("不该记录：%s", raw)
		}
	}
}

func TestFromCodexAllowsOverrides(t *testing.T) {
	call, ok := FromCodex([]byte(`{"tool_name":"shell"}`), "custom-server", "my-agent")
	if !ok || call.Server != "custom-server" || call.Agent != "my-agent" {
		t.Fatalf("覆盖没生效：%+v ok=%v", call, ok)
	}
}
