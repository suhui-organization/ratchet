package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suhui-organization/ratchet/internal/guard"
	"github.com/suhui-organization/ratchet/internal/hook"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/observe"
	"github.com/suhui-organization/ratchet/internal/store"
)

// guardRecord 是一条拦截判定记录。
//
// 它比 observe.Call 多出来的东西，正是"拦截"这件事需要被证明的部分：
// 依据哪一条规则、命中了哪个受保护目标、那个目标的指纹是什么、
// 以及**按哪一版策略判的**（PolicyHash）。
//
// 没有 PolicyHash，事后无法回答"当时是规则松还是执行点没生效"——
// 而这两件事的处理方式完全不同。
type guardRecord struct {
	TS         string        `json:"ts"`
	Agent      string        `json:"agent,omitempty"`
	Server     string        `json:"server"`
	Tool       string        `json:"tool"`
	Decision   string        `json:"decision"`
	Rule       string        `json:"rule"`
	Reason     string        `json:"reason"`
	Capability string        `json:"capability,omitempty"`
	Matches    []guard.Match `json:"matches,omitempty"`
	ArgKeys    []string      `json:"argKeys,omitempty"`
	PolicyHash string        `json:"policyHash,omitempty"`
	Caller     string        `json:"caller,omitempty"`
}

// guardOptions 是命令层的开关集合。
type guardOptions struct {
	policyPath string
	harness    string
	server     string
	agent      string
	storePath  string
	failClosed bool
	recordVals bool
	quiet      bool
}

func cmdGuard(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "check":
			return cmdGuardCheck(args[1:])
		case "install":
			return cmdGuardInstall(args[1:])
		}
	}

	fs := flag.NewFlagSet("guard", flag.ContinueOnError)
	opts := guardOptions{}
	fs.StringVar(&opts.policyPath, "policy", "", "策略文件（默认 $RATCHET_HOME/policy.json）")
	fs.StringVar(&opts.harness, "harness", "auto", "事件来源：auto / codex / claude-code / generic")
	fs.StringVar(&opts.server, "server", "", "覆盖 server 名")
	fs.StringVar(&opts.agent, "agent", "", "覆盖 agent 名")
	fs.StringVar(&opts.storePath, "store", "", "调用记录路径（默认 $RATCHET_HOME/calls.jsonl）")
	fs.BoolVar(&opts.failClosed, "fail-closed", false, "策略缺失或读不出时也拒绝（默认放行并告警）")
	fs.BoolVar(&opts.recordVals, "record-values", false, "把参数值原文写进拦截记录（默认只写哈希与前 12 字符）")
	if err := fs.Parse(args); err != nil {
		return 0 // hook 上下文：解析失败也不能卡住 agent
	}
	return runGuardHook(opts)
}

// runGuardHook 是挂在 PreToolUse 上的那个进程。
//
// 纪律与观测侧**不同**，必须区分清楚：
//   - 观测侧（ingest）永不阻塞、永不失败——它只是个记录器。
//   - 执行点（本命令）在判定为拒绝时必须真的拒绝，**它就是要阻塞**。
//
// 但对"自己出错"的处理又必须保守：解析不出事件、策略读不到这类故障，
// 不能把客户锁在自己的机器外。所以默认放行 + 大声告警 + 落一条记录，
// 让"执行点当时没在保护"这件事可查。要更严就加 --fail-closed。
func runGuardHook(opts guardOptions) int {
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil || len(raw) == 0 {
		return 0 // 读不到事件：不是我们能判的调用，交回 agent 的默认流程
	}

	// harness 只判一次，解析与输出共用同一个答案。
	// 分两次判会出现"按 CodeBuddy 解析、按 Claude 输出"这种错配——
	// 而错配的表现是**静默不拦**，最难发现的一类故障。
	h := effectiveHarness(opts.harness, raw)

	call, ok := hook.TranslatePre(raw, h, opts.server, opts.agent)
	if !ok {
		// 认不出来的形状：不猜、不拦。
		//
		// 拦是错的——钩子认不出输入就阻断，会把客户的 agent 卡死在无关的事情上。
		// 但**必须说出来**：静默返回 0 与"判定为放行"在日志里长得一模一样，
		// 而这两件事的含义完全不同（一个是"我看过了，没问题"，一个是"我没看懂"）。
		// 客户装了钩子却因为格式不匹配而从来没生效过，是这套东西最糟的失败形态。
		fmt.Fprintf(os.Stderr,
			"ratchet guard：认不出这次的载荷（--harness %s），本次**未拦截**。\n"+
				"  多半是配置文件挂错了事件，或 agent 版本换了载荷格式。\n"+
				"  拿一份真实载荷试：ratchet guard check < payload.json\n", h)
		return 0
	}

	p, policyHash, loadErr := loadGuardPolicy(opts.policyPath)
	if loadErr != nil {
		return guardOnConfigFailure(call, h, opts, loadErr)
	}

	verdict := guard.Decide(p, guard.Request{
		Agent: call.Agent, Server: call.Server, Tool: call.Tool,
		Args: call.Args, Values: call.Values,
	}, guard.Options{FailClosed: opts.failClosed})

	recordGuard(opts, call, verdict, policyHash)
	emitGuardDecision(h, call, verdict)
	return 0
}

// effectiveHarness 定出到底是哪个 agent 在调我们。
//
// 顺序：显式 --harness > 环境变量 > 载荷形状 > 兜底 Claude。
//
// 环境变量优先于形状，是因为 Claude / CodeBuddy / Qwen 三家的载荷几乎一模一样，
// 只靠形状会认错；而各家 hook 进程都继承了 agent 自己设的变量
// （CLAUDE_PROJECT_DIR / CODEBUDDY_PROJECT_DIR / QWEN_PROJECT_DIR），那是最可靠的判据。
func effectiveHarness(explicit string, raw []byte) hook.Harness {
	if h := hook.ParseHarness(explicit); h != hook.HarnessAuto {
		return h
	}
	if h := hook.DetectFromEnv(os.Environ()); h != hook.HarnessAuto {
		return h
	}
	if h := hook.Detect(raw); h != hook.HarnessAuto {
		return h
	}
	return hook.HarnessClaude // 兜底：最常用且有标准化阻断协议的一家
}

// guardOnConfigFailure 处理"策略读不出来"这件事。
//
// 这是本产品最需要想清楚的一个失败模式：策略文件丢了，执行点该怎么办？
//   - 全放 → 客户以为自己被保护着，其实没有（危险，但工作不中断）
//   - 全拦 → 客户当场用不了自己的电脑（安全，但是产品事故）
//
// 默认选前者，但把它变成**可见**的：stderr 上一条明确的话 + 记录里一条
// rule=guard.config-error 的条目。沉默的失效才是真正不可接受的。
func guardOnConfigFailure(call hook.PreCall, h hook.Harness, opts guardOptions, cause error) int {
	fmt.Fprintf(os.Stderr,
		"ratchet guard: 策略读不出来（%v）——本次**未拦截**。\n"+
			"  这表示执行点当前没有在保护这台机器，请尽快修好策略文件。\n"+
			"  高安全场景请在 hook 上加 --fail-closed，那时这种情况会直接拒绝。\n", cause)
	rec := guardRecord{
		TS: time.Now().UTC().Format(time.RFC3339), Agent: call.Agent,
		Server: call.Server, Tool: call.Tool, Decision: string(guard.Allow),
		Rule: "guard.config-error", Reason: cause.Error(),
		ArgKeys: argKeys(call), Caller: "guard",
	}
	appendGuardRecord(opts, rec)
	if opts.failClosed {
		// 打开 fail-closed 时，配置故障按拒绝处理。
		deny := guard.Verdict{
			Decision: guard.Deny, Rule: "guard.fail-closed",
			Reason: "策略读不出来且已开启 fail-closed，拒绝本次调用：" + cause.Error(),
		}
		emitGuardDecision(h, call, deny)
	}
	return 0
}

// loadGuardPolicy 读策略并算出它的指纹。
//
// 指纹算的是**文件原文**，不是解析后的结构：这样记录的 PolicyHash 能直接
// 和客户手上的那份 policy.json 对上（sha256sum 一比就完事），不需要再实现一遍序列化。
func loadGuardPolicy(path string) (model.Policy, string, error) {
	if path == "" {
		path = filepath.Join(store.DefaultHome(), "policy.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Policy{}, "", err
	}
	var p model.Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return model.Policy{}, "", fmt.Errorf("%s 不是合法 JSON：%w", path, err)
	}
	return p, guard.HashValue(string(raw)), nil
}

// emitGuardDecision 把判定翻译成**该 agent 认识的形状**。
//
// 这层翻译是执行点能否真的拦住的分水岭：判定得再对，输出形状不对，
// agent 就不会照做——而且是**静默地**不照做，日志里看起来一切正常。
//
// 一条纪律压过所有格式细节：**我们从不输出 allow。**
//
// 在 Claude Code 与 CodeBuddy 的文档里，"allow"的语义是"绕过权限系统直接执行"。
// 输出它等于替客户跳过了他自己的确认——那不是"我没意见"，那是替人做决定。
// 而**沉默**才是真正的"没有意见"：exit 0、什么都不打，agent 的常规权限流程照常走。
//
// 于是：拒绝 → 说出来；要求人工确认 → 能说就说；放行 → 闭嘴。
// 这样执行点在结构上就只可能让事情变得更严，不可能更松。
func emitGuardDecision(h hook.Harness, call hook.PreCall, v guard.Verdict) {
	_ = call
	e := emissionFor(h, v)
	if e.Stderr != "" {
		fmt.Fprintln(os.Stderr, e.Stderr)
	}
	if e.Payload != nil {
		writeJSONLine(os.Stdout, e.Payload)
	}
	if e.ExitCode != 0 {
		os.Exit(e.ExitCode)
	}
}

// emission 是一次输出的完整描述。
//
// 抽成纯函数是为了让"从不输出 allow"这条纪律可以被测试直接锁住——
// 它是这套东西里最容易被后来者无意破坏的一条，而破坏之后的表现是
// **替客户跳过了他自己的权限确认**，没有任何报错。
//
// 另一条同样重要的纪律：**不用退出码阻断。**
// Gemini CLI 认退出码 2，但反重力系的非零退出码**只写日志、不阻断**；
// 两家都认 stdout 上的 JSON。所以统一走"退出码 0 + JSON"——
// 一份输出同时喂两家，而且避开"退出码在另一家失效"这个坑。
type emission struct {
	Payload  any    // 打到 stdout 的 JSON（nil = 不打）
	Stderr   string // 打到 stderr 的话（只用于说明，不承担阻断）
	ExitCode int    // 非 0 表示配合退出码阻断
}

func emissionFor(h hook.Harness, v guard.Verdict) emission {
	proto := hook.ProtocolOf(h)

	switch v.Decision {
	case guard.Allow:
		// 沉默。**这是本函数存在的全部意义**：我们从不输出 allow。
		return emission{}

	case guard.Ask:
		if !proto.SupportsAsk {
			// 这个 agent 没有"交给人确认"这一档（Codex 会把带 ask 的输出
			// 判成钩子失败然后继续执行）。
			//
			// 降级成**沉默**而不是拒绝：沉默会让 agent 自己的权限流程接手，
			// 而拒绝会把每一次正常写入都变成硬失败——客户当天就会卸载我们，
			// 那时保护强度归零。
			return emission{
				Stderr: fmt.Sprintf("ratchet guard：%s 不支持人工确认，本次交回它自己的权限流程（依据：%s）",
					proto.Name, v.Reason),
			}
		}
		return emission{Payload: askPayload(h, proto, v.Reason)}

	case guard.Deny:
		e := emission{Payload: denyPayload(h, proto, v.Reason, v.Rule)}
		return e
	}
	return emission{}
}

// hookEventNameFor 是各家对"工具执行前"这个事件的叫法。
func hookEventNameFor(h hook.Harness) string {
	switch h {
	case hook.HarnessGemini:
		return "BeforeTool" // Gemini 系叫 BeforeTool
	case hook.HarnessCursor:
		return "preToolUse" // Cursor 用小驼峰
	}
	return "PreToolUse"
}

func denyPayload(h hook.Harness, p hook.Protocol, reason, rule string) any {
	switch p.DenyField {
	case hook.DenyTopLevelDecision:
		// Qoder / Crush：拒绝写在顶层 decision 上。
		// （Qoder 里 "deny" 等价于退出码 2；Crush 里 deny 的优先级高于 allow。）
		return map[string]any{"decision": "deny", "reason": reason}
	case hook.DenyDecisionBlock:
		// Gemini CLI：顶层 decision 取 "block"。
		return map[string]any{"decision": "block", "reason": reason}
	case hook.DenyCursorPermission:
		// Cursor 的字段名与所有别家都不同，而且理由要分两份：
		// user_message 给人看，agent_message 给 agent 看。
		// 只给一份的话，要么用户看不懂为什么被拦，要么 agent 不知道该怎么改。
		return map[string]any{
			"permission":    "deny",
			"user_message":  reason,
			"agent_message": reason,
		}
	case hook.DenyCancel:
		// Cline：钩子是一个可执行文件，靠 cancel 布尔值决定放不放行。
		return map[string]any{"cancel": true, "errorMessage": reason}
	case hook.DenyPluginThrow:
		// OpenCode：载荷由我们自己的插件消费，插件读到 deny 就抛错。
		return map[string]any{"decision": "deny", "reason": reason}
	case hook.DenyGeneric:
		return map[string]any{"decision": "deny", "rule": rule, "reason": reason}
	default:
		return map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            hookEventNameFor(h),
				"permissionDecision":       "deny",
				"permissionDecisionReason": reason,
			},
		}
	}
}

func askPayload(h hook.Harness, p hook.Protocol, reason string) any {
	if p.DenyField == hook.DenyTopLevelDecision {
		// Qoder 的 ask 不走顶层 decision，要用 hookSpecificOutput.permissionDecision。
		return map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":      hookEventNameFor(h),
				"permissionDecision": "ask",
			},
		}
	}
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            hookEventNameFor(h),
			"permissionDecision":       "ask",
			"permissionDecisionReason": reason,
		},
	}
}

// recordGuard 落一条判定记录，并把被拒的调用接进既有的调用记录。
//
// 两处都写是有意的：
//   - guard.jsonl 是**拦截证据**（判据、命中的目标、策略指纹）
//   - calls.jsonl 是**行为记录**，进去之后就自动进入现有的哈希链事件流
//
// 第二点是这次改动里最要紧的整合：ROLES.md 里写着"decision=deny 的事件是模型
// 自己拦的，我们只是记下来"。从这条命令开始，**Ratchet 自己能产生这个事件了**——
// 于是"某年某月某日它挡下了一次删除"是可验证的，而不是转述别人的话。
func recordGuard(opts guardOptions, call hook.PreCall, v guard.Verdict, policyHash string) {
	rec := guardRecord{
		TS: time.Now().UTC().Format(time.RFC3339), Agent: call.Agent,
		Server: call.Server, Tool: call.Tool, Decision: string(v.Decision),
		Rule: v.Rule, Reason: v.Reason, Capability: v.Capability,
		Matches: v.Matches, ArgKeys: argKeys(call),
		PolicyHash: policyHash, Caller: "guard",
	}
	if v.Decision != guard.Allow {
		if !opts.recordVals {
			rec.Matches = redactForRecord(rec.Matches)
		}
	}
	appendGuardRecord(opts, rec)
	recordCallReflectsGuard(opts, call, v)
}

// redactForRecord 去掉记录里的敏感原文（保留模式与哈希）。
func redactForRecord(ms []guard.Match) []guard.Match {
	out := make([]guard.Match, 0, len(ms))
	for _, m := range ms {
		m.Value = "" // 展示形式已在告警里给过；落盘的记录只留指纹
		out = append(out, m)
	}
	return out
}

func appendGuardRecord(opts guardOptions, rec guardRecord) {
	path := filepath.Join(store.DefaultHome(), "guard.jsonl")
	if opts.storePath != "" {
		// 调用记录路径给了就跟着走，测试里能把两处产物收到同一个目录
		path = filepath.Join(filepath.Dir(opts.storePath), "guard.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "ratchet guard: 建目录失败：%v\n", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ratchet guard: 打开拦截记录失败：%v\n", err)
		return
	}
	defer f.Close()
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
}

// recordCallReflectsGuard 把这次判定写进调用记录，让它进入事件链。
func recordCallReflectsGuard(opts guardOptions, call hook.PreCall, v guard.Verdict) {
	c := observe.Call{
		Agent: call.Agent, Server: call.Server, Tool: call.Tool,
		ArgKeys: argKeys(call),
	}
	switch v.Decision {
	case guard.Deny:
		c.Decision, c.Outcome = "deny", "blocked"
	case guard.Ask:
		// ask 不写 deny：它还没被拒，是在等人。
		// 记成 deny 会把"拦住了"和"在等人看"混成一件事，事后统计全错。
		c.Decision, c.Outcome = "approve", "pending"
	default:
		c.Decision, c.Outcome = "allow", "ok"
	}
	if err := store.Append(opts.storePath, c); err != nil {
		fmt.Fprintf(os.Stderr, "ratchet guard: 写调用记录失败：%v\n", err)
	}
}

func argKeys(call hook.PreCall) []string {
	keys := make([]string, 0, len(call.Args))
	for k := range call.Args {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func writeJSONLine(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// cmdGuardCheck 拿一次**具体的调用**试跑执行点，用来在挂 hook 之前确认不会误拦。
//
// 存在的理由和 policy check 一样：写完拦截规则后，"它会不会把我正常的工作流挡掉"
// 是唯一要紧的问题，而这个问题只能靠真实参数试出来。
func cmdGuardCheck(args []string) int {
	fs := flag.NewFlagSet("guard check", flag.ContinueOnError)
	policyPath := fs.String("policy", "", "策略文件（默认 $RATCHET_HOME/policy.json）")
	asJSON := fs.Bool("json", false, "输出机器可读结果")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil || len(raw) == 0 {
		fmt.Fprintln(os.Stderr, `从 stdin 读不到调用。形状：{"server":"…","tool":"…","args":{"path":"…"}}`)
		return 2
	}
	call, ok := hook.TranslatePre(raw, hook.HarnessGeneric, "", "")
	if !ok {
		// 非 generic 形状（Claude/Codex 的 hook payload）也允许直接喂进来试跑
		call, ok = hook.TranslatePre(raw, hook.HarnessAuto, "", "")
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "调用解析不了：需要 server/tool（或 tool_name）")
		return 2
	}

	p, hash, err := loadGuardPolicy(*policyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取策略失败：%v\n", err)
		return 1
	}
	v := guard.Decide(p, guard.Request{
		Agent: call.Agent, Server: call.Server, Tool: call.Tool,
		Args: call.Args, Values: call.Values,
	}, guard.Options{})

	if *asJSON {
		writeJSONLine(os.Stdout, map[string]any{
			"call":       map[string]any{"server": call.Server, "tool": call.Tool},
			"verdict":    v,
			"policyHash": hash,
		})
		return 0
	}
	fmt.Printf("ratchet %s — 执行点试跑\n\n", version)
	fmt.Printf("  调用        %s/%s\n", call.Server, call.Tool)
	fmt.Printf("  能力        %s\n", orDash(v.Capability))
	fmt.Printf("  动作        %s\n", strings.ToUpper(string(v.Decision)))
	fmt.Printf("  规则        %s\n", v.Rule)
	fmt.Printf("  依据        %s\n", v.Reason)
	for _, m := range v.Matches {
		fmt.Printf("  命中        %s → %s（指纹 %s）\n", m.ArgKey, m.Pattern, m.Hash)
	}
	fmt.Printf("  策略指纹    %s\n", hash)
	fmt.Println()
	switch v.Decision {
	case guard.Deny:
		fmt.Println("  → 这次调用会被**拒绝**，agent 拿不到执行结果。")
	case guard.Ask:
		fmt.Println("  → 这次调用会**停下来等人确认**（agent 无法自行继续）。")
	default:
		fmt.Println("  → 这次调用会**放行**，同时留一条记录。")
	}
	return 0
}
