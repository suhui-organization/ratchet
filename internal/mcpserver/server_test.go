package mcpserver

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// roundTrip 把几条请求喂进去，取回响应。stdout 里除了 JSON-RPC 不该有别的东西。

// TestInitializeReportsInjectedVersion 盯的是一次真实事故：initialize 里的
// serverInfo.version 曾经硬编码成 "0.9.0"，而二进制已经是 0.12.0——
// 对外报的版本和实际不符，客户端与目录都会引用它。
// 现在版本由 main 注入，这个测试保证"注入什么就报什么"，且不再有写死的字面量。
func TestInitializeReportsInjectedVersion(t *testing.T) {
	old := ServerVersion
	defer func() { ServerVersion = old }()
	ServerVersion = "9.9.9-test"

	out := roundTrip(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(out) != 1 {
		t.Fatalf("期望 1 条响应，得到 %d", len(out))
	}
	result, _ := out[0]["result"].(map[string]any)
	info, _ := result["serverInfo"].(map[string]any)
	if got := info["version"]; got != "9.9.9-test" {
		t.Fatalf("serverInfo.version = %v，期望注入进去的 9.9.9-test", got)
	}
}
func roundTrip(t *testing.T, reqs ...string) []map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	in := strings.NewReader(strings.Join(reqs, "\n") + "\n")
	if err := Serve(in, &stdout, &stderr); err != nil {
		t.Fatalf("Serve 失败：%v（stderr=%s）", err, stderr.String())
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("stdout 里有一行不是 JSON-RPC：%q", line)
		}
		out = append(out, m)
	}
	return out
}

func TestHandshakeAndToolList(t *testing.T) {
	resps := roundTrip(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	if len(resps) != 2 {
		t.Fatalf("通知不该有响应，实际收到 %d 条", len(resps))
	}
	init := resps[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2024-11-05" {
		t.Fatalf("协议版本不对：%v", init["protocolVersion"])
	}
	list := resps[1]["result"].(map[string]any)["tools"].([]any)
	if len(list) != 3 {
		t.Fatalf("应暴露 3 个工具，实际 %d", len(list))
	}
	names := map[string]bool{}
	for _, raw := range list {
		tool := raw.(map[string]any)
		names[tool["name"].(string)] = true
		for _, field := range []string{"name", "description", "inputSchema"} {
			if _, ok := tool[field]; !ok {
				t.Errorf("工具缺 %s：%v", field, tool)
			}
		}
	}
	for _, want := range []string{"ratchet_scan", "ratchet_policy", "ratchet_check"} {
		if !names[want] {
			t.Errorf("缺工具 %s", want)
		}
	}
}

func TestNoiseOnStdinDoesNotBreakTheConnection(t *testing.T) {
	// 客户端可能会往 stdin 打字、日志可能混进来；一行噪音不该断掉整个会话
	resps := roundTrip(t,
		`this is not json`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
	)
	if len(resps) != 1 || resps[0]["result"] == nil {
		t.Fatalf("噪音之后应仍能正常响应：%+v", resps)
	}
}

func TestUnknownMethodReturnsRPCError(t *testing.T) {
	resps := roundTrip(t, `{"jsonrpc":"2.0","id":9,"method":"does/not/exist"}`)
	if resps[0]["error"] == nil {
		t.Fatalf("未知方法应返回 error：%+v", resps[0])
	}
}

func TestToolErrorIsNotAConnectionError(t *testing.T) {
	// 工具级失败按约定走 isError，而不是 JSON-RPC error——
	// 后者会让客户端以为连接坏了
	resps := roundTrip(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"ratchet_policy","arguments":{"policy":"/nope/x.json"}}}`)
	res := resps[0]["result"].(map[string]any)
	if res["isError"] != true {
		t.Fatalf("读不到策略应返回 isError：%+v", res)
	}
	if resps[0]["error"] != nil {
		t.Fatalf("不该是 JSON-RPC error：%+v", resps[0])
	}
}

func TestRatchetCheckUsesTheSameEvaluator(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.json")
	body := `{"version":"1","defaultDecision":"deny","servers":{"filesystem":{
	  "allow":["read_file"],"approve":[],"deny":[],
	  "constraints":{"read_file":{"paths":{"deny":["**/.ssh/**"]}}}}},
	  "rationale":{},"needsReview":[]}`
	if err := os.WriteFile(policyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	call := func(args string) string {
		req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"ratchet_check","arguments":{"policy":` +
			jsonString(policyPath) + `,"server":"filesystem","tool":"read_file","args":` + args + `}}}`
		res := roundTrip(t, req)[0]["result"].(map[string]any)
		return res["content"].([]any)[0].(map[string]any)["text"].(string)
	}
	if got := call(`{"path":"/ws/main.go"}`); !strings.Contains(got, "decision=allow") {
		t.Errorf("正常路径应放行，实际：%s", got)
	}
	if got := call(`{"path":"/home/me/.ssh/id_rsa"}`); !strings.Contains(got, "decision=deny") {
		t.Errorf("私钥路径应拒绝，实际：%s", got)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
