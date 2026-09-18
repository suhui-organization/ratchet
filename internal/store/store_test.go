package store

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/suhui-organization/ratchet/internal/observe"
)

func TestAppendCreatesDirAndWritesOneLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "calls.jsonl")
	if err := Append(path, observe.Call{Server: "filesystem", Tool: "read_file"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "\n") != 1 {
		t.Fatalf("应恰好写一行：%q", raw)
	}
	// 目录权限 0700 / 文件 0600：调用记录里可能有路径与会话 id
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("文件权限 = %v，期望 0600", info.Mode().Perm())
	}
}

func TestAppendFillsTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	if err := Append(path, observe.Call{Server: "s", Tool: "t"}); err != nil {
		t.Fatal(err)
	}
	s, err := observe.ReadCallsFile(path)
	if err != nil || s.Total != 1 {
		t.Fatalf("写出的记录读不回来：%v / %+v", err, s)
	}
}

func TestConcurrentAppendsDoNotCorrupt(t *testing.T) {
	// hook 会被并发调用；O_APPEND 保证每行原子落盘，读回来必须行行可解析
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = Append(path, observe.Call{Server: "s", Tool: "t"})
		}(i)
	}
	wg.Wait()

	s, err := observe.ReadCallsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 0 {
		t.Fatalf("并发写入产生了 %d 个坏行", s.Skipped)
	}
	if s.Total != 40 {
		t.Fatalf("写入 %d 条，期望 40", s.Total)
	}
}
