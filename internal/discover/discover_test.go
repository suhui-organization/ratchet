package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsClaudeCodeAndParsesServers(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".claude.json"), `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
      "env": { "GITHUB_TOKEN": "ghp_thisMustNeverAppear", "PLAIN": "x" }
    }
  }
}`)
	report := Scan(home, t.TempDir())
	if len(report.Harnesses) != 1 {
		t.Fatalf("期望 1 个 harness，实际 %d", len(report.Harnesses))
	}
	h := report.Harnesses[0]
	if h.ID != "claude-code" || !h.Parsed {
		t.Fatalf("harness = %+v", h)
	}
	if len(h.Servers) != 1 || h.Servers[0].Name != "filesystem" {
		t.Fatalf("servers = %+v", h.Servers)
	}
	if len(h.Servers[0].EnvKeys) != 2 {
		t.Fatalf("envKeys = %v", h.Servers[0].EnvKeys)
	}
	for _, s := range h.Servers {
		if s.Command == "ghp_thisMustNeverAppear" || s.Risk == "ghp_thisMustNeverAppear" {
			t.Fatal("密钥值被读进了结构体")
		}
		for _, a := range s.Args {
			if a == "ghp_thisMustNeverAppear" {
				t.Fatal("密钥值出现在 args 里")
			}
		}
	}
}

func TestScanParsesCodexTOML(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".codex/config.toml"), `
[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem@1.2.3", "/tmp"]

[mcp_servers.internal]
command = "uvx"
args = ["acme-mcp"]
`)
	report := Scan(home, t.TempDir())
	if len(report.Harnesses) != 1 || report.Harnesses[0].ID != "codex" {
		t.Fatalf("harnesses = %+v", report.Harnesses)
	}
	servers := report.Harnesses[0].Servers
	if len(servers) != 2 {
		t.Fatalf("期望 2 个 server，实际 %d：%+v", len(servers), servers)
	}
	byName := map[string]Server{}
	for _, s := range servers {
		byName[s.Name] = s
	}
	if byName["filesystem"].Risk != "" {
		t.Errorf("锁了版本的 server 不该报警：%q", byName["filesystem"].Risk)
	}
	if byName["internal"].Risk == "" {
		t.Error("uvx 拉未锁版本的包应该报警")
	}
}

func TestLatestTagIsStillUnpinned(t *testing.T) {
	// 真机数据逼出来的用例：`@latest` 看起来像锁了版本，实际每次都拉最新。
	home := t.TempDir()
	write(t, filepath.Join(home, ".claude.json"), `{
  "mcpServers": {
    "a": {"command": "npx", "args": ["-y", "chrome-devtools-mcp@latest"]},
    "b": {"command": "npx", "args": ["-y", "pkg@^1.2.0"]},
    "c": {"command": "npx", "args": ["-y", "pkg@1.2.3"]},
    "d": {"command": "npx", "args": ["-y", "@scope/pkg@2.0.0"]}
  }
}`)
	report := Scan(home, t.TempDir())
	risky := map[string]bool{}
	for _, s := range report.Harnesses[0].Servers {
		risky[s.Name] = s.Risk != ""
	}
	if !risky["a"] {
		t.Error("@latest 必须算未锁版本")
	}
	if !risky["b"] {
		t.Error("范围版本 ^1.2.0 必须算未锁版本")
	}
	if risky["c"] {
		t.Error("pkg@1.2.3 是锁了版本的")
	}
	if risky["d"] {
		t.Error("@scope/pkg@2.0.0 是锁了版本的")
	}
}

func TestScanFindsProjectLevelConfig(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	write(t, filepath.Join(work, ".mcp.json"),
		`{"mcpServers":{"github":{"command":"npx","args":["-y","@modelcontextprotocol/server-github"]}}}`)
	report := Scan(home, work)
	if len(report.Harnesses) != 1 || report.Harnesses[0].ID != "project" {
		t.Fatalf("项目级配置没被发现：%+v", report.Harnesses)
	}
	if len(report.Unpinned()) != 1 {
		t.Fatalf("未锁版本统计 = %v", report.Unpinned())
	}
}

func TestScanReportsUnparsedButNeverSilentlyDrops(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".config/zed/settings.json"), `{"context_servers":{}}`)
	report := Scan(home, t.TempDir())
	if len(report.Unparsed) != 1 {
		t.Fatalf("不认识的格式必须显式列出，实际 %v", report.Unparsed)
	}
	if len(report.Harnesses) != 1 || report.Harnesses[0].Parsed {
		t.Fatalf("未解析的 harness 应标记 parsed=false：%+v", report.Harnesses)
	}
}

func TestScanOnEmptyHomeIsNotAnError(t *testing.T) {
	report := Scan(t.TempDir(), t.TempDir())
	if len(report.Harnesses) != 0 || len(report.Unparsed) != 0 {
		t.Fatalf("空环境应得到空报告：%+v", report)
	}
}

func TestBrokenConfigIsReportedNotIgnored(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".cursor/mcp.json"), `{ this is not json`)
	report := Scan(home, t.TempDir())
	if len(report.Unparsed) == 0 {
		t.Fatal("坏配置必须被报出来，不能当成「没有 server」")
	}
}
