package mcp

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func python() string {
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	return "python3"
}

func TestListToolsAgainstFakeServer(t *testing.T) {
	fixture := filepath.Join("testdata", "fake_server.py")
	tools, err := ListTools(Options{Command: python(), Args: []string{fixture}, Timeout: 15 * time.Second})
	if err != nil {
		t.Fatalf("introspect 失败：%v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("期望 2 个工具，实际 %d：%+v", len(tools), tools)
	}
	if tools[0].Name != "read_file" || tools[0].Description == "" {
		t.Fatalf("工具解析不对：%+v", tools[0])
	}
}

func TestTimeoutIsAnErrorNotAnEmptyList(t *testing.T) {
	// "连不上"与"这个 server 没有工具"是两件事。返回空列表会让使用者
	// 以为某个 server 是干净的——那是最危险的误读。
	fixture := filepath.Join("testdata", "hanging_server.py")
	start := time.Now()
	tools, err := ListTools(Options{Command: python(), Args: []string{fixture}, Timeout: 2 * time.Second})
	if err == nil {
		t.Fatalf("超时必须报错，实际返回 %+v", tools)
	}
	if tools != nil {
		t.Fatalf("超时时不应返回工具：%+v", tools)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("超时控制失效，用时 %v", elapsed)
	}
}

func TestMissingCommandIsRejected(t *testing.T) {
	if _, err := ListTools(Options{}); err == nil {
		t.Fatal("没有 command 时应直接拒绝")
	}
}

func TestServerThatExitsImmediately(t *testing.T) {
	tools, err := ListTools(Options{Command: python(), Args: []string{"-c", "pass"}, Timeout: 5 * time.Second})
	if err == nil {
		t.Fatalf("server 直接退出应报错，实际 %+v", tools)
	}
}
