package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/suhui-organization/ratchet/internal/observe"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func calls() []observe.Call {
	return []observe.Call{
		{TS: "2026-09-20T01:00:00Z", Agent: "a1", Server: "filesystem", Tool: "read_file", Decision: "allow", ArgKeys: []string{"path"}},
		{TS: "2026-09-20T01:00:01Z", Agent: "a1", Server: "filesystem", Tool: "write_file", Decision: "deny"},
		{TS: "2026-09-20T01:00:02Z", Agent: "a1", Server: "postgres", Tool: "query", Decision: "approve"},
	}
}

func TestBuildChainsFromZero(t *testing.T) {
	events, err := Build(calls(), "fallback-agent")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(events) != 3 || events[0].Seq != 0 || events[2].Seq != 2 {
		t.Fatalf("序号应当从 0 连续：%+v", events)
	}
	if events[0].PrevHash != "" {
		t.Fatalf("第一条的 prevHash 必须为空串：%q", events[0].PrevHash)
	}
	for i := 1; i < len(events); i++ {
		if events[i].PrevHash != events[i-1].Hash {
			t.Fatalf("第 %d 条没接上上一条", i)
		}
	}
	if events[0].Agent != "a1" {
		t.Fatalf("调用记录里的 agent 优先于兜底值：%q", events[0].Agent)
	}
}

func TestDigestIsStableAndMatchesTheEmittedJSON(t *testing.T) {
	events, err := Build(calls(), "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// 重算一遍必须得到同样的哈希：验证方就是这样重算的。
	for i, event := range events {
		again, err := Digest(event)
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		if again != event.Hash {
			t.Fatalf("第 %d 条哈希不稳定：%s vs %s", i, again, event.Hash)
		}
	}
	// 同样的输入必须得到同样的链（否则摘要不能当锚点被引用）。
	second, _ := Build(calls(), "")
	for i := range events {
		if events[i].Hash != second[i].Hash {
			t.Fatalf("同输入不同哈希，链不可复现：第 %d 条", i)
		}
	}
}

func TestDigestIgnoresFieldOrderNotContent(t *testing.T) {
	base := Event{Seq: 1, PrevHash: "p", Tool: "read_file"}
	digest, err := Digest(base)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	tampered := base
	tampered.Tool = "delete_file"
	other, _ := Digest(tampered)
	if digest == other {
		t.Fatal("改了工具名哈希却不变——链就白加了")
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	events, _ := Build(calls(), "")
	if err := WriteJSONL(path, events); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	back, err := ReadJSONL(path)
	if err != nil {
		t.Fatalf("ReadJSONL: %v", err)
	}
	if len(back) != len(events) {
		t.Fatalf("读回来 %d 条，写进去 %d 条", len(back), len(events))
	}
	head, count := Head(back)
	if count != 3 || head != events[2].Hash {
		t.Fatalf("摘要不对：%s / %d", head, count)
	}
}

func TestReadRejectsABadLineInsteadOfSkippingIt(t *testing.T) {
	// 坏行被跳过，就会让"断流"看起来正常——这正是要防的事。
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	os.WriteFile(path, []byte("{\"seq\":0,\"prevHash\":\"\"}\n这不是 JSON\n"), 0o600)
	if _, err := ReadJSONL(path); err == nil {
		t.Fatal("坏行必须报错")
	}
}

func TestWrittenLinesMatchTheVerifierCanonicalisation(t *testing.T) {
	// 验证器（Python）把事件解成 dict 后 sort_keys 重算；这里检查我们写出去的
	// JSON 里没有多余字段、也没有把 hash 自己算进去。
	events, _ := Build(calls(), "")
	raw, err := json.Marshal(events[1])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	json.Unmarshal(raw, &generic)
	if _, ok := generic["argKeys"]; ok {
		t.Fatal("空 argKeys 不该出现（omitempty）")
	}
	delete(generic, "hash")
	canonical, _ := json.Marshal(generic)
	sum := sha256Hex(canonical)
	if sum != events[1].Hash {
		t.Fatalf("写出去的字节与哈希口径不一致：%s vs %s", sum, events[1].Hash)
	}
}

func TestCommittedVectorStillMatches(t *testing.T) {
	// 仓库里的向量是 Go 与 Python 两边的共同锚点：Go 造出它，Python 的验证器读它。
	// 任何一边偷偷改了哈希口径，这个用例就会红——口径漂移是我们最想早发现的事。
	path := filepath.Join("..", "..", "spec", "testvectors", "silence", "events.jsonl")
	fromFile, err := ReadJSONL(path)
	if err != nil {
		t.Fatalf("读向量失败：%v", err)
	}
	rebuilt, err := Build(vector(), "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(fromFile) != len(rebuilt) {
		t.Fatalf("向量 %d 条，重建 %d 条", len(fromFile), len(rebuilt))
	}
	for i := range fromFile {
		if fromFile[i].Hash != rebuilt[i].Hash {
			t.Fatalf("第 %d 条哈希与向量不一致：%s vs %s（口径漂了）",
				i, fromFile[i].Hash, rebuilt[i].Hash)
		}
	}
}

func vector() []observe.Call {
	return []observe.Call{
		{TS: "2026-09-20T01:00:00Z", Agent: "a1", Server: "filesystem", Tool: "read_file", Decision: "allow", ArgKeys: []string{"path"}},
		{TS: "2026-09-20T01:00:01Z", Agent: "a1", Server: "filesystem", Tool: "write_file", Decision: "deny"},
		{TS: "2026-09-20T01:00:02Z", Agent: "a1", Server: "postgres", Tool: "query", Decision: "approve"},
	}
}
