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

	"github.com/suhui-organization/ratchet/internal/discover"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/observe"
	"github.com/suhui-organization/ratchet/internal/policy"
)

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
			Note: fmt.Sprintf("could not run %q — policy and inventory are still here; run the report step yourself",
				strings.Join(args, " ")),
		})
		res.Delivery = ""
		return res, nil // 前三步的产物有效，不当作整体失败
	}
	res.Steps = append(res.Steps, Step{Name: "report", OK: true, Note: delivery})
	res.Delivery, res.Bundle = delivery, bundle
	return res, nil
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
