// Command ratchet 是跑在你自己机器上的引擎：
// 把 agent 用到的工具清单编译成一份最小权限策略。
//
// 设计约束（见 docs/DECISIONS.md D2）：整个引擎编译成单个静态二进制，
// 目标机器不需要 Node、Python 或任何运行时——"装得上"是上一代产品最大的教训。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/suhui-organization/ratchet/internal/discover"
	"github.com/suhui-organization/ratchet/internal/mcp"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/policy"
)

const version = "0.1.0"

// InventoryFormat 是本工具认识的清单格式标识。
const InventoryFormat = "ratchet-inventory/v1"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("ratchet %s\n", version)
	case "policy":
		os.Exit(cmdPolicy(os.Args[2:]))
	case "scan":
		os.Exit(cmdScan(os.Args[2:]))
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ratchet — 从 agent 的真实行为编译最小权限策略

用法：
  ratchet version
  ratchet scan [--home <dir>] [--workdir <dir>] [--introspect]
               [--out <清单.json>] [--timeout <秒>] [--json]
  ratchet policy draft --from <清单.json> [--out <策略.json>]
                       [--agent <名字>] [--strict-unknown] [--json]

说明：
  scan          只读本机配置，列出装了哪些 agent、挂了哪些 MCP server。
                默认**不执行任何东西**；加 --introspect 才会连上 server 取工具名
                （连上就会执行配置里写的命令，所以必须显式开启）。

  policy draft  读工具清单，编译出最小权限策略；每条判定都带依据。
                未登记的工具一律拒绝；无法判定能力的默认置为 approve 并列入待确认。

  --strict-unknown  无法判定能力的工具直接 deny（默认是 approve + 待确认）
  --json            把策略 JSON 打到 stdout（不给 --out 时也能用管道接）
`)
}

func cmdScan(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	home := fs.String("home", "", "扫描哪个 HOME（默认当前用户的）")
	work := fs.String("workdir", "", "项目级配置所在目录（默认当前目录）")
	introspect := fs.Bool("introspect", false, "连上每个 server 取工具名（会执行配置里的命令）")
	out := fs.String("out", "", "把清单写到这个文件（需要 --introspect）")
	timeout := fs.Int("timeout", 20, "单个 server 的 introspect 超时（秒）")
	asJSON := fs.Bool("json", false, "把扫描报告打成 JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			*home = h
		} else {
			fmt.Fprintf(os.Stderr, "无法确定 HOME：%v\n", err)
			return 1
		}
	}
	if *work == "" {
		if wd, err := os.Getwd(); err == nil {
			*work = wd
		}
	}
	if *out != "" && !*introspect {
		// 静态扫描只知道 server，不知道工具名。这一点必须讲清楚，
		// 否则用户会以为"没报错就是没问题"，实际上清单是空的。
		fmt.Fprintln(os.Stderr, "静态扫描只知道有哪些 server，不知道它们暴露了哪些工具。")
		fmt.Fprintln(os.Stderr, "要生成可编译的清单，请加 --introspect（会执行配置里写的命令）。")
		return 2
	}

	report := discover.Scan(*home, *work)

	var inv model.Inventory
	failures := map[string]string{}
	if *introspect {
		inv = model.Inventory{Format: InventoryFormat, Agent: primaryHarness(report)}
		for _, h := range report.Harnesses {
			for _, s := range h.Servers {
				tools, err := mcp.ListTools(mcp.Options{
					Command: s.Command,
					Args:    s.Args,
					Env:     os.Environ(),
					Timeout: time.Duration(*timeout) * time.Second,
				})
				if err != nil {
					// 连不上与"没有工具"是两件事，必须分别记录
					failures[h.ID+"/"+s.Name] = err.Error()
					continue
				}
				for _, tool := range tools {
					inv.Tools = append(inv.Tools, model.ToolObservation{
						Server:      s.Name,
						Tool:        tool.Name,
						Description: tool.Description,
					})
				}
			}
		}
	}

	if *asJSON {
		payload := map[string]any{"scan": report, "introspect_failures": failures}
		if *introspect {
			payload["inventory"] = inv
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON 失败：%v\n", err)
			return 1
		}
	} else {
		renderScan(os.Stdout, report, inv, failures, *introspect)
	}

	if *out != "" {
		if len(inv.Tools) == 0 {
			// 一份没有工具的清单会让 policy draft 直接报错；这里先讲清楚原因。
			fmt.Fprintln(os.Stderr, "没有取到任何工具，不写清单——检查上面的失败原因。")
			return 1
		}
		if err := writeJSON(*out, inv); err != nil {
			fmt.Fprintf(os.Stderr, "写入清单失败：%v\n", err)
			return 1
		}
		if !*asJSON {
			fmt.Fprintf(os.Stdout, "\n清单已写出：%s（%d 个工具）\n", *out, len(inv.Tools))
			fmt.Fprintf(os.Stdout, "下一步：ratchet policy draft --from %s --out policy.json\n", *out)
		}
	}
	return 0
}

// primaryHarness 取第一个被解析的 harness 作为清单里的 agent 名。
// 多 harness 环境下这是近似值，报告里会显示全部，用户可自行覆盖。
func primaryHarness(r discover.Report) string {
	for _, h := range r.Harnesses {
		if h.Parsed && len(h.Servers) > 0 {
			return h.ID
		}
	}
	return ""
}

func renderScan(w *os.File, r discover.Report, inv model.Inventory, failures map[string]string, introspected bool) {
	servers := 0
	for _, h := range r.Harnesses {
		servers += len(h.Servers)
	}
	parsed := 0
	for _, h := range r.Harnesses {
		if h.Parsed {
			parsed++
		}
	}
	fmt.Fprintf(w, "ratchet %s — 本机 agent 与 MCP server 扫描（只读，不执行任何东西）\n\n", version)
	fmt.Fprintf(w, "  HOME      %s\n", r.Home)
	fmt.Fprintf(w, "  harness   %d（已解析 %d）\n", len(r.Harnesses), parsed)
	fmt.Fprintf(w, "  server    %d\n", servers)
	if unpinned := r.Unpinned(); len(unpinned) > 0 {
		fmt.Fprintf(w, "  未锁版本  %d（同名包被替换时无法察觉）\n", len(unpinned))
	}

	if len(r.Harnesses) == 0 {
		fmt.Fprintln(w, "\n没有发现任何 agent 配置。")
		return
	}

	fmt.Fprintln(w, "\n按 harness")
	for _, h := range r.Harnesses {
		note := ""
		if !h.Parsed {
			note = "（配置格式不认识，这里可能还有我们看不到的 server）"
		}
		fmt.Fprintf(w, "  %-12s %-16s %d 个 server %s\n", h.ID, h.Name, len(h.Servers), note)
		for _, s := range h.Servers {
			fmt.Fprintf(w, "      %-18s %s %s\n", s.Name, serverDesc(s), riskMark(s))
		}
	}

	if len(r.Unparsed) > 0 {
		fmt.Fprintln(w, "\n未解析的配置")
		for _, u := range r.Unparsed {
			fmt.Fprintf(w, "  %s\n", u)
		}
	}

	if introspected {
		fmt.Fprintln(w, "\nintrospect 结果（已实际连接 server）")
		if len(inv.Tools) == 0 && len(failures) == 0 {
			fmt.Fprintln(w, "  没有可连接的 server")
		}
		if len(inv.Tools) > 0 {
			fmt.Fprintf(w, "  取到工具 %d 个\n", len(inv.Tools))
		}
		keys := make([]string, 0, len(failures))
		for k := range failures {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "  ⚠ %s 连接失败：%s\n", k, failures[k])
		}
		fmt.Fprintln(w, "  注意：连接失败的 server 不在清单里——它们的能力面是未知的，不要当成没有风险。")
	} else {
		fmt.Fprintln(w, "\n下一步")
		fmt.Fprintln(w, "  加 --introspect 连上这些 server 取工具名，才能编译出策略：")
		fmt.Fprintln(w, "    ratchet scan --introspect --out inventory.json")
	}
}

func serverDesc(s discover.Server) string {
	if s.URL != "" {
		return "remote " + s.URL
	}
	parts := append([]string{s.Command}, s.Args...)
	return strings.Join(parts, " ")
}

func riskMark(s discover.Server) string {
	if s.Risk == "" {
		return ""
	}
	return "⚠ " + s.Risk
}

func cmdPolicy(args []string) int {
	if len(args) == 0 || args[0] != "draft" {
		fmt.Fprintln(os.Stderr, "用法：ratchet policy draft --from <清单.json> [--out <策略.json>]")
		return 2
	}
	fs := flag.NewFlagSet("policy draft", flag.ContinueOnError)
	from := fs.String("from", "", "工具清单 JSON（必填）")
	out := fs.String("out", "", "策略输出路径（不给则只打印摘要）")
	agent := fs.String("agent", "", "覆盖清单里的 agent 名")
	strict := fs.Bool("strict-unknown", false, "无法判定能力的工具直接 deny")
	asJSON := fs.Bool("json", false, "把策略 JSON 打到 stdout")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *from == "" {
		fmt.Fprintln(os.Stderr, "缺少 --from：需要一份工具清单（见 examples/inventory.json）")
		return 2
	}

	inv, err := loadInventory(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取清单失败：%v\n", err)
		return 1
	}

	p := policy.Draft(inv, policy.Options{StrictUnknown: *strict, Agent: *agent})

	if *out != "" {
		if err := writeJSON(*out, p); err != nil {
			fmt.Fprintf(os.Stderr, "写入策略失败：%v\n", err)
			return 1
		}
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(p); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON 失败：%v\n", err)
			return 1
		}
		return 0
	}
	renderSummary(os.Stdout, *from, p, *out)
	return 0
}

func loadInventory(path string) (model.Inventory, error) {
	var inv model.Inventory
	raw, err := os.ReadFile(path)
	if err != nil {
		return inv, err
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return inv, fmt.Errorf("%s 不是合法 JSON：%w", path, err)
	}
	if inv.Format != "" && inv.Format != InventoryFormat {
		return inv, fmt.Errorf("不认识的清单格式 %q（期望 %s）", inv.Format, InventoryFormat)
	}
	if len(inv.Tools) == 0 {
		// 空清单会产出一份"全部拒绝"的策略——那不是编译结果，是误读。
		// 宁可报错，也不要给用户一份看起来正常、其实什么都没覆盖的策略。
		return inv, errors.New("清单里没有任何工具（tools 为空或缺失）")
	}
	return inv, nil
}

func writeJSON(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(buf, '\n'), 0o600)
}

// renderSummary 是给人看的那一屏。
//
// 判定的展示顺序按危险度从高到低——用户第一眼要看到的是"什么被拒了"，
// 而不是"什么被放行了"。
func renderSummary(w *os.File, from string, p model.Policy, out string) {
	servers, tools, allow, approve, deny := policy.Counts(p)
	fmt.Fprintf(w, "ratchet %s — 最小权限策略编译\n\n", version)
	fmt.Fprintf(w, "  输入      %s（agent: %s，%d 个工具）\n", from, orDash(p.Agent), tools)
	if out != "" {
		fmt.Fprintf(w, "  输出      %s\n", out)
	}
	fmt.Fprintf(w, "  默认决策  %s（未登记的一律拒绝）\n", p.DefaultDecision)
	fmt.Fprintf(w, "  server    %d\n", servers)
	fmt.Fprintf(w, "  工具      %d → allow %d · approve %d · deny %d\n", tools, allow, approve, deny)
	if n := len(p.NeedsReview); n > 0 {
		fmt.Fprintf(w, "  待确认    %d（无法判定能力，已置为 approve，请核对后再用）\n", n)
	}

	type row struct {
		rank int
		dec  model.Decision
		key  string
	}
	rows := make([]row, 0, len(p.Rationale))
	rank := map[model.Decision]int{model.Deny: 0, model.Approve: 1, model.Allow: 2}
	for key := range p.Rationale {
		dec := decisionOf(p, key)
		rows = append(rows, row{rank: rank[dec], dec: dec, key: key})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].key < rows[j].key
	})

	fmt.Fprintln(w, "\n判定明细")
	for _, r := range rows {
		fmt.Fprintf(w, "  %-8s %-34s %s\n",
			strings.ToUpper(string(r.dec)), r.key, p.Rationale[r.key])
	}

	fmt.Fprintln(w, "\n下一步")
	fmt.Fprintln(w, "  把这份策略交给使用方之前，先复核「待确认」那一批；")
	fmt.Fprintln(w, "  要更严可以用 --strict-unknown 重跑，未知工具会直接 deny。")
}

func decisionOf(p model.Policy, key string) model.Decision {
	// key 自带 server，所以要按 server 精确匹配——
	// 只按工具名匹配会在"两个 server 有同名工具"时给出错误结论。
	for server, rules := range p.Servers {
		for _, t := range rules.Allow {
			if model.Key(server, t) == key {
				return model.Allow
			}
		}
		for _, t := range rules.Approve {
			if model.Key(server, t) == key {
				return model.Approve
			}
		}
		for _, t := range rules.Deny {
			if model.Key(server, t) == key {
				return model.Deny
			}
		}
	}
	return model.Approve
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
