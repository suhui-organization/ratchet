package policy

import (
	"testing"

	"github.com/suhui-organization/ratchet/internal/model"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want Capability
	}{
		// 破坏性：不可逆操作必须最高优先
		{name: "delete_file", want: CapDestructive},
		{name: "filesystem.delete_file", want: CapDestructive},
		{name: "deleteFile", want: CapDestructive},
		{name: "rmdir", want: CapDestructive},
		{name: "drop_table", want: CapDestructive},
		// 执行
		{name: "execute_command", want: CapExecute},
		{name: "run_shell", want: CapExecute},
		{name: "bash", want: CapExecute},
		// 写入
		{name: "write_file", want: CapWrite},
		{name: "create_pull_request", want: CapWrite},
		{name: "update_issue", want: CapWrite},
		{name: "send_email", want: CapWrite},
		// 出网
		{name: "fetch_url", want: CapNetwork},
		{name: "http_request", want: CapNetwork},
		{name: "web_search", want: CapNetwork},
		// 读取
		{name: "read_file", want: CapRead},
		{name: "list_directory", want: CapRead},
		{name: "search_files", want: CapRead},
		{name: "get_weather", want: CapRead},
		// 未知
		{name: "frobnicate", want: CapUnknown},
		{name: "acme_opaque_thing", want: CapUnknown},
		// 危险度优先：既能读又能删 → 按删处理
		{name: "read_and_delete_records", want: CapDestructive},
		{name: "get_or_create_user", want: CapWrite},
		// 名字判不出来时看描述
		{name: "xyz", desc: "Permanently removes the given record", want: CapDestructive},
		{name: "blob", desc: "Returns the current value", want: CapRead},
	}
	for _, c := range cases {
		got, reason := Classify(model.ToolObservation{Server: "s", Tool: c.name, Description: c.desc})
		if got != c.want {
			t.Errorf("%q（描述 %q）判定 %s，期望 %s（依据：%s）", c.name, c.desc, got, c.want, reason)
		}
		if reason == "" {
			t.Errorf("%q：依据不能为空——策略里要能看懂为什么这么判", c.name)
		}
	}
}

func TestTokenizeHandlesCaseAndSeparators(t *testing.T) {
	// 同一个工具名的几种写法必须拆出同样的词元，否则会出现"换个命名风格就漏判"
	cases := map[string][]string{
		"read_file":    {"read", "file"},
		"read-file":    {"read", "file"},
		"read.file":    {"read", "file"},
		"readFile":     {"read", "file"},
		"Read File":    {"read", "file"},
		"fs/read_file": {"fs", "read", "file"},
	}
	for in, want := range cases {
		got := tokenize(in)
		if len(got) != len(want) {
			t.Fatalf("%q → %v，期望 %v", in, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q → %v，期望 %v", in, got, want)
			}
		}
	}
}

func TestDecideMapping(t *testing.T) {
	cases := []struct {
		cap        Capability
		strict     bool
		want       model.Decision
		wantReview bool
	}{
		{CapDestructive, false, model.Deny, false},
		{CapExecute, false, model.Approve, false},
		{CapWrite, false, model.Approve, false},
		{CapNetwork, false, model.Approve, false},
		{CapRead, false, model.Allow, false},
		{CapUnknown, false, model.Approve, true},
		{CapUnknown, true, model.Deny, false},
	}
	for _, c := range cases {
		got, review := Decide(c.cap, c.strict)
		if got != c.want || review != c.wantReview {
			t.Errorf("%s(strict=%v) → %s/%v，期望 %s/%v", c.cap, c.strict, got, review, c.want, c.wantReview)
		}
	}
}
