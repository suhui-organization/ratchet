// Package observe 把"实际发生过的调用"与"配置里存在的能力"对上。
//
// 这是最小权限的落点：**只授予观测到的东西**。
// 静态扫描告诉你 agent *能*做什么；调用记录告诉你它*实际*做了什么。
// 两者的差集才是可以安全收掉的部分。
//
// 输入是 JSONL，一行一次调用。只有 server 与 tool 是必需的——
// 契约越窄，越容易从各种来源（hook、网关日志、厂商导出）喂进来。
package observe

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/suhui-organization/ratchet/internal/model"
)

// Call 是一次工具调用。
type Call struct {
	TS       string `json:"ts,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Server   string `json:"server"`
	Tool     string `json:"tool"`
	Decision string `json:"decision,omitempty"`
	Outcome  string `json:"outcome,omitempty"`
}

// Summary 是观测结果。
type Summary struct {
	// Counts 的 key 是 "server/tool"。
	Counts map[string]int `json:"counts"`
	// Total 是读进来的调用条数（含无法解析而被跳过的？不含——跳过的单独计数）。
	Total int `json:"total"`
	// Skipped 是无法解析的行数。坏行要报出来，不能静默当成"没调用"。
	Skipped int `json:"skipped"`
}

// ReadCalls 读一份 JSONL 调用记录。
//
// 空行忽略；坏行计入 Skipped 而不是中断——真实的日志里总有半行。
// 但坏行数会被报出来：静默吞掉等于伪造"没有调用"。
func ReadCalls(r io.Reader) (Summary, error) {
	summary := Summary{Counts: map[string]int{}}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var call Call
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			summary.Skipped++
			continue
		}
		if call.Server == "" || call.Tool == "" {
			summary.Skipped++
			continue
		}
		summary.Total++
		summary.Counts[model.Key(call.Server, call.Tool)]++
	}
	if err := scanner.Err(); err != nil {
		return summary, err
	}
	return summary, nil
}

// ReadCallsFile 读文件形式的调用记录。
func ReadCallsFile(path string) (Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return Summary{}, err
	}
	defer f.Close()
	return ReadCalls(f)
}

// Comparison 是"能力面 vs 使用面"的比对结果。
type Comparison struct {
	// Called 是既在清单里、也被调用过的工具。
	Called []string `json:"called"`
	// Unused 在清单里、但从未被调用过——最小权限下这些应该被收掉。
	Unused []string `json:"unused"`
	// Unknown 被调用过、但不在清单里。**这是最重要的信号**：
	// 要么清单不全（扫描漏了），要么有人绕过了配置。
	Unknown []string `json:"unknown"`
	// Counts 是各工具的调用次数。
	Counts map[string]int `json:"counts"`
}

// Compare 把观测结果对到清单上。
func Compare(inv model.Inventory, summary Summary) Comparison {
	result := Comparison{Counts: summary.Counts}
	inInventory := map[string]bool{}
	for _, t := range inv.Tools {
		key := model.Key(t.Server, t.Tool)
		inInventory[key] = true
		if summary.Counts[key] > 0 {
			result.Called = append(result.Called, key)
		} else {
			result.Unused = append(result.Unused, key)
		}
	}
	for key := range summary.Counts {
		if !inInventory[key] {
			result.Unknown = append(result.Unknown, key)
		}
	}
	sort.Strings(result.Called)
	sort.Strings(result.Unused)
	sort.Strings(result.Unknown)
	return result
}

// Apply 把调用次数写进清单，供 policy draft 使用。
func Apply(inv model.Inventory, summary Summary) model.Inventory {
	out := model.Inventory{Format: inv.Format, Agent: inv.Agent, Tools: make([]model.ToolObservation, 0, len(inv.Tools))}
	for _, t := range inv.Tools {
		t.Calls = summary.Counts[model.Key(t.Server, t.Tool)]
		out.Tools = append(out.Tools, t)
	}
	return out
}

// Validate 给出契约层面的检查，避免把"读错文件"当成"没有调用"。
func Validate(summary Summary) error {
	if summary.Total == 0 && summary.Skipped == 0 {
		return errors.New("调用记录是空的（既没有有效行，也没有坏行）——确认文件对不对")
	}
	if summary.Total == 0 {
		return errors.New("所有行都无法解析：确认是第一行就是 JSON，且每行有 server 与 tool")
	}
	return nil
}
