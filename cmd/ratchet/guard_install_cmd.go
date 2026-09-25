package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/suhui-organization/ratchet/internal/hook"
)

// snippetShape 是配置文件的形状。同一样东西（"在这个事件上跑这条命令"）
// 在九家 agent 里有五种写法，抄错一种就是静默不生效。
type snippetShape int

const (
	// shapeHooksWrapped：{"hooks":{"PreToolUse":[{"matcher":…,"hooks":[{…}]}]}}
	// Claude Code / Codex / CodeBuddy / Qwen Code / Qoder / Gemini CLI
	shapeHooksWrapped snippetShape = iota
	// shapeNamed：{"<组名>":{"enabled":true,"PreToolUse":[…]}}（反重力）
	shapeNamed
	// shapeUnwrapped：{"PreToolUse":[{"matcher":…,"hooks":[…]}]}（Factory Droid）
	//
	// 官方文档：hooks.json 直接用事件名做键；只有写进 settings.json 时才套 hooks 外层。
	shapeUnwrapped
	// shapeCrushFlat：{"hooks":{"PreToolUse":[{"command":…,"matcher":…,"timeout":10}]}}
	//
	// Crush 的条目是**扁平的**，没有里面那层 {"hooks":[…]}。
	shapeCrushFlat
	// shapeCursor：{"version":1,"hooks":{"preToolUse":[{"command":…,"matcher":…}]}}
	//
	// Cursor 多一个 version 字段，事件名是小驼峰，条目同样扁平。
	shapeCursor
	// shapeFileDrop：不写配置，往约定目录丢一个可执行文件（Cline）
	shapeFileDrop
	// shapePlugin：写一个插件文件（OpenCode）
	shapePlugin
)

// installTarget 是一家 agent 的挂载位置与注意事项。
type installTarget struct {
	harness hook.Harness
	// label 是给人看的名字
	label string
	// file 是配置文件相对家目录的路径（fileDrop/plugin 形状下是脚本位置）
	file string
	// event 是"工具执行前"在这家的叫法
	event string
	// trustNote 是"装完之后还差哪一步"——这一栏写错，用户会以为装上了其实没生效
	trustNote string
	// verified 为 false 表示配置路径没有一手文档佐证，只打印不代写
	verified bool
	// shape 是配置文件的形状
	shape snippetShape
}

// hookSetName 是 named 形状下使用的钩子组名。
const hookSetName = "ratchet-guard"

// installTargets 是已知的挂载点。
//
// 每一条 verified=true 的都有官方文档佐证；没有佐证的宁可标 false 只打印不代写——
// 往客户机器上写一个错误路径，比不写更糟：他会以为装上了。
func installTargets() []installTarget {
	return []installTarget{
		{
			harness: hook.HarnessClaude, label: "Claude Code", file: ".claude/settings.json",
			event: "PreToolUse", verified: true,
			trustNote: "Claude Code 会拦住未信任目录的钩子；出现工作区信任对话框时选信任即可。",
		},
		{
			harness: hook.HarnessCodex, label: "Codex CLI", file: ".codex/hooks.json",
			event: "PreToolUse", verified: true,
			trustNote: "**必须**在 Codex 里跑一次 /hooks，审核并信任这条钩子，否则它不会运行。" +
				"信任是按钩子内容哈希记录的——以后改了命令，要重新信任一次。",
		},
		{
			harness: hook.HarnessCodeBuddy, label: "CodeBuddy（腾讯云）", file: ".codebuddy/settings.json",
			event: "PreToolUse", verified: true,
			trustNote: "CodeBuddy 不热加载配置：改完要重启会话，并在 /hooks 面板里审核一次。",
		},
		{
			harness: hook.HarnessQwen, label: "Qwen Code（阿里）", file: ".qwen/settings.json",
			event: "PreToolUse", verified: true,
			trustNote: "项目级钩子需要该目录处于受信任状态。",
		},
		{
			harness: hook.HarnessQoder, label: "Qoder / 通义灵码（阿里）", file: ".qoder/settings.json",
			event: "PreToolUse", verified: true,
			// 官方原文："Edit the config file and they take effect immediately."
			trustNote: "配置改动立即生效，无需额外信任步骤；重启会话即可。",
		},
		{
			harness: hook.HarnessGemini, label: "Gemini CLI（Google）", file: ".gemini/settings.json",
			event: "BeforeTool", verified: true,
			trustNote: "注意：Gemini CLI 的个人免费档已于 2026-06-18 迁到 Antigravity CLI；" +
				"企业档不受影响。用 Antigravity 的话改用 --harness antigravity。",
		},
		{
			harness: hook.HarnessAntigravity, label: "Antigravity CLI（Google）", file: ".gemini/antigravity-cli/hooks.json",
			event: "PreToolUse", verified: true, shape: shapeNamed,
			trustNote: "反重力的配置形状和别家不同（按钩子组名分组，见下）。" +
				"项目级配置放在仓库根的 .agents/hooks.json。",
		},
		{
			harness: hook.HarnessCursor, label: "Cursor", file: ".cursor/hooks.json",
			event: "preToolUse", verified: true, shape: shapeCursor,
			trustNote: "Cursor 上 ask 在 preToolUse 里**不被强制执行**，所以" +
				"需要人工确认的调用会退回它自己的权限流程；要强制确认只能挂 beforeShellExecution" +
				"（但那覆盖不到文件读写）。项目级放 .cursor/hooks.json，用户级是同一个相对路径。",
		},
		{
			harness: hook.HarnessCrush, label: "Crush（Charm）", file: ".config/crush/crush.json",
			event: "PreToolUse", verified: true, shape: shapeCrushFlat,
			trustNote: "Crush 的 decision 只有 allow / deny / 沉默三档，没有 ask。" +
				"注意它的语义陷阱：返回 allow 是「预先批准并跳过用户确认」，不是「我没意见」——" +
				"这正是 ratchet 从不输出 allow 的原因之一。项目级配置是仓库根的 crush.json。",
		},
		{
			harness: hook.HarnessDroid, label: "Factory Droid", file: ".factory/hooks.json",
			event: "PreToolUse", verified: true, shape: shapeUnwrapped,
			trustNote: "Droid 在启动时对钩子做快照，外部改了会告警；" +
				"改完在 /hooks 界面里确认一次。项目级配置是仓库根的 .factory/hooks.json。",
		},
		{
			harness: hook.HarnessCline, label: "Cline", file: ".clinerules/hooks/PreToolUse",
			event: "PreToolUse", verified: true, shape: shapeFileDrop,
			trustNote: "**Cline 没有配置文件**：钩子就是约定目录里的一个可执行文件，" +
				"文件名必须正好叫 PreToolUse（CLI 那边是 .cline/hooks/PreToolUse.sh）。" +
				"而且必须在 Cline 的设置里勾上「Enable Hooks」，否则它不跑。" +
				"VS Code 与 CLI 的载荷形状不同，我们的脚本两边都兼容。",
		},
		{
			harness: hook.HarnessOpenCode, label: "OpenCode", file: ".opencode/plugin/ratchet-guard.js",
			event: "tool.execute.before", verified: true, shape: shapePlugin,
			trustNote: "OpenCode 没有 shell 钩子，只有 JS/TS 插件；" +
				"所以装的是一个插件文件，它会在工具执行前调用 ratchet guard，拒绝就抛错。" +
				"插件里用的是 ratchet 的绝对路径，换机器要重装一次。",
		},
	}
}

func findTarget(name string) (installTarget, bool) {
	h := hook.ParseHarness(name)
	for _, t := range installTargets() {
		if t.harness == h {
			return t, true
		}
	}
	return installTarget{}, false
}

// cmdGuardInstall 打印（默认不写）把执行点挂进某个 agent 的配置片段。
//
// 默认只打印不落盘：改用户的 agent 配置是高影响动作，要显式 --write 才动。
func cmdGuardInstall(args []string) int {
	fs := flag.NewFlagSet("guard install", flag.ContinueOnError)
	harness := fs.String("harness", "claude-code",
		"claude-code / codex / codebuddy / qwen-code / qoder / gemini-cli / antigravity")
	write := fs.Bool("write", false, "真的写进配置文件（默认只打印）")
	target := fs.String("settings", "", "要写入的配置文件（默认取各家的标准位置）")
	bin := fs.String("bin", "ratchet", "hook 里调用的可执行文件路径")
	matcher := fs.String("matcher", "*", "限制哪些工具的调用会触发（默认全部）")
	all := fs.Bool("all", false, "把所有已知 agent 的配置片段都打出来")
	list := fs.Bool("list", false, "只列出支持的 agent")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *list {
		fmt.Println("ratchet guard 支持的 agent：")
		for _, t := range installTargets() {
			mark := "✅ 已核实"
			if !t.verified {
				mark = "⚠️ 路径待核实（只打印不代写）"
			}
			fmt.Printf("  %-14s %-32s %s\n", t.harness, t.label+" · "+t.event, mark)
		}
		return 0
	}

	if *all {
		for i, t := range installTargets() {
			if i > 0 {
				fmt.Println()
			}
			printSnippet(t, *bin, *matcher)
		}
		return 0
	}

	t, ok := findTarget(*harness)
	if !ok {
		fmt.Fprintf(os.Stderr, "不认识的 agent：%s（用 --list 看支持的）\n", *harness)
		return 2
	}

	if !*write {
		printSnippet(t, *bin, *matcher)
		return 0
	}
	if !t.verified {
		fmt.Fprintf(os.Stderr,
			"拒绝代写 %s：这个 agent 的配置路径还没有一手文档佐证。\n"+
				"照上面的片段手动合并，或用 --settings 指定确切路径。\n", t.label)
		return 2
	}
	return writeSnippet(t, *target, *bin, *matcher)
}
