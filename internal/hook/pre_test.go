package hook

import "testing"

// 基本形状：Claude Code 的 PreToolUse 报文要能拆出 server / tool / 参数值。
func TestTranslatePreFromClaude(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","tool_name":"mcp__filesystem__delete_file",
		"tool_input":{"path":"/srv/backup/a.sql"},"transcript_path":"/x"}`)
	call, ok := TranslatePre(raw, HarnessAuto, "", "")
	if !ok {
		t.Fatal("应当能翻译")
	}
	if call.Server != "filesystem" || call.Tool != "delete_file" {
		t.Fatalf("MCP 工具名应当拆成真实来源，得到 %s/%s", call.Server, call.Tool)
	}
	if len(call.Values) != 1 || call.Values[0].Value != "/srv/backup/a.sql" {
		t.Fatalf("执行点必须拿到参数**值**（观测侧才不拿），得到 %+v", call.Values)
	}
}

// 嵌套与数组要展开成一条条独立的值。
//
// 这一条是"防误删"的关键：把 ["/srv/app","/srv/backup"] 压成一个 JSON 字符串再匹配，
// 会让 /srv/backup 因为和别的字符粘在一起而漏判——漏判的代价是一次不可恢复的删除。
func TestNestedAndArrayArgsAreFlattened(t *testing.T) {
	raw := []byte(`{"server":"filesystem","tool":"delete_many",
		"args":{"paths":["/srv/app","/srv/backup"],"opts":{"recursive":true}}}`)
	call, ok := TranslatePre(raw, HarnessGeneric, "", "")
	if !ok {
		t.Fatal("应当能翻译")
	}
	want := map[string]string{
		"paths[0]":       "/srv/app",
		"paths[1]":       "/srv/backup",
		"opts.recursive": "true",
	}
	got := map[string]string{}
	for _, v := range call.Values {
		got[v.Key] = v.Value
	}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("展平后 %s 应当是 %q，得到 %q（全部：%+v）", k, w, got[k], call.Values)
		}
	}
	// 顶层键要能把嵌套值折回来，供策略的参数约束判定使用
	if call.Args["paths"] == "" {
		t.Fatalf("顶层参数 paths 应当折叠出字符串，得到 %+v", call.Args)
	}
}

// 认不出来的形状要老实返回 false，不能猜一条记录出来。
func TestUnknownShapeIsNotGuessed(t *testing.T) {
	if _, ok := TranslatePre([]byte(`{"nothing":"useful"}`), HarnessGeneric, "", ""); ok {
		t.Fatal("没有 server/tool 的形状不该被猜成一个调用")
	}
	if _, ok := TranslatePre([]byte(`not json`), HarnessAuto, "", ""); ok {
		t.Fatal("非法 JSON 不该被猜成一个调用")
	}
}

// 显式给 --server 时要覆盖解析结果，这样才能把多个 agent 归到同一策略下。
func TestExplicitServerOverridesMCPName(t *testing.T) {
	raw := []byte(`{"tool_name":"mcp__github__create_issue","tool_input":{},"transcript_path":"/x"}`)
	call, ok := TranslatePre(raw, HarnessClaude, "filesystem", "")
	if !ok {
		t.Fatal("应当能翻译")
	}
	if call.Server != "filesystem" {
		t.Fatalf("显式 server 应当覆盖，得到 %s", call.Server)
	}
}
