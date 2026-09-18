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
  ratchet policy draft --from <清单.json> [--out <策略.json>]
                       [--agent <名字>] [--strict-unknown] [--json]

说明：
  policy draft  读工具清单，编译出最小权限策略；每条判定都带依据。
                未登记的工具一律拒绝；无法判定能力的默认置为 approve 并列入待确认。

  --strict-unknown  无法判定能力的工具直接 deny（默认是 approve + 待确认）
  --json            把策略 JSON 打到 stdout（不给 --out 时也能用管道接）
`)
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
