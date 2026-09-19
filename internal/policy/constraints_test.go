package policy

import (
	"testing"

	"github.com/suhui-organization/ratchet/internal/model"
)

func policyWithPaths(allow, deny []string) model.Policy {
	return model.Policy{
		DefaultDecision: model.Deny,
		Servers: map[string]model.ServerRules{
			"filesystem": {
				Allow: []string{"read_file"},
				Constraints: map[string]model.ToolConstraints{
					"read_file": {Paths: &model.PathRules{Allow: allow, Deny: deny}},
				},
			},
		},
	}
}

func TestDenyGlobCoversSensitivePaths(t *testing.T) {
	p := policyWithPaths(nil, SensitivePathDeny)
	for _, path := range []string{
		"/workspace/.env", "/home/me/project/.env.local",
		"/home/me/.ssh/id_rsa", "/Users/x/.aws/credentials",
		"/repo/.git-credentials", "/app/.kube/config",
	} {
		if v := CheckArgs(p, "filesystem", "read_file", map[string]string{"path": path}); len(v) == 0 {
			t.Errorf("%s 应该被拒", path)
		}
	}
	// 正常路径不该被误伤
	for _, path := range []string{"/workspace/src/main.go", "/tmp/data.json", "docs/readme.md"} {
		if v := CheckArgs(p, "filesystem", "read_file", map[string]string{"path": path}); len(v) != 0 {
			t.Errorf("%s 不该被拒：%+v", path, v)
		}
	}
}

func TestAllowListRestrictsScope(t *testing.T) {
	p := policyWithPaths([]string{"/workspace/**"}, nil)
	if v := CheckArgs(p, "filesystem", "read_file", map[string]string{"path": "/workspace/a/b.txt"}); len(v) != 0 {
		t.Errorf("白名单内的路径不该被拒：%+v", v)
	}
	v := CheckArgs(p, "filesystem", "read_file", map[string]string{"path": "/etc/passwd"})
	if len(v) == 0 {
		t.Fatal("白名单外的路径应该被拒")
	}
	if v[0].Reason == "" || v[0].Rule == "" {
		t.Fatalf("违规要说清是哪条规则拒的：%+v", v[0])
	}
}

func TestNonPathValuesAreNotJudged(t *testing.T) {
	// 把普通字符串拿去匹配路径白名单，会把正常调用判成违规——
	// 判断不出来就不判，这是刻意的。
	p := policyWithPaths([]string{"/workspace/**"}, nil)
	if v := CheckArgs(p, "filesystem", "read_file", map[string]string{"encoding": "utf-8"}); len(v) != 0 {
		t.Errorf("非路径值不该参与路径判定：%+v", v)
	}
}

func TestToolWithoutConstraintsIsUntouched(t *testing.T) {
	p := policyWithPaths(nil, SensitivePathDeny)
	// 策略里没有约束的工具 → 不产生违规（交给三态判定）
	if v := CheckArgs(p, "filesystem", "write_file", map[string]string{"path": "/x/.env"}); len(v) != 0 {
		t.Errorf("无约束的工具不该产生违规：%+v", v)
	}
	// 策略里没有的 server 也一样
	if v := CheckArgs(p, "other", "read_file", map[string]string{"path": "/x/.env"}); len(v) != 0 {
		t.Errorf("未知 server 不该产生违规：%+v", v)
	}
}

func TestCheckArgsIsDeterministic(t *testing.T) {
	p := policyWithPaths(nil, SensitivePathDeny)
	args := map[string]string{"b_path": "/x/.env", "a_path": "/y/.ssh/id_rsa"}
	first := CheckArgs(p, "filesystem", "read_file", args)
	for i := 0; i < 5; i++ {
		again := CheckArgs(p, "filesystem", "read_file", args)
		if len(again) != len(first) || again[0].ArgKey != first[0].ArgKey {
			t.Fatalf("多次判定结果不一致：%+v vs %+v", first, again)
		}
	}
}

func TestLooksLikePathArg(t *testing.T) {
	for _, k := range []string{"path", "file_path", "filepath", "dir", "cwd", "target_path"} {
		if !LooksLikePathArg(k) {
			t.Errorf("%s 应被认成路径参数", k)
		}
	}
	for _, k := range []string{"encoding", "content", "timeout", "limit"} {
		if LooksLikePathArg(k) {
			t.Errorf("%s 不该被认成路径参数", k)
		}
	}
}

func TestDraftAttachesPathDenyOnlyToPathTools(t *testing.T) {
	inv := model.Inventory{Tools: []model.ToolObservation{
		{Server: "filesystem", Tool: "read_file", Calls: 5, ArgKeys: []string{"path"}},
		{Server: "filesystem", Tool: "list_directory", Calls: 3, ArgKeys: []string{"encoding"}},
	}}
	p := Draft(inv, Options{Now: fixedNow()})
	if _, ok := p.Servers["filesystem"].Constraints["read_file"]; !ok {
		t.Fatal("带路径参数的工具应该挂上路径黑名单")
	}
	if _, ok := p.Servers["filesystem"].Constraints["list_directory"]; ok {
		t.Fatal("参数不像路径的工具不该被挂约束——猜错会让正常调用被拒")
	}
	// 依据里要说明挂了约束
	if r := p.Rationale["filesystem/read_file"]; !containsStr(r, "path deny-list") {
		t.Fatalf("依据里没提约束：%q", r)
	}
}

func containsStr(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

func TestVerdictLooksUpTheThreeLists(t *testing.T) {
	p := model.Policy{DefaultDecision: model.Deny, Servers: map[string]model.ServerRules{
		"fs": {Allow: []string{"read_file"}, Approve: []string{"write_file"}, Deny: []string{"delete_file"}},
	}}
	cases := map[string]model.Decision{
		"read_file": model.Allow, "write_file": model.Approve, "delete_file": model.Deny,
		"unknown_tool": model.Deny, // 未登记 → 默认决策
	}
	for tool, want := range cases {
		if got := Verdict(p, "fs", tool); got != want {
			t.Errorf("%s → %s，期望 %s", tool, got, want)
		}
	}
	if got := Verdict(p, "no_such_server", "read_file"); got != model.Deny {
		t.Errorf("未知 server 应落到默认决策，实际 %s", got)
	}
}

func TestDefaultDecisionNeverMeansAllow(t *testing.T) {
	// 策略文件缺了 defaultDecision 时也必须拒绝——写坏了不等于放行
	p := model.Policy{Servers: map[string]model.ServerRules{}}
	if got := Verdict(p, "fs", "read_file"); got != model.Deny {
		t.Fatalf("缺省决策应为 deny，实际 %s", got)
	}
}

func TestEvaluateConstraintsBeatAllow(t *testing.T) {
	// 顺序不可颠倒：read_file 是 allow，但读 .env 必须拒——
	// 命中黑名单的调用不能因为"工具本身放行"就放行
	p := policyWithPaths(nil, SensitivePathDeny)
	dec, v := Evaluate(p, "filesystem", "read_file", map[string]string{"path": "/ws/.env"})
	if dec != model.Deny || len(v) == 0 {
		t.Fatalf("命中黑名单应判 deny，实际 %s %+v", dec, v)
	}
	dec, v = Evaluate(p, "filesystem", "read_file", map[string]string{"path": "/ws/main.go"})
	if dec != model.Allow || v != nil {
		t.Fatalf("正常路径应放行，实际 %s %+v", dec, v)
	}
}
