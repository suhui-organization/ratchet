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
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/suhui-organization/ratchet/internal/discover"
	"github.com/suhui-organization/ratchet/internal/hook"
	"github.com/suhui-organization/ratchet/internal/mcp"
	"github.com/suhui-organization/ratchet/internal/mcpserver"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/observe"
	"github.com/suhui-organization/ratchet/internal/policy"
	"github.com/suhui-organization/ratchet/internal/store"
)

const version = "0.9.0"

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
	case "observe":
		os.Exit(cmdObserve(os.Args[2:]))
	case "ingest":
		os.Exit(cmdIngest(os.Args[2:]))
	case "mcp":
		// 以 stdio MCP server 运行（供 MCP 客户端与官方 Registry 收录）
		if err := mcpserver.Serve(os.Stdin, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "ratchet mcp: %v\n", err)
			os.Exit(1)
		}
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
  ratchet observe --calls <调用记录.jsonl> [--inventory <清单.json>]
                  [--out <清单.json>] [--json]
  ratchet ingest [--hook auto|codex|claude-code|generic] [--store <路径>]
                 [--server <名>] [--agent <名>]
  ratchet mcp                                  # 以 stdio MCP server 运行
  ratchet policy draft --from <清单.json> [--out <策略.json>]
                       [--agent <名字>] [--strict-unknown] [--only-observed]
                       [--lang en-US|zh-CN] [--json]
  ratchet policy check --policy <策略.json>    # 从 stdin 读一次调用试跑

说明：
  scan          只读本机配置，列出装了哪些 agent、挂了哪些 MCP server。
                默认**不执行任何东西**；加 --introspect 才会连上 server 取工具名
                （连上就会执行配置里写的命令，所以必须显式开启）。

  policy draft  读工具清单，编译出最小权限策略；每条判定都带依据。
                未登记的工具一律拒绝；无法判定能力的默认置为 approve 并列入待确认。
                带路径参数的工具会额外挂上敏感路径黑名单（.env / .ssh / 凭据等）。

  policy check  拿一次**假设的调用**试跑：先看参数约束，再看三态。
                写完规则后"会不会挡掉正常调用"是唯一要紧的问题，试一次比读 JSON 可靠。
                stdin: {"server":"…","tool":"…","args":{"path":"/workspace/x"}}

  observe       把真实调用记录对到清单上：哪些在用、哪些从未被用过、
                哪些**不在清单里却被调用过**（清单不全或有人绕过配置）。

  ingest        从 stdin 读一条 agent 事件，追加进本地调用记录。默认 auto：
                按 payload 形状认出 Codex / Claude Code，认不出按 generic
                （{"server","tool"} 两个字段即可）。解析失败静默退出，**绝不阻塞 agent**。
                默认只记 server/tool，不记工具参数（那里常含密钥与文件内容）。

                Claude Code 的 MCP 工具名是 mcp__<server>__<tool>，会被拆成真实来源；
                PermissionDenied 事件记为 deny/blocked——那是 agent **想做但被拦下**的事。

  mcp           以 stdio MCP server 运行，暴露三个只读工具：
                ratchet_scan / ratchet_policy / ratchet_check。
                让 agent 自己能问"我现在能碰什么"；也是官方 Registry 收录的前提。

  --strict-unknown  无法判定能力的工具直接 deny（默认是 approve + 待确认）
  --only-observed   只授予被观测到调用过的工具（最小权限最严格的一档）
  --lang            产物语言（默认 en-US）；CLI 自身的终端输出暂为中文
  --json            把策略 JSON 打到 stdout（不给 --out 时也能用管道接）
`)
}

// cmdPolicyCheck 拿一次**假设的调用**试跑策略。
//
// 存在的意义：写完规则之后，"这会不会把我正常的工作流挡掉"是唯一要紧的问题。
// 拿真实调用试一次，比读一遍 JSON 可靠得多。
//
// stdin 形状：{"server":"filesystem","tool":"read_file","args":{"path":"/workspace/x"}}
func cmdPolicyCheck(args []string) int {
	fs := flag.NewFlagSet("policy check", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "策略文件（必填）")
	asJSON := fs.Bool("json", false, "输出机器可读结果")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *policyPath == "" {
		fmt.Fprintln(os.Stderr, "缺少 --policy：需要一份策略文件")
		return 2
	}
	raw, err := os.ReadFile(*policyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读策略失败：%v\n", err)
		return 1
	}
	var p model.Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		fmt.Fprintf(os.Stderr, "策略文件不是合法 JSON：%v\n", err)
		return 1
	}

	callRaw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil || len(callRaw) == 0 {
		fmt.Fprintln(os.Stderr, "从 stdin 读不到调用。形状：{\"server\":\"…\",\"tool\":\"…\",\"args\":{…}}")
		return 2
	}
	var call struct {
		Server string            `json:"server"`
		Tool   string            `json:"tool"`
		Args   map[string]string `json:"args"`
	}
	if err := json.Unmarshal(callRaw, &call); err != nil {
		fmt.Fprintf(os.Stderr, "调用不是合法 JSON：%v\n", err)
		return 2
	}
	if call.Server == "" || call.Tool == "" {
		fmt.Fprintln(os.Stderr, "调用缺少 server 或 tool")
		return 2
	}

	three := policy.Verdict(p, call.Server, call.Tool)
	decision, violations := policy.Evaluate(p, call.Server, call.Tool, call.Args)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{
			"call":       call,
			"verdict":    three,
			"decision":   decision,
			"violations": violations,
		})
		return 0
	}

	fmt.Printf("ratchet %s — 单次调用试跑\n\n", version)
	fmt.Printf("  调用        %s/%s\n", call.Server, call.Tool)
	fmt.Printf("  三态判定    %s\n", three)
	if len(violations) == 0 {
		fmt.Printf("  参数约束    无命中\n")
	} else {
		for _, v := range violations {
			fmt.Printf("  参数约束    %s = %q → %s（规则 %s）\n", v.ArgKey, v.Value, v.Reason, v.Rule)
		}
	}
	fmt.Printf("  最终结论    %s\n", decision)
	if decision != three {
		fmt.Printf("              ↑ 参数约束把它从 %s 收成了 %s\n", three, decision)
	}
	return 0
}

func cmdObserve(args []string) int {
	fs := flag.NewFlagSet("observe", flag.ContinueOnError)
	calls := fs.String("calls", "", "调用记录 JSONL（必填；每行 {server, tool, ...}）")
	invPath := fs.String("inventory", "", "工具清单（给了就做能力面 vs 使用面的比对）")
	out := fs.String("out", "", "把调用次数写回清单并输出到该文件")
	asJSON := fs.Bool("json", false, "输出 JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *calls == "" {
		fmt.Fprintln(os.Stderr, "缺少 --calls：需要一份调用记录（JSONL，每行一次调用）")
		return 2
	}

	summary, err := observe.ReadCallsFile(*calls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取调用记录失败：%v\n", err)
		return 1
	}
	if err := observe.Validate(summary); err != nil {
		fmt.Fprintf(os.Stderr, "调用记录不可用：%v\n", err)
		return 1
	}

	var inv model.Inventory
	if *invPath != "" {
		inv, err = loadInventory(*invPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取清单失败：%v\n", err)
			return 1
		}
	}

	withCalls := observe.Apply(inv, summary)
	var cmp *observe.Comparison
	if *invPath != "" {
		c := observe.Compare(inv, summary)
		cmp = &c
	}

	if *out != "" {
		if err := writeJSON(*out, withCalls); err != nil {
			fmt.Fprintf(os.Stderr, "写出清单失败：%v\n", err)
			return 1
		}
	}

	if *asJSON {
		payload := map[string]any{"summary": summary}
		if cmp != nil {
			payload["comparison"] = cmp
		}
		if *out != "" {
			payload["inventory"] = withCalls
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON 失败：%v\n", err)
			return 1
		}
		return 0
	}
	renderObserve(os.Stdout, summary, cmp, *out, len(withCalls.Tools))
	return 0
}

// cmdIngest 从 stdin 读一条 agent 事件，追加进本地调用记录。
//
// **绝不阻塞 agent**：这是挂在一个交互式工具调用链上的 hook，
// 它出错、超时、或者 pod 二进制不在——都不该让用户的工作流卡住。
// 所以任何异常都安静地退出 0。
func cmdIngest(args []string) int {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	hookName := fs.String("hook", "auto", "事件来源：auto / codex / claude-code / generic")
	storePath := fs.String("store", "", "调用记录文件路径（默认 $RATCHET_HOME/calls.jsonl）")
	server := fs.String("server", "", "覆盖 server 名（默认 codex-tools）")
	agent := fs.String("agent", "", "覆盖 agent 名（默认 codex）")
	if err := fs.Parse(args); err != nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return 0
	}
	call, ok := hook.Translate(raw, hook.ParseHarness(*hookName), *server, *agent)
	if !ok {
		return 0
	}
	if err := store.Append(*storePath, call); err != nil {
		// 写不进去也不能打断 agent；但要让用户有机会发现，
		// 所以往 stderr 说一句（hook 的 stderr 不进 agent 上下文）。
		fmt.Fprintf(os.Stderr, "ratchet ingest: 写入调用记录失败：%v\n", err)
		return 0
	}
	return 0
}

func renderObserve(w *os.File, s observe.Summary, cmp *observe.Comparison, out string, tools int) {
	fmt.Fprintf(w, "ratchet %s — 观测到的调用\n\n", version)
	fmt.Fprintf(w, "  调用记录  %d 条\n", s.Total)
	if s.Skipped > 0 {
		fmt.Fprintf(w, "  跳过      %d 行（无法解析或缺字段）\n", s.Skipped)
	}
	fmt.Fprintf(w, "  涉及工具  %d 个\n", len(s.Counts))

	if cmp == nil {
		if out != "" {
			fmt.Fprintf(w, "\n清单已写出：%s（%d 个工具）\n", out, tools)
		}
		fmt.Fprintln(w, "\n下一步：加 --inventory <清单.json> 就能看出哪些工具从未被用过。")
		return
	}

	fmt.Fprintf(w, "\n  清单里      %d 个工具\n", len(cmp.Called)+len(cmp.Unused))
	fmt.Fprintf(w, "  被调用过    %d\n", len(cmp.Called))
	fmt.Fprintf(w, "  从未被调用  %d  ← 最小权限下这些应该被收掉\n", len(cmp.Unused))
	if len(cmp.Unknown) > 0 {
		fmt.Fprintf(w, "  清单之外    %d  ⚠ 被调用过但不在清单里：要么扫描漏了，要么有人绕过了配置\n", len(cmp.Unknown))
	}

	if len(cmp.Unused) > 0 {
		fmt.Fprintln(w, "\n从未被调用的工具（按最小权限应当移除）")
		for _, key := range cmp.Unused {
			fmt.Fprintf(w, "  %s\n", key)
		}
	}
	if len(cmp.Unknown) > 0 {
		fmt.Fprintln(w, "\n清单之外的调用（先查清来源，再决定是补清单还是堵绕过）")
		for _, key := range cmp.Unknown {
			fmt.Fprintf(w, "  %-34s %d 次\n", key, cmp.Counts[key])
		}
	}
	if out != "" {
		fmt.Fprintf(w, "\n带调用次数的清单已写出：%s\n", out)
	}
	fmt.Fprintln(w, "\n下一步（最严格的一档：只授予观测到的东西）")
	if out == "" {
		out = "<清单.json>"
	}
	fmt.Fprintf(w, "  ratchet policy draft --from %s --only-observed --out policy.json\n", out)
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
	if len(args) > 0 && args[0] == "check" {
		return cmdPolicyCheck(args[1:])
	}
	if len(args) == 0 || args[0] != "draft" {
		fmt.Fprintln(os.Stderr, "用法：ratchet policy draft --from <清单.json> [--out <策略.json>]")
		fmt.Fprintln(os.Stderr, "     ratchet policy check --policy <策略.json>   # 从 stdin 读一次调用试跑")
		return 2
	}
	fs := flag.NewFlagSet("policy draft", flag.ContinueOnError)
	from := fs.String("from", "", "工具清单 JSON（必填）")
	out := fs.String("out", "", "策略输出路径（不给则只打印摘要）")
	agent := fs.String("agent", "", "覆盖清单里的 agent 名")
	strict := fs.Bool("strict-unknown", false, "无法判定能力的工具直接 deny")
	onlyObserved := fs.Bool("only-observed", false, "只授予被观测到调用过的工具")
	lang := fs.String("lang", "", "产物语言：en-US（默认）或 zh-CN；也可用 RATCHET_LANG")
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

	loc := policy.ParseLocale(*lang)
	if *lang == "" {
		loc = policy.ParseLocale(os.Getenv("RATCHET_LANG"))
	}
	p := policy.Draft(inv, policy.Options{
		StrictUnknown: *strict,
		OnlyObserved:  *onlyObserved,
		Agent:         *agent,
		Locale:        loc,
	})

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
