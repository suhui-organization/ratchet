package hook

import "strings"

// 本文件回答一个问题：**同一个 `ratchet guard`，怎么让不同的 agent 都真的停下来。**
//
// 各家钩子的输入形状高度相似（几乎都是 tool_name + tool_input + permission_mode），
// 但输出协议有实质差异，而差异错了就是"静默不拦"——最危险的一类失败：
// 看起来装了、日志里有记录，但 agent 根本没收到拒绝。
//
// 所以协议按能力显式声明，不靠猜。

// Protocol 是某个 harness 的钩子协议能力。
//
// 三个字段各自对应一个真实踩过的坑：
//   - SupportsAsk：不支持 ask 的 agent（Codex、Gemini）会把带 ask 的输出当成
//     **钩子执行失败**然后继续执行——那是静默 fail-open。
//   - AllowIsNotSilent：在这些 agent 上返回 "allow" 不是"我没意见"，
//     而是"**绕过权限系统直接执行**"，等于替用户跳过了他自己的确认。
//   - IgnoreStdoutOnExit2：Qwen 的退出码 2 会**丢弃 stdout**、只把 stderr 当反馈。
type Protocol struct {
	Name string
	// SupportsAsk 为 true 时，可以用 hookSpecificOutput.permissionDecision="ask"
	// 让 agent 弹人工确认；false 时"需要人看"只能退化成拒绝或沉默。
	SupportsAsk bool
	// AllowIsNotSilent 为 true 表示"返回 allow"会绕过 agent 自己的权限系统。
	// 我们从不返回 allow——见 Decide 上方的纪律说明。
	AllowIsNotSilent bool
	// DenyField 决定拒绝写在哪个字段上。
	DenyField DenyStyle
}

// DenyStyle 是"拒绝"的编码方式。三种都是实测各家的官方文档口径。
type DenyStyle int

const (
	// DenyPermissionDecision：hookSpecificOutput.permissionDecision = "deny"
	// （Claude Code / CodeBuddy / Codex / Qwen Code / Factory Droid）
	DenyPermissionDecision DenyStyle = iota
	// DenyTopLevelDecision：顶层 {"decision":"deny","reason":…}（Qoder / Crush）
	DenyTopLevelDecision
	// DenyDecisionBlock：{"decision":"block","reason":…} + 退出码 2 + stderr（Gemini CLI）
	DenyDecisionBlock
	// DenyCursorPermission：{"permission":"deny","user_message":…,"agent_message":…}（Cursor）
	//
	// Cursor 的字段名和所有别家都不同：叫 permission 不叫 permissionDecision，
	// 而且理由要分两份（给用户看的 user_message、给 agent 看的 agent_message）。
	DenyCursorPermission
	// DenyCancel：{"cancel":true,"errorMessage":…}（Cline）
	//
	// Cline 的钩子是**一个可执行文件**，不是配置项；它靠 cancel 布尔值决定放不放行。
	DenyCancel
	// DenyPluginThrow：插件里抛异常表示拒绝（OpenCode）
	//
	// OpenCode 没有 shell 钩子，只有 JS/TS 插件；拦截靠插件抛错。
	// 这条路要用户装一个插件文件，所以它是本表里唯一"需要额外落地物"的一档。
	DenyPluginThrow
	// DenyGeneric：非标准 agent 用的自描述形状
	DenyGeneric
)

// ProtocolOf 返回某个 harness 的协议能力。
func ProtocolOf(h Harness) Protocol {
	switch h {
	case HarnessCodex:
		// 官方文档：permissionDecision: "ask" 被解析但**不支持**——
		// Codex 会把这次 hook 标记为失败并继续执行工具调用。
		return Protocol{Name: "codex", SupportsAsk: false, AllowIsNotSilent: true, DenyField: DenyPermissionDecision}
	case HarnessClaude:
		return Protocol{Name: "claude-code", SupportsAsk: true, AllowIsNotSilent: true, DenyField: DenyPermissionDecision}
	case HarnessCodeBuddy:
		// 与 Claude 同构：同样的输入字段、同样的 hookSpecificOutput。
		// 文档里 "allow" 的措辞也是"Bypass the permission system"。
		return Protocol{Name: "codebuddy", SupportsAsk: true, AllowIsNotSilent: true, DenyField: DenyPermissionDecision}
	case HarnessQwen:
		return Protocol{Name: "qwen-code", SupportsAsk: true, AllowIsNotSilent: true, DenyField: DenyPermissionDecision}
	case HarnessQoder:
		// 拒绝用顶层 decision；ask 才用 hookSpecificOutput.permissionDecision。
		return Protocol{Name: "qoder", SupportsAsk: true, AllowIsNotSilent: true, DenyField: DenyTopLevelDecision}
	case HarnessGemini:
		// Gemini CLI 的 BeforeTool 只有 decision:"deny"/"block"，没有 ask。
		//
		// 走退出码 0 + JSON 而不走退出码 2：官方把 0 + JSON 列为
		// "Preferred for all logic"，而退出码 2 是退路。
		return Protocol{Name: "gemini-cli", SupportsAsk: false, AllowIsNotSilent: true, DenyField: DenyDecisionBlock}
	case HarnessAntigravity:
		// Antigravity 与 Gemini CLI 在**阻断机制上是反的**，这是整个协议表里
		// 最容易踩错的一处：
		//
		//   Gemini CLI：非零退出码（2）才是 System Block。
		//   Antigravity：非零退出码**只被记进日志，不阻断**；
		//                必须用 stdout 上的 {"decision":"deny"} 才拦得住。
		//
		// 所以两者都统一走"退出码 0 + JSON"——Gemini 认这条路（且是官方推荐），
		// Antigravity 只认这条路。一份输出同时喂两家，没有取舍。
		return Protocol{Name: "antigravity", SupportsAsk: false, AllowIsNotSilent: true, DenyField: DenyDecisionBlock}

	case HarnessCursor:
		// Cursor 的 preToolUse 覆盖所有工具，但**ask 在 preToolUse 上不被强制执行**
		// （官方原文："accepted by the schema but not enforced for preToolUse today"）。
		// 所以 ask 只能降级；真需要人工确认，得挂在只覆盖 shell 的 beforeShellExecution 上。
		return Protocol{Name: "cursor", SupportsAsk: false, AllowIsNotSilent: true, DenyField: DenyCursorPermission}

	case HarnessCrush:
		// Crush 的决策只有 allow / deny / null 三档，没有 ask。
		//
		// 而且这里有个真陷阱：**返回 allow 不是"我没意见"，是"预先批准并跳过用户的确认提示"**
		// （官方原文："If the final aggregated decision is allow, Crush pre-approves
		// the tool call and skips the permission prompt"）。所以沉默才是没意见。
		return Protocol{Name: "crush", SupportsAsk: false, AllowIsNotSilent: true, DenyField: DenyTopLevelDecision}

	case HarnessDroid:
		// Factory Droid 的输出协议与 Claude Code 完全一致（官方文档同一套字段），
		// 差别只在**配置形状**：Droid 的 hooks.json 直接用事件名做键，没有 hooks 外层。
		return Protocol{Name: "factory-droid", SupportsAsk: true, AllowIsNotSilent: true, DenyField: DenyPermissionDecision}

	case HarnessCline:
		// Cline 钩子是一个可执行文件，读 JSON、回 {"cancel":true/false}。
		// 没有 ask 这一档：要么取消这次调用，要么放它过去。
		return Protocol{Name: "cline", SupportsAsk: false, AllowIsNotSilent: false, DenyField: DenyCancel}

	case HarnessOpenCode:
		// OpenCode 没有 shell 钩子，靠 JS/TS 插件在工具执行前抛错来拦。
		// 没有 ask，也不解析 stdout 的 JSON——插件拿到的是我们生成的代码。
		return Protocol{Name: "opencode", SupportsAsk: false, AllowIsNotSilent: false, DenyField: DenyPluginThrow}
	}
	return Protocol{Name: "generic", SupportsAsk: false, AllowIsNotSilent: false, DenyField: DenyGeneric}
}

// ParseHarnessName 归一化用户在 --harness 里写的名字。
func ParseHarnessName(raw string) Harness {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "codex":
		return HarnessCodex
	case "claude", "claude-code", "claudecode":
		return HarnessClaude
	case "codebuddy", "code-buddy", "tencent":
		return HarnessCodeBuddy
	case "qwen", "qwen-code", "qwencode":
		return HarnessQwen
	case "qoder", "lingma", "tongyi":
		return HarnessQoder
	case "gemini", "gemini-cli":
		return HarnessGemini
	// Antigravity 是**独立的一档**，不能并到 Gemini：
	// 它的阻断机制和 Gemini CLI 不同（见 ProtocolOf），混在一起会拦不住。
	case "antigravity", "agy":
		return HarnessAntigravity
	case "cursor":
		return HarnessCursor
	case "crush":
		return HarnessCrush
	case "droid", "factory", "factory-droid", "factorydroid":
		return HarnessDroid
	case "cline":
		return HarnessCline
	case "opencode", "open-code", "sst":
		return HarnessOpenCode
	case "generic", "raw":
		return HarnessGeneric
	case "", "auto":
		return HarnessAuto
	}
	return HarnessAuto
}

// DetectFromEnv 从环境变量判断是哪个 agent 在调我们。
//
// 为什么优先看环境变量而不是 payload：各家 hook 进程都继承了 agent 自己设的
// 变量（CLAUDE_PROJECT_DIR / CODEBUDDY_PROJECT_DIR / QWEN_PROJECT_DIR），
// 这是**最可靠**的判据。而 payload 形状雷同（Claude / CodeBuddy / Qwen 三家
// 几乎一样），只靠形状会认错——认错会把拒绝发成对方看不懂的格式。
func DetectFromEnv(env []string) Harness {
	get := func(key string) string {
		for _, kv := range env {
			if v, ok := strings.CutPrefix(kv, key+"="); ok {
				return v
			}
		}
		return ""
	}
	switch {
	case get("CODEBUDDY_PROJECT_DIR") != "" || get("CODEBUDDY_PLUGIN_ROOT") != "":
		return HarnessCodeBuddy
	case get("QWEN_PROJECT_DIR") != "":
		return HarnessQwen
	case get("QODER_PROJECT_DIR") != "" || get("QODER_PLUGIN_ROOT") != "":
		return HarnessQoder
	case get("ANTIGRAVITY_CONVERSATION_ID") != "":
		return HarnessAntigravity
	case get("CLAUDE_PROJECT_DIR") != "" || get("CLAUDE_PLUGIN_ROOT") != "":
		// Codex 的插件钩子也会设 CLAUDE_PLUGIN_ROOT（兼容用），
		// 但它同时会设 PLUGIN_ROOT——那个是 Codex 独有的，先判它。
		if get("PLUGIN_ROOT") != "" {
			return HarnessCodex
		}
		return HarnessClaude
	case get("CRUSH") != "":
		// Crush 为钩子子进程设 CRUSH=1（官方文档）。
		return HarnessCrush
	case get("FACTORY_PROJECT_DIR") != "":
		return HarnessDroid
	case get("CLINE_HOOK") != "" || get("CLINE_WORKSPACE_ROOT") != "":
		return HarnessCline
	case get("OPENCODE_PLUGIN") != "":
		return HarnessOpenCode
	case get("CURSOR_PROJECT_DIR") != "":
		return HarnessCursor
	case get("PLUGIN_ROOT") != "":
		return HarnessCodex
	}
	return HarnessAuto
}
