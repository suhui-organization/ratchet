// Package feedback 生成一份**用户可以先看、再决定发不发**的反馈正文。
//
// 为什么不自动上报：这个产品的卖点就是"数据不出机器"。
// 加一行遥测，卖点就没了。所以反馈只能由用户主动发起——
// 我们能做的是把"发什么"准备好，并且**默认把能识别到个人的部分打掉**。
package feedback

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Input 是反馈要带上的事实。全部来自用户本机，且都可被打码。
type Input struct {
	Version  string
	Kind     string // bug / false-positive / feature
	Summary  string // 用户的一句话描述
	Detail   string // 补充（例如 think 某条规则为什么是误报）
	Home     string
	Counts   map[string]int
	Samples  []string // 例如被拒的 server/tool 与命中的规则
	IssueURL string
}

// 路径类字符串：绝对路径、home 下的路径、Windows 路径。
// 也匹配已经被换成 `<home>` 的路径——两遍处理要能接上。
var pathRe = regexp.MustCompile(`(?:^|[\s"'=(])((?:<home>|[A-Za-z]:\\|/|~/)[^\s"',;:)\]]*)`)

// Redact 把能识别到个人或项目的路径换成占位符。
//
// 只保留**最后一段**（文件名与扩展名），其余全折叠成 `…`。
//
// 为什么只留一段：中间层级里有客户名、项目名、用户名。
// 那些对定位问题没用，但会跟着 issue 一起公开。留 `.env` 这种扩展名就够了——
// 它能说明"是密钥文件被拒了"，而不说明是谁的。
func Redact(s, home string) string {
	if home != "" {
		s = strings.ReplaceAll(s, home, "<home>")
	}
	return pathRe.ReplaceAllStringFunc(s, func(m string) string {
		lead := ""
		if len(m) > 0 && (m[0] == ' ' || m[0] == '"' || m[0] == '\'' || m[0] == '(' || m[0] == '=') {
			lead = string(m[0])
			m = m[1:]
		}
		trimmed := strings.TrimRight(m, "/")
		parts := strings.Split(trimmed, "/")
		last := parts[len(parts)-1]
		if last == "" || strings.HasPrefix(last, "…") {
			return lead + "<path>"
		}
		if len(parts) == 1 && !strings.Contains(trimmed, "home") {
			return lead + last // 单段（例如相对文件名），本来就不含目录信息
		}
		return lead + "…/" + last
	})
}

// Build 生成 Markdown 正文。
//
// 它**不发送任何东西**：只把正文打印出来，用户自己贴到 issue 里。
func Build(in Input) string {
	var b strings.Builder
	kind := in.Kind
	if kind == "" {
		kind = "bug"
	}
	b.WriteString("### What happened\n\n")
	b.WriteString(orDash(Redact(in.Summary, in.Home)))
	b.WriteString("\n\n### Environment\n\n")
	fmt.Fprintf(&b, "- ratchet: `%s`\n", in.Version)
	if len(in.Counts) > 0 {
		keys := make([]string, 0, len(in.Counts))
		for k := range in.Counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("- counts: ")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "%s %d", k, in.Counts[k])
		}
		b.WriteString("\n")
	}
	if in.Detail != "" {
		b.WriteString("\n### Detail\n\n")
		b.WriteString(Redact(in.Detail, in.Home))
		b.WriteString("\n")
	}
	if len(in.Samples) > 0 {
		b.WriteString("\n### What the tool decided\n\n```\n")
		for _, s := range in.Samples {
			b.WriteString(Redact(s, in.Home))
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}
	b.WriteString("\n---\n")
	b.WriteString("Every path above was reduced to `…/its-last-segment` (so `.env` survives, but not whose `.env`). ")
	b.WriteString("**Read it once more before posting** — you decide what leaves your machine.\n")
	if in.IssueURL != "" {
		fmt.Fprintf(&b, "\nOpen an issue: %s\n", in.IssueURL)
	}
	_ = kind
	return b.String()
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "_(describe it here)_"
	}
	return s
}
