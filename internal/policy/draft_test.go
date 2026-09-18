package policy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/suhui-organization/ratchet/internal/model"
)

func sample() model.Inventory {
	return model.Inventory{
		Format: "ratchet-inventory/v1",
		Agent:  "codex",
		Tools: []model.ToolObservation{
			{Server: "filesystem", Tool: "read_file", Calls: 120},
			{Server: "filesystem", Tool: "write_file", Calls: 9},
			{Server: "filesystem", Tool: "delete_file", Calls: 1},
			{Server: "filesystem", Tool: "list_directory", Calls: 40},
			{Server: "github", Tool: "create_pull_request", Calls: 3},
			{Server: "github", Tool: "get_issue"},
			{Server: "shell", Tool: "execute_command", Calls: 17},
			{Server: "internal", Tool: "acme_opaque_thing"},
		},
	}
}

func fixedNow() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) }

func TestDraftBucketsAndRationale(t *testing.T) {
	p := Draft(sample(), Options{Now: fixedNow(), Locale: LocaleZH})

	fs := p.Servers["filesystem"]
	if !reflect.DeepEqual(fs.Allow, []string{"list_directory", "read_file"}) {
		t.Errorf("filesystem.allow = %v", fs.Allow)
	}
	if !reflect.DeepEqual(fs.Approve, []string{"write_file"}) {
		t.Errorf("filesystem.approve = %v", fs.Approve)
	}
	if !reflect.DeepEqual(fs.Deny, []string{"delete_file"}) {
		t.Errorf("filesystem.deny = %v", fs.Deny)
	}
	if !reflect.DeepEqual(p.Servers["shell"].Approve, []string{"execute_command"}) {
		t.Errorf("shell.approve = %v", p.Servers["shell"].Approve)
	}

	// 未登记的一律拒绝是硬约束
	if p.DefaultDecision != model.Deny {
		t.Fatalf("defaultDecision = %s，必须是 deny", p.DefaultDecision)
	}
	if p.Agent != "codex" {
		t.Errorf("agent = %q", p.Agent)
	}

	// 每条判定都要有依据，且依据里要能看出"观测到没有"
	for key, reason := range p.Rationale {
		if reason == "" {
			t.Errorf("%s 缺依据", key)
		}
	}
	if r := p.Rationale["filesystem/delete_file"]; !strings.Contains(r, "观测到 1 次调用") {
		t.Errorf("delete_file 的依据没有体现观测次数：%q", r)
	}
	if r := p.Rationale["github/get_issue"]; !strings.Contains(r, "未观测到调用") {
		t.Errorf("get_issue 的依据没有体现未观测：%q", r)
	}
}

func TestDraftDeterministic(t *testing.T) {
	// 同一份输入必须产出逐字节相同的策略，否则没法进版本控制、也没法 diff
	a := Draft(sample(), Options{Now: fixedNow()})
	b := Draft(sample(), Options{Now: fixedNow()})
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatalf("两次编译结果不一致：\n%s\n%s", ja, jb)
	}
}

func TestDraftUnknownGoesToNeedsReview(t *testing.T) {
	p := Draft(sample(), Options{Now: fixedNow(), Locale: LocaleZH})
	if !reflect.DeepEqual(p.NeedsReview, []string{"internal/acme_opaque_thing"}) {
		t.Fatalf("needsReview = %v", p.NeedsReview)
	}
	if !reflect.DeepEqual(p.Servers["internal"].Approve, []string{"acme_opaque_thing"}) {
		t.Fatalf("未知工具应落在 approve：%v", p.Servers["internal"])
	}
}

func TestDraftStrictUnknownDenies(t *testing.T) {
	p := Draft(sample(), Options{StrictUnknown: true, Now: fixedNow()})
	if len(p.NeedsReview) != 0 {
		t.Fatalf("strict 模式下不应再有待确认项：%v", p.NeedsReview)
	}
	if !reflect.DeepEqual(p.Servers["internal"].Deny, []string{"acme_opaque_thing"}) {
		t.Fatalf("strict 模式下未知工具应为 deny：%v", p.Servers["internal"])
	}
}

func TestDraftDeduplicates(t *testing.T) {
	inv := model.Inventory{Tools: []model.ToolObservation{
		{Server: "s", Tool: "read_file", Calls: 1},
		{Server: "s", Tool: "read_file", Calls: 5},
	}}
	p := Draft(inv, Options{Now: fixedNow(), Locale: LocaleZH})
	if !reflect.DeepEqual(p.Servers["s"].Allow, []string{"read_file"}) {
		t.Fatalf("重复工具没有去重：%v", p.Servers["s"])
	}
	if r := p.Rationale["s/read_file"]; !strings.Contains(r, "5 次") {
		t.Fatalf("去重时应保留较大调用次数：%q", r)
	}
}

func TestDraftSkipsIncompleteTools(t *testing.T) {
	inv := model.Inventory{Tools: []model.ToolObservation{
		{Server: "", Tool: "read_file"},
		{Server: "s", Tool: ""},
		{Server: "s", Tool: "read_file"},
	}}
	p := Draft(inv, Options{Now: fixedNow()})
	_, tools, _, _, _ := Counts(p)
	if tools != 1 {
		t.Fatalf("缺 server/tool 的条目应被跳过，实际工具数 %d", tools)
	}
}

func TestDraftEnglishRationale(t *testing.T) {
	// 默认语言是 en-US：产物是给客户看的，而站点与用户群是英文。
	// 这一条守的是"整篇英文"——依据里出现任何一个中文字符都算失败。
	p := Draft(sample(), Options{Now: fixedNow()})
	if r := p.Rationale["filesystem/delete_file"]; !strings.Contains(r, `name matches "delete"`) ||
		!strings.Contains(r, "observed 1 call(s)") {
		t.Fatalf("英文依据不对：%q", r)
	}
	if r := p.Rationale["github/get_issue"]; !strings.Contains(r, "never observed") {
		t.Fatalf("未观测的英文依据不对：%q", r)
	}
	if r := p.Rationale["internal/acme_opaque_thing"]; !strings.Contains(r, "no capability keyword") {
		t.Fatalf("未知工具的英文依据不对：%q", r)
	}
	for key, reason := range p.Rationale {
		for _, r := range reason {
			if r >= 0x4e00 && r <= 0x9fff {
				t.Fatalf("%s 的依据里出现中文：%q", key, reason)
			}
		}
	}
}
