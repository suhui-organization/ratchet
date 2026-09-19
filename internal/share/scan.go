// Package share 把一次扫描渲染成一个**自包含的静态页**。
//
// 为什么是单文件：内容要能被转发——贴到 issue、发给客户、丢进聊天窗口。
// 任何"得先部署一个站"的方案都会在转发这一步流失人。
// 单文件 HTML 没有任何外部依赖：没有 CDN、没有字体请求、没有脚本。
package share

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/suhui-organization/ratchet/internal/discover"
)

// Meta 是页面上的来源信息。
type Meta struct {
	Version   string
	Generated time.Time
}

// RenderScan 生成页面。
//
// 纪律：所有来自配置的字符串都走 html.EscapeString——
// server 名与路径是**外部输入**，直接拼进 HTML 就是一个注入口。
func RenderScan(rep discover.Report, meta Meta) string {
	servers := 0
	unpinned := 0
	var rows []string
	for _, h := range rep.Harnesses {
		for _, s := range h.Servers {
			servers++
			mark := ""
			if s.Risk != "" {
				unpinned++
				mark = `<span class="flag">unpinned</span>`
			}
			rows = append(rows, fmt.Sprintf(
				`<tr><td class="mono">%s</td><td class="mono">%s</td><td>%s</td><td class="mono dim">%s</td></tr>`,
				html.EscapeString(h.ID), html.EscapeString(s.Name), mark, html.EscapeString(desc(s))))
		}
	}
	sort.Strings(rows)

	// 点阵：每个 server 一格，未锁版本的填成品牌色
	dots := make([]string, 0, servers)
	for i := 0; i < servers; i++ {
		cls := "dot"
		if i < unpinned {
			cls = "dot on"
		}
		dots = append(dots, `<span class="`+cls+`"></span>`)
	}

	var unparsed strings.Builder
	if len(rep.Unparsed) > 0 {
		unparsed.WriteString(`<h2>Configs we could not parse</h2><p class="lead">These exist and were deliberately left unread rather than guessed at.`)
		unparsed.WriteString(` Anything in them is not in the numbers above.</p><ul class="mono small">`)
		for _, u := range rep.Unparsed {
			unparsed.WriteString("<li>" + html.EscapeString(u) + "</li>")
		}
		unparsed.WriteString("</ul>")
	}

	ts := meta.Generated.UTC().Format("2006-01-02 15:04 UTC")
	return strings.NewReplacer(
		"{{SERVERS}}", fmt.Sprint(servers),
		"{{HARNESSES}}", fmt.Sprint(len(rep.Harnesses)),
		"{{UNPINNED}}", fmt.Sprint(unpinned),
		"{{DOTS}}", strings.Join(dots, ""),
		"{{ROWS}}", strings.Join(rows, "\n"),
		"{{UNPARSED}}", unparsed.String(),
		"{{HOME}}", html.EscapeString(rep.Home),
		"{{TS}}", ts,
		"{{VERSION}}", html.EscapeString(meta.Version),
	).Replace(pageTemplate)
}

func desc(s discover.Server) string {
	if s.URL != "" {
		return "remote " + s.URL
	}
	return strings.TrimSpace(s.Command + " " + strings.Join(s.Args, " "))
}

// pageTemplate 里没有任何外部请求：字体走系统栈，样式内联，无脚本。
const pageTemplate = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>MCP servers on this machine — Ratchet</title>
<style>
:root{--ink:#202124;--muted:#8691a1;--faint:#9ca3af;--line:#e5e7eb;--brand:#4d6bfe;--bg:#fff;--soft:#f7f8fa}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:16px/1.6 ui-sans-serif,system-ui,-apple-system,"Segoe UI","PingFang SC",sans-serif;-webkit-font-smoothing:antialiased}
.wrap{max-width:860px;margin:0 auto;padding:56px 24px 72px}
h1{font-size:2rem;line-height:1.15;letter-spacing:-.03em;margin:0 0 8px}
h2{font-size:1.05rem;margin:44px 0 8px}
.lead{color:var(--muted);margin:0 0 28px}
.mono{font:13px/1.5 ui-monospace,"SF Mono",Menlo,monospace}
.small{font-size:12px}
.dim{color:var(--faint)}
.stats{display:flex;gap:32px;flex-wrap:wrap;margin:28px 0 8px}
.stat b{display:block;font-size:1.5rem;font-variant-numeric:tabular-nums}
.stat span{color:var(--muted);font-size:13px}
.dots{display:flex;flex-wrap:wrap;gap:5px;margin:18px 0 6px}
.dot{width:14px;height:14px;border-radius:3px;background:var(--line)}
.dot.on{background:var(--brand)}
.note{color:var(--muted);font-size:14px;margin:14px 0 0;max-width:62ch}
table{width:100%;border-collapse:collapse;margin-top:14px}
th{text-align:left;font-size:12px;color:var(--faint);font-weight:500;border-bottom:1px solid var(--line);padding:8px 10px}
td{border-bottom:1px solid #f0f1f4;padding:9px 10px;vertical-align:top}
.flag{color:#b35200;background:#fff4e5;border-radius:4px;padding:1px 6px;font-size:12px}
ul{margin:8px 0 0;padding-left:20px;color:var(--muted)}
footer{margin-top:56px;padding-top:20px;border-top:1px solid var(--line);color:var(--faint);font-size:13px}
code{background:var(--soft);padding:2px 6px;border-radius:4px;font-family:ui-monospace,Menlo,monospace;font-size:12.5px}
.warn{background:#fff8e1;border:1px solid #ffe0a3;border-radius:8px;padding:12px 14px;color:#7a4a05;font-size:13.5px;margin:0 0 28px}
</style></head><body><div class="wrap">

<h1>MCP servers on this machine</h1>
<p class="lead">Read from configuration files. Nothing was executed to build this picture.</p>

<p class="warn">This page contains local paths and server names from one machine. Check it before sharing it publicly.</p>

<div class="stats">
  <div class="stat"><b>{{SERVERS}}</b><span>MCP servers</span></div>
  <div class="stat"><b>{{HARNESSES}}</b><span>agent harnesses</span></div>
  <div class="stat"><b>{{UNPINNED}}</b><span>unpinned</span></div>
</div>

<div class="dots">{{DOTS}}</div>
<p class="note">Each square is one MCP server; the filled ones launch from a package manager without
pinning a version. That means the package can change under you and the config looks identical
before and after. <code>@latest</code> counts as unpinned — it reads like a version and resolves like nothing.</p>

<h2>Servers</h2>
<table><thead><tr><th>harness</th><th>server</th><th></th><th>how it is launched</th></tr></thead>
<tbody>{{ROWS}}</tbody></table>

{{UNPARSED}}

<footer>
  <p>Scanned <code>{{HOME}}</code> on {{TS}} with Ratchet {{VERSION}}.<br>
  Reproduce it: <code>ratchet scan --home ~ --share out.html</code></p>
  <p>Ratchet compiles least-privilege policy from what your agents actually reach, and hands over
  evidence the recipient verifies on their own machine.</p>
</footer>

</div></body></html>
`
