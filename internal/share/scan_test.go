package share

import (
	"strings"
	"testing"
	"time"

	"github.com/suhui-organization/ratchet/internal/discover"
)

func sampleReport() discover.Report {
	return discover.Report{
		Home: "/home/me",
		Harnesses: []discover.Harness{{
			ID: "codex", Name: "Codex", Parsed: true,
			Servers: []discover.Server{
				{Name: "filesystem", Command: "npx", Args: []string{"-y", "@mcp/server-filesystem"}},
				{Name: "github", Command: "npx", Args: []string{"-y", "pkg@1.2.3"}, Risk: ""},
				{Name: "risky", Command: "npx", Args: []string{"-y", "some-mcp"}, Risk: "依赖未锁版本（some-mcp）"},
			},
		}},
		Unparsed: []string{"/home/me/.config/zed/settings.json"},
	}
}

func render() string {
	return RenderScan(sampleReport(), Meta{Version: "0.11.0", Generated: time.Unix(1700000000, 0)})
}

func TestCountsAppear(t *testing.T) {
	p := render()
	for _, want := range []string{">3</b><span>MCP servers", ">1</b><span>agent harnesses", ">1</b><span>unpinned"} {
		if !strings.Contains(p, want) {
			t.Errorf("缺统计：%s", want)
		}
	}
}

func TestUnpinnedIsMarkedAndOthersAreNot(t *testing.T) {
	p := render()
	if strings.Count(p, `class="flag"`) != 1 {
		t.Fatalf("应恰好标记 1 个未锁版本，实际 %d", strings.Count(p, `class="flag"`))
	}
	if strings.Count(p, `class="dot on"`) != 1 {
		t.Fatalf("点阵应恰好有 1 个亮点，实际 %d", strings.Count(p, `class="dot on"`))
	}
}

func TestUnparsedIsReportedNotHidden(t *testing.T) {
	p := render()
	if !strings.Contains(p, "Configs we could not parse") {
		t.Fatal("未解析的配置必须出现在页面上")
	}
	if !strings.Contains(p, "zed/settings.json") {
		t.Fatal("未解析的配置路径要列出来")
	}
}

func TestEverythingFromConfigIsEscaped(t *testing.T) {
	// server 名与路径是外部输入，直接拼进 HTML 就是注入口
	rep := discover.Report{
		Home: "/home/x",
		Harnesses: []discover.Harness{{
			ID: "evil", Parsed: true,
			Servers: []discover.Server{{
				Name:    `<script>alert(1)</script>`,
				Command: "npx", Args: []string{"<img src=x onerror=alert(2)>"},
			}},
		}},
		Unparsed: []string{`"><script>alert(3)</script>`},
	}
	p := RenderScan(rep, Meta{Version: "0.11.0", Generated: time.Unix(1700000000, 0)})
	if strings.Contains(p, "<script>alert(1)</script>") {
		t.Fatal("server 名没有被转义")
	}
	if strings.Contains(p, "<img src=x") {
		t.Fatal("命令行没有被转义")
	}
	if strings.Contains(p, "<script>alert(3)</script>") {
		t.Fatal("未解析路径没有被转义")
	}
	if !strings.Contains(p, "&lt;script&gt;") {
		t.Fatal("转义后的实体应该出现在页面上")
	}
}

func TestPageIsSelfContained(t *testing.T) {
	// 单文件才能被转发：不许有 CDN、外链字体、外链脚本
	p := render()
	for _, forbidden := range []string{"http://", "https://cdn", "<script", "src=\"http"} {
		if strings.Contains(p, forbidden) {
			t.Errorf("页面不该包含外部依赖：%q", forbidden)
		}
	}
	if !strings.HasPrefix(p, "<!doctype html>") {
		t.Fatal("应是一份完整 HTML")
	}
}
