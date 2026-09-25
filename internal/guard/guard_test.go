package guard

import (
	"sort"
	"strings"
	"testing"

	"github.com/suhui-organization/ratchet/internal/hook"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/policy"
)

// fixture 是一份够真实的策略：只读工具放行、写入与执行要审批、删除直接拒。
func fixture() model.Policy {
	return model.Policy{
		Version:         "1",
		DefaultDecision: model.Deny,
		Servers: map[string]model.ServerRules{
			"codex-tools": {
				Allow:   []string{"read_file"},
				Approve: []string{"write_file", "shell"},
			},
			"filesystem": {Allow: []string{"read_file", "list_dir"}},
		},
		Rationale: map[string]string{},
		Guard:     policy.DefaultGuard(),
	}
}

func req(server, tool string, args map[string]string) Request {
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vals := make([]hook.ArgValue, 0, len(keys))
	for _, k := range keys {
		vals = append(vals, hook.ArgValue{Key: k, Value: args[k]})
	}
	return Request{Server: server, Tool: tool, Args: args, Values: vals}
}

// —— 这一组是这次改动要证明的事：错误动作真的会被拦下 ——

// 最核心的一条：agent 去删备份 —— PocketOS 那类事故的形态。
func TestDeletingABackupIsRefused(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "delete_file",
		map[string]string{"path": "/srv/app/backup/prod.sql"}), Options{})
	if v.Decision != Deny {
		t.Fatalf("删备份必须被拒，得到 %s（规则 %s）", v.Decision, v.Rule)
	}
	if v.Rule != RuleProtectedTarget {
		t.Fatalf("应当由受保护目标这条规则拒下，得到 %s", v.Rule)
	}
	if len(v.Matches) == 0 {
		t.Fatal("拒绝必须说清命中了哪个目标，否则事后无法复盘")
	}
}

// 目标优先于工具：一个被标成 allow 的工具照样不能把受保护的东西写掉。
func TestProtectedTargetBeatsAnAllowedTool(t *testing.T) {
	p := fixture()
	// 刻意把 write_file 放进 allow，模拟"这份策略把它当无害工具"
	p.Servers["codex-tools"] = model.ServerRules{
		Allow: []string{"write_file", "read_file"},
	}
	v := Decide(p, req("codex-tools", "write_file",
		map[string]string{"path": "/srv/backup/dump.sql"}), Options{})
	if v.Decision != Deny {
		t.Fatalf("被标成 allow 的工具写受保护目标也必须拒，得到 %s", v.Decision)
	}
}

// 破坏性藏在命令字符串里：工具名 shell 完全中性。
func TestDestructiveCommandHiddenInArgsIsCaught(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "shell",
		map[string]string{"command": "rm -rf /srv/app/backup"}), Options{})
	if v.Decision != Deny {
		t.Fatalf("藏在参数里的删除必须被拒，得到 %s（规则 %s）", v.Decision, v.Rule)
	}
}

// 凭据类：读也不行。私钥被读走和被删掉，代价一样。
func TestReadingCredentialsIsRefused(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "read_file",
		map[string]string{"path": "/srv/app/.env"}), Options{})
	if v.Decision != Deny {
		t.Fatalf("读 .env 必须被拒，得到 %s", v.Decision)
	}
	if v.Rule != RuleSensitiveTarget {
		t.Fatalf("应当命中凭据类规则，得到 %s", v.Rule)
	}
}

// 反过来的一条同样重要：读一份 schema.sql 是正常干活，不能拦。
// 这一条是"把清单拆成两类"的全部意义——不拆就会把 agent 废掉。
func TestReadingASqlFileIsNotRefused(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "read_file",
		map[string]string{"path": "/srv/app/db/schema.sql"}), Options{})
	if v.Decision == Deny {
		t.Fatalf("读 schema.sql 不能拒（会把 agent 废掉，客户会关掉执行点）：%s", v.Reason)
	}
}

// 破坏性 + 普通目标（不是受保护的、也不是安全清单里的）→ 至少停下等人。
func TestDestructiveOnAnOrdinaryTargetAsksForAHuman(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "shell",
		map[string]string{"command": "rm -rf /home/dev/projects/old"}), Options{})
	if v.Decision != Ask {
		t.Fatalf("普通目标上的破坏性操作应当要求人工确认，得到 %s", v.Decision)
	}
}

// 安全清单生效时，清构建产物不该弹确认——否则客户第一天就关掉执行点。
func TestDeletingABuildArtifactDoesNotAsk(t *testing.T) {
	p := fixture()
	p.Guard.SafeTargets = policy.DefaultSafeTargets
	v := Decide(p, req("codex-tools", "shell",
		map[string]string{"command": "rm -rf ./node_modules"}), Options{})
	if v.Decision == Deny {
		t.Fatalf("清 node_modules 不该被拒：%s", v.Reason)
	}
	if v.Rule == RuleDestructive {
		t.Fatalf("安全清单内的目标不该升级为人工确认：%s", v.Reason)
	}
}

// 误报防线：一个叫 deleted_items.csv 的文件不该被当成删除动作。
func TestAFileNameIsNotADestructiveAction(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "read_file",
		map[string]string{"path": "/srv/app/deleted_items.csv"}), Options{})
	if v.Decision != Allow {
		t.Fatalf("读一个恰好叫 deleted_items.csv 的文件应当放行，得到 %s（%s）", v.Decision, v.Reason)
	}
}

// —— 不变式：执行点只会更严 ——

// 这是整个 guard 最重要的一条性质。拦截比放行危险得多：
// 如果执行点会在某个角落给出比策略更松的判定，那"策略说拒"就变成了空话。
func TestGuardNeverLoosensThePolicy(t *testing.T) {
	p := fixture()
	p.Guard.SafeTargets = policy.DefaultSafeTargets

	tools := []struct{ server, tool string }{
		{"codex-tools", "read_file"},
		{"codex-tools", "write_file"},
		{"codex-tools", "delete_file"},
		{"codex-tools", "shell"},
		{"filesystem", "list_dir"},
		{"filesystem", "nope_never_registered"},
	}
	argSets := []map[string]string{
		{"path": "/srv/app/src/main.go"},
		{"path": "/srv/backup/prod.sql"},
		{"path": "/srv/app/.env"},
		{"command": "ls -la"},
		{"command": "rm -rf /srv/app/backup"},
		{"command": "rm -rf ./node_modules"},
		{},
	}
	for _, tl := range tools {
		for i, args := range argSets {
			dec, _ := policy.Evaluate(p, tl.server, tl.tool, args)
			v := Decide(p, req(tl.server, tl.tool, args), Options{})
			if severity(v.Decision) < severityOfPolicy(dec) {
				t.Fatalf("%s/%s 参数组 %d：执行点 %s 比策略 %s 更松——不变式被破坏",
					tl.server, tl.tool, i, v.Decision, dec)
			}
		}
	}
}

// severityOfPolicy 把三态映射到与 Decision 同一个尺度。
// approve 与 ask 同级：两者都是"不能无人值守地执行"。
func severityOfPolicy(d model.Decision) int {
	switch d {
	case model.Allow:
		return 0
	case model.Approve:
		return 1
	}
	return 2
}

// —— 配置与失败模式 ——

func TestUnregisteredToolIsRefused(t *testing.T) {
	v := Decide(fixture(), req("codex-tools", "totally_unknown_tool",
		map[string]string{"x": "y"}), Options{})
	if v.Decision != Deny {
		t.Fatalf("未登记的工具必须拒绝（fail-closed），得到 %s", v.Decision)
	}
}

func TestDisabledGuardRecordsButDoesNotBlock(t *testing.T) {
	p := fixture()
	p.Guard.Enabled = false
	v := Decide(p, req("codex-tools", "delete_file",
		map[string]string{"path": "/srv/backup/x.sql"}), Options{})
	if v.Decision != Allow {
		t.Fatalf("关掉执行点后不该拦截，得到 %s", v.Decision)
	}
	if v.Rule != RuleGuardOff {
		t.Fatalf("关掉这件事本身要留在记录里，规则应为 %s，得到 %s", RuleGuardOff, v.Rule)
	}
	if !strings.Contains(v.Reason, "deny") {
		t.Fatalf("即使不拦，也要如实写出策略的判定：%s", v.Reason)
	}
}

// 记录里的值必须是脱敏的：产物要能证明拦了什么，但不能把内容写进去。
func TestMatchesCarryAHashAndNotTheRawValue(t *testing.T) {
	secretish := "/srv/app/backup/very-sensitive-dump-name.sql"
	v := Decide(fixture(), req("codex-tools", "delete_file",
		map[string]string{"path": secretish}), Options{})
	if len(v.Matches) == 0 {
		t.Fatal("应当有命中项")
	}
	m := v.Matches[0]
	if m.Hash == "" {
		t.Fatal("命中项必须带指纹：它是「当时确实是这个值」的唯一凭据")
	}
	if m.Value == secretish {
		t.Fatal("记录里不能出现完整原文")
	}
	if len([]rune(m.Value)) > 13 {
		t.Fatalf("展示形式应当截断，得到 %q", m.Value)
	}
}

// 同样的调用必须永远得到同样的判定——判定是纯函数，否则无法复算。
func TestDecisionIsDeterministic(t *testing.T) {
	r := req("codex-tools", "delete_file", map[string]string{"path": "/srv/app/backup/a.sql"})
	first := Decide(fixture(), r, Options{})
	for i := 0; i < 50; i++ {
		again := Decide(fixture(), r, Options{})
		if again.Decision != first.Decision || again.Rule != first.Rule {
			t.Fatalf("第 %d 次判定与第一次不同：%s/%s vs %s/%s",
				i, again.Decision, again.Rule, first.Decision, first.Rule)
		}
	}
}
