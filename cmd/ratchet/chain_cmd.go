package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/suhui-organization/ratchet/internal/chain"
	"github.com/suhui-organization/ratchet/internal/observe"
	"github.com/suhui-organization/ratchet/internal/store"
)

// cmdChain 把调用记录变成可验证的事件流（ASAS-5.3 / 6.6）。
//
// 这里**不验链**：验证是收货方的事，实现在 Python 的 asas.rule_silence_auditable
// 里。造链和验链各写一份，等于给"链对不对"准备两个答案——出问题时没人知道信哪个。
func cmdChain(args []string) int {
	fs := flag.NewFlagSet("chain", flag.ContinueOnError)
	calls := fs.String("calls", "", "调用记录 JSONL（默认 ~/.ratchet/calls.jsonl）")
	out := fs.String("out", "", "把事件流写到这个文件（默认只打印摘要）")
	agent := fs.String("agent", "", "调用记录里缺 agent 时用的兜底名字")
	asJSON := fs.Bool("json", false, "以 JSON 打印批次摘要")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := *calls
	if path == "" {
		path = store.CallsPath()
	}
	callsList, err := readCalls(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读调用记录失败：%v\n", err)
		return 1
	}
	if len(callsList) == 0 {
		// "没有事件"本身是结论，但要和"没做成"区分开：所以明说文件里是空的。
		fmt.Printf("没有可造链的调用记录：%s（文件为空或不存在）\n", path)
		return 0
	}

	events, err := chain.Build(callsList, *agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	head, count := chain.Head(events)
	if *out != "" {
		if err := chain.WriteJSONL(*out, events); err != nil {
			fmt.Fprintf(os.Stderr, "写事件流失败：%v\n", err)
			return 1
		}
	}

	if *asJSON {
		payload := map[string]any{
			"source": path, "out": *out, "events": count, "head": head,
			"tools": chain.SortedTools(events),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}
	fmt.Printf("事件流：%d 条 · 摘要 %s\n", count, head)
	if tools := chain.SortedTools(events); len(tools) > 0 {
		fmt.Printf("  覆盖工具 %d 个：%s\n", len(tools), strings.Join(tools, "，"))
	}
	if *out != "" {
		fmt.Printf("  已写出：%s\n", *out)
	}
	fmt.Println("  验收：把这份文件交给验证方（或控制面），序号断裂与哈希不符都会被指出。")
	return 0
}

func readCalls(path string) ([]observe.Call, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	calls, _, err := observe.ReadCallListFile(path)
	return calls, err
}
