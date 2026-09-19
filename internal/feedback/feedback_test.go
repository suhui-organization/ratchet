package feedback

import (
	"strings"
	"testing"
)

func TestRedactRemovesHomeAndKeepsShape(t *testing.T) {
	got := Redact("failed reading /Users/alice/projects/secret-client/.env", "/Users/alice")
	if strings.Contains(got, "alice") || strings.Contains(got, "secret-client") {
		t.Fatalf("还留着可识别信息：%q", got)
	}
	// 只留最后一段：`.env` 能说明"密钥文件被拒了"，而不说明是谁的
	if !strings.Contains(got, ".env") {
		t.Errorf("末尾路径段应该保留：%q", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(got), "failed reading …/") {
		t.Errorf("其余层级应折叠：%q", got)
	}
}

func TestRedactHandlesBareAbsolutePaths(t *testing.T) {
	got := Redact("scanned /etc/ssl/private/key.pem and /var/tmp/x", "/home/me")
	if !strings.Contains(got, "…/") {
		t.Fatalf("长路径应折叠成 …/最后一段：%q", got)
	}
	if strings.Contains(got, "/etc/ssl/private") {
		t.Fatalf("中间层级不该保留：%q", got)
	}
}

func TestBuildNeverSendsAnything(t *testing.T) {
	// 这一条守的是产品承诺：反馈正文只是文本，工具不替用户发出去
	out := Build(Input{Version: "0.12.0", Summary: "reading .env got flagged", Home: "/home/me"})
	if !strings.Contains(out, "you decide what leaves your machine") {
		t.Fatal("正文里必须写明是用户自己决定发不发")
	}
	if strings.Contains(out, "POST ") || strings.Contains(out, "http.POST") {
		t.Fatal("不该包含任何发送逻辑")
	}
}

func TestBuildIncludesCountsAndSamples(t *testing.T) {
	out := Build(Input{
		Version: "0.12.0", Summary: "x", Home: "/home/me",
		Counts:  map[string]int{"servers": 16, "unpinned": 12},
		Samples: []string{"filesystem/read_file denied by **/.ssh/**"},
	})
	if !strings.Contains(out, "servers 16") || !strings.Contains(out, "unpinned 12") {
		t.Errorf("计数没带出来：%s", out)
	}
	if !strings.Contains(out, "filesystem/read_file") {
		t.Errorf("判定样本没带出来：%s", out)
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	once := Redact("path /home/me/a/b/c.txt", "/home/me")
	twice := Redact(once, "/home/me")
	if once != twice {
		t.Fatalf("重复打码不该继续改变内容：%q → %q", once, twice)
	}
}
