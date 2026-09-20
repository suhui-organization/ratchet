// Package deliver 把一次交付的四步串成一条命令。
//
// 为什么值得单独做：一次交付现在是
//
//	scan → observe → policy draft → ratchet-report build
//
// 四条命令、两个语言、中间靠手工传路径。**利润率就在这四步之间**——
// 同样的 30 分钟，串起来能交付 5 份，不串只能交付 1 份。
//
// 编排放在 Go（引擎侧），报告步骤用子进程调 Python 工具：
// 报告渲染只有一份实现在 Python，这里绝不重写一遍。
package deliver

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/suhui-organization/ratchet/internal/chain"
	"github.com/suhui-organization/ratchet/internal/discover"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/observe"
	"github.com/suhui-organization/ratchet/internal/policy"
)

// shortHash 只用于打印：完整 64 位十六进制在终端里没人读，前 12 位足够对上号。
func shortHash(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12] + "…"
}

// Options 是一次交付的全部输入。
type Options struct {
	Home         string
	Workdir      string
	Calls        string // 调用记录；为空或文件不存在则跳过观测这一步
	OutDir       string
	Agent        string
	Lang         string
	OnlyObserved bool
	Client       string // 写进报告封面的客户名
	// ReportCmd 是报告生成器。默认就是文档里那条命令。
	ReportCmd []string
	// Exemptions 是 "name=YYYY-MM-DD" 形式的书面豁免（ASAS-3.1）：
	// 明知未定版而接受，必须写进凭据让收货方看得见，而不是被悄悄放过。
	Exemptions []string
}

// Result 是每一步的产物路径，打印给使用者。
type Result struct {
	Steps    []Step
	Policy   string
	Delivery string
	Bundle   string
}

type Step struct {
	Name string
	OK   bool
	Note string
}

// Run 依次执行四步。任何一步失败都立即停下并报出是哪一步——
// 流水线最糟的行为是"跑完了但结果是错的"。
func Run(opts Options) (Result, error) {
	var res Result
	if opts.OutDir == "" {
		return res, fmt.Errorf("需要 --out（交付目录）")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return res, err
	}

	// ① 扫描（只读）
	report := discover.Scan(opts.Home, opts.Workdir)
	servers := 0
	for _, h := range report.Harnesses {
		servers += len(h.Servers)
	}
	res.Steps = append(res.Steps, Step{
		Name: "scan", OK: true,
		Note: fmt.Sprintf("%d harnesses, %d servers, %d unpinned",
			len(report.Harnesses), servers, len(report.Unpinned())),
	})

	// 清单：静态扫描给不出工具名，所以这里只把 server 当作"能力面"的近似。
	// 要真实工具名，先用 scan --introspect 产出清单，再用 --inventory 传进来。
	inv := inventoryFrom(report, opts.Agent)
	// server 级事实落盘：ASAS-A 凭据要用它把"定版状态/版本"从 unknown 变成已知。
	if err := writeJSON(filepath.Join(opts.OutDir, "serverfacts.json"), inv.Servers); err != nil {
		return res, err
	}

	// ② 观测（有语料才做）
	if opts.Calls != "" {
		if _, err := os.Stat(opts.Calls); err == nil {
			summary, err := observe.ReadCallsFile(opts.Calls)
			if err != nil {
				return res, fmt.Errorf("读调用记录失败：%w", err)
			}
			inv = observe.Apply(inv, summary)
			cmp := observe.Compare(inv, summary)
			res.Steps = append(res.Steps, Step{
				Name: "observe", OK: true,
				Note: fmt.Sprintf("%d calls, %d used, %d never used, %d outside the inventory",
					summary.Total, len(cmp.Called), len(cmp.Unused), len(cmp.Unknown)),
			})
			// ②b 事件流：把调用记录变成带哈希链的事件流（ASAS-5.3/6.6）。
			// 没有这一步，凭据里的 "silenceIsAuditable" 永远是"未评估"——
			// 而这一条恰恰是"监控数据可被删改而不被发现"的唯一防线。
			eventDir := filepath.Join(opts.OutDir, "delivery")
			if err := os.MkdirAll(eventDir, 0o755); err != nil {
				return res, err
			}
			calls, skipped, err := observe.ReadCallListFile(opts.Calls)
			if err != nil {
				return res, fmt.Errorf("读调用记录失败：%w", err)
			}
			events, err := chain.Build(calls, opts.Agent)
			if err != nil {
				return res, err
			}
			eventsPath := filepath.Join(eventDir, "events.jsonl")
			if err := chain.WriteJSONL(eventsPath, events); err != nil {
				return res, err
			}
			head, count := chain.Head(events)
			note := fmt.Sprintf("%d events · head %s", count, shortHash(head))
			if skipped > 0 {
				note += fmt.Sprintf(" · %d bad lines skipped（坏行会让断流看起来正常，请查）", skipped)
			}
			res.Steps = append(res.Steps, Step{Name: "chain", OK: skipped == 0, Note: note})
		} else {
			res.Steps = append(res.Steps, Step{
				Name: "observe", OK: false,
				Note: "no call records yet — run your agents with the hook attached",
			})
		}
	}

	// ③ 编译策略
	p := policy.Draft(inv, policy.Options{
		OnlyObserved: opts.OnlyObserved,
		Agent:        opts.Agent,
		Locale:       policy.ParseLocale(opts.Lang),
	})
	policyPath := filepath.Join(opts.OutDir, "policy.json")
	if err := writeJSON(policyPath, p); err != nil {
		return res, err
	}
	res.Policy = policyPath
	_, tools, allow, approve, deny := policy.Counts(p)
	res.Steps = append(res.Steps, Step{
		Name: "policy", OK: true,
		Note: fmt.Sprintf("%d tools: %d allow, %d approve, %d deny", tools, allow, approve, deny),
	})

	// ④ 报告（Python 工具；缺了就明说，不假装跑过）
	delivery := filepath.Join(opts.OutDir, "delivery")
	bundle := filepath.Join(opts.OutDir, "share.json")
	args := append([]string{}, opts.ReportCmd...)
	args = append(args, "build", "--dir", delivery, "--policy", policyPath, "--bundle", bundle)
	if opts.Lang != "" {
		args = append(args, "--lang", opts.Lang)
	}
	if opts.Client != "" {
		args = append(args, "--note", "client: "+opts.Client)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		res.Steps = append(res.Steps, Step{
			Name: "report", OK: false,
			// 这条提示必须**可执行**。原先是把失败的命令原样打回去让人照抄，
			// 但那条命令只有在 ratchet-report 已安装时才跑得通——缺工具的人照着抄
			// 只会再撞一次墙（实测：直接执行 cli.py 会因相对导入报 ImportError）。
			// 所以这里改成告诉他"缺什么、怎么装、或者怎么自己指定"。
			Note: fmt.Sprintf("report step skipped: %q is not runnable (%v). policy.json and inventory are still valid. "+
				"To generate the report: install the Python side (in a checkout: `pip install -e service`) "+
				"or pass your own command with --report-cmd.",
				strings.Join(opts.ReportCmd, " "), err),
		})
		res.Delivery = ""
		return res, nil // 前三步的产物有效，不当作整体失败
	}
	res.Steps = append(res.Steps, Step{Name: "report", OK: true, Note: delivery})
	res.Delivery, res.Bundle = delivery, bundle

	// ⑤ ASAS-A 凭据：产出后**当场用同一套规则自查**。
	// 自查不过就不算交付（见 docs/DEVELOPMENT-PLAN.md 的 P0/T2）——自己产的凭据先过
	// 自己的校验，否则收货方拿到的就是一份连签发方都没验过的东西。
	// 判定逻辑不在这里重复实现：Go 负责产出，Python 负责验证，规则只有一份。
	org := opts.Client
	if org == "" {
		org = "local"
	}
	owner := os.Getenv("USER")
	if owner == "" {
		owner = "unassigned"
	}
	attArgs := append([]string{}, opts.ReportCmd...)
	attArgs = append(attArgs, "asas",
		"--policy", policyPath, "--dir", delivery, "--org", org, "--owner", owner)
	for _, item := range opts.Exemptions {
		attArgs = append(attArgs, "--exempt", item)
	}
	if inv := filepath.Join(opts.OutDir, "serverfacts.json"); fileExists(inv) {
		attArgs = append(attArgs, "--inventory", inv)
	}
	attCmd := exec.Command(attArgs[0], attArgs[1:]...)
	attCmd.Stdout, attCmd.Stderr = os.Stdout, os.Stderr
	if err := attCmd.Run(); err != nil {
		res.Steps = append(res.Steps, Step{
			Name: "attest", OK: false,
			Note: "ASAS-A 凭据未通过自查，交付不算完成（见上面的规则输出）",
		})
		return res, nil
	}
	res.Steps = append(res.Steps, Step{
		Name: "attest", OK: true,
		Note: filepath.Join(delivery, "attestation.json") + "（已通过 ASAS-V 自查）",
	})
	return res, nil
}

// fileExists 只用于"可选输入是否存在"这类判断，不吞掉真实错误。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// inventoryFrom 把扫描结果降级成清单。
//
// 静态扫描只知道 server、不知道工具名，所以这里每个 server 记一条占位观测：
// 它让策略至少能覆盖到"这个 server 存在"，而工具级判定要靠 --introspect。
func inventoryFrom(rep discover.Report, agent string) model.Inventory {
	inv := model.Inventory{Format: "ratchet-inventory/v1", Agent: agent}
	for _, h := range rep.Harnesses {
		for _, s := range h.Servers {
			inv.Tools = append(inv.Tools, model.ToolObservation{Server: s.Name, Tool: s.Name})
			inv.Servers = append(inv.Servers, model.ServerFact{
				Name: s.Name, Command: s.Command, Source: s.Source,
				Ref: s.Ref, Pinned: s.Pinned, Risk: s.Risk,
			})
		}
	}
	return inv
}

func writeJSON(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(buf, '\n'), 0o600)
}
