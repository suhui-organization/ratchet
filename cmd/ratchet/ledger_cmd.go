package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/suhui-organization/ratchet/internal/ledger"
)

// 台账相关的两条命令：问"谁能碰 X"、以及动手拿下某个人。
//
// 为什么这两条要放在 CLI 里而不是只留 HTTP 接口：事故现场的手边工具就是这个二进制。
// 要求人现场拼 curl 和 jq，等于要求他在最紧张的时候出错。

func ledgerClient(api string) *ledger.Client {
	if api == "" {
		api = os.Getenv("ASAS_API")
	}
	if api == "" {
		fmt.Fprint(os.Stderr, `缺少控制面地址：--api <url> 或环境变量 ASAS_API
例如：ratchet reach --api http://asas-api.asas.svc:8080 --subject filesystem
`)
		os.Exit(2)
	}
	return ledger.New(api)
}

// cmdReach 回答 ASAS-8.3：哪些 agent 曾能触达 X。
func cmdReach(args []string) int {
	fs := flag.NewFlagSet("reach", flag.ContinueOnError)
	api := fs.String("api", "", "控制面地址（也可用 ASAS_API）")
	subject := fs.String("subject", "", "要问的资产名：server 或 tool（必填）")
	asJSON := fs.Bool("json", false, "以 JSON 输出（给自动化用）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "缺少 --subject：要问的是哪个资产（server 或 tool）")
		return 2
	}

	answer, err := ledgerClient(*api).Reach(*subject)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(answer)
		return 0
	}

	fmt.Printf("谁曾能触达 %s\n\n", answer.Subject)
	if len(answer.Granted) == 0 {
		fmt.Println("  能（授权面）：无记录")
	} else {
		fmt.Printf("  能（授权面）%d 条：\n", len(answer.Granted))
		for _, g := range answer.Granted {
			mark := "  "
			if g.Contained {
				mark = "🚫"
			}
			// basis 与凭据 id 都要打出来：没有依据的结论在应急时等于没有结论。
			fmt.Printf("    %s %-24s %-7s %s\n", mark, g.Agent, g.Verdict, g.Basis)
			fmt.Printf("       └ 依据：%s\n", g.AttestationID)
			if len(g.Chain) > 0 {
				parts := make([]string, 0, len(g.Chain))
				for _, link := range g.Chain {
					parts = append(parts, link.Parent+" → "+link.Child)
				}
				fmt.Printf("       └ 委派链：%s\n", strings.Join(parts, "，"))
			}
		}
	}
	if len(answer.Denied) > 0 {
		fmt.Printf("  明确拒绝 %d 条：\n", len(answer.Denied))
		for _, d := range answer.Denied {
			fmt.Printf("      %-24s %s\n", d.Agent, d.Basis)
		}
	}
	if len(answer.Observed) > 0 {
		fmt.Printf("  确实用过（观测面）：\n")
		for _, o := range answer.Observed {
			fmt.Printf("      %-24s %d 次\n", o.Tool, o.Count)
		}
	} else {
		fmt.Println("  确实用过（观测面）：无记录")
	}
	if len(answer.Contained) > 0 {
		fmt.Printf("  已遏制：%s\n", strings.Join(answer.Contained, "，"))
	}
	// 这一行必须显眼：空列表和"答不了"是两件事，混了会直接误导事故处理。
	for _, note := range answer.Unanswerable {
		fmt.Printf("\n  ⚠️  %s\n", note)
	}
	return 0
}

// cmdContain 吊销一个 agent（默认级联到它的下级，ASAS-8.5）。
func cmdContain(args []string) int {
	fs := flag.NewFlagSet("contain", flag.ContinueOnError)
	api := fs.String("api", "", "控制面地址（也可用 ASAS_API）")
	agent := fs.String("agent", "", "要吊销的 agent id（必填）")
	reason := fs.String("reason", "", "写进留痕的原因")
	dryRun := fs.Bool("dry-run", false, "只看会连累谁，不动手")
	yes := fs.Bool("yes", false, "确认执行（不写就只预告，不动手）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *agent == "" {
		fmt.Fprintln(os.Stderr, "缺少 --agent：要吊销哪个 agent")
		return 2
	}

	client := ledgerClient(*api)
	// 默认**不**动手：吊销是不可逆的动作（本文具只会追加记录），
	// 所以先让它把"会连累谁"打出来，人看一眼再确认。--dry-run 可以显式预演，
	// --yes 才是真正的扳机。
	preview, err := client.Contain(*agent, *reason, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	fmt.Printf("吊销 %s 会连累：\n", preview.Agent)
	fmt.Printf("  %s（本人）\n", preview.Agent)
	for _, c := range preview.Cascaded {
		fmt.Printf("  %s（第 %d 层，来自 %s）\n", c.Agent, c.Depth, c.Via)
	}
	if *dryRun || !*yes {
		if !*dryRun {
			fmt.Printf("\n这是预告，**没有执行**。确认要动手：\n  ratchet contain --agent %s --reason %q --yes\n",
				*agent, *reason)
		}
		return 0
	}

	result, err := client.Contain(*agent, *reason, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	fmt.Printf("\n✅ 已吊销 %d 个：%s\n", len(result.Contained), strings.Join(result.Contained, "，"))
	fmt.Printf("   留痕时间：%s\n", result.RecordedAt)
	fmt.Println("   凭据不会因此变成合规——遏制记录是**另一个维度**，验证时会两者一起看（V9）。")
	return 0
}
