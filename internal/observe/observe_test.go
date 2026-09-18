package observe

import (
	"strings"
	"testing"

	"github.com/suhui-organization/ratchet/internal/model"
)

const sampleJSONL = `{"ts":"2026-09-18T10:00:00Z","agent":"codex","server":"filesystem","tool":"read_file"}
{"server":"filesystem","tool":"read_file"}
{"server":"filesystem","tool":"write_file"}
{"server":"github","tool":"create_pull_request"}

{"server":"ghost","tool":"not_in_config"}
this line is broken
{"server":"","tool":""}
`

func TestReadCallsCountsAndSkipped(t *testing.T) {
	s, err := ReadCalls(strings.NewReader(sampleJSONL))
	if err != nil {
		t.Fatal(err)
	}
	if s.Total != 5 {
		t.Fatalf("有效调用 = %d，期望 5", s.Total)
	}
	// 坏行必须被数出来：静默吞掉等于伪造"没有调用"
	if s.Skipped != 2 {
		t.Fatalf("坏行 = %d，期望 2", s.Skipped)
	}
	if s.Counts["filesystem/read_file"] != 2 {
		t.Fatalf("read_file 计数 = %d", s.Counts["filesystem/read_file"])
	}
}

func TestCompareFindsUnusedAndUnknown(t *testing.T) {
	inv := model.Inventory{Tools: []model.ToolObservation{
		{Server: "filesystem", Tool: "read_file"},
		{Server: "filesystem", Tool: "write_file"},
		{Server: "filesystem", Tool: "delete_file"},     // 从未被调用
		{Server: "internal", Tool: "acme_tool"},         // 从未被调用
		{Server: "github", Tool: "create_pull_request"}, // 调用过
	}}
	s, _ := ReadCalls(strings.NewReader(sampleJSONL))
	c := Compare(inv, s)

	if len(c.Unused) != 2 {
		t.Fatalf("未使用 = %v，期望 2 项", c.Unused)
	}
	if c.Unused[0] != "filesystem/delete_file" {
		t.Fatalf("未使用清单不对：%v", c.Unused)
	}
	// 被调用但不在清单里 = 清单不全或有人绕过配置
	if len(c.Unknown) != 1 || c.Unknown[0] != "ghost/not_in_config" {
		t.Fatalf("清单之外的调用 = %v", c.Unknown)
	}
	if len(c.Called) != 3 {
		t.Fatalf("被调用 = %v", c.Called)
	}
}

func TestApplyWritesCallCounts(t *testing.T) {
	inv := model.Inventory{Format: "ratchet-inventory/v1", Agent: "codex", Tools: []model.ToolObservation{
		{Server: "filesystem", Tool: "read_file"},
		{Server: "filesystem", Tool: "delete_file"},
	}}
	s, _ := ReadCalls(strings.NewReader(sampleJSONL))
	out := Apply(inv, s)
	if out.Agent != "codex" || out.Format != "ratchet-inventory/v1" {
		t.Fatalf("元信息丢了：%+v", out)
	}
	if out.Tools[0].Calls != 2 {
		t.Errorf("read_file calls = %d", out.Tools[0].Calls)
	}
	if out.Tools[1].Calls != 0 {
		t.Errorf("delete_file 从未被调用，calls 应为 0：%d", out.Tools[1].Calls)
	}
}

func TestValidateRejectsEmptyAndAllBroken(t *testing.T) {
	empty, _ := ReadCalls(strings.NewReader("\n\n"))
	if err := Validate(empty); err == nil {
		t.Fatal("空文件必须报错——不能当成「没有调用」")
	}
	broken, _ := ReadCalls(strings.NewReader("not json\nstill not json\n"))
	if err := Validate(broken); err == nil {
		t.Fatal("全部无法解析时也必须报错")
	}
	good, _ := ReadCalls(strings.NewReader(sampleJSONL))
	if err := Validate(good); err != nil {
		t.Fatalf("正常记录不该报错：%v", err)
	}
}
