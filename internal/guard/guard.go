// Package guard 是执行点：在工具调用**发生之前**给出放行 / 人工确认 / 拒绝。
//
// 它和 internal/policy 的分工：
//
//	policy  说"该不该"——编译期，看清单，产出三态判定与依据。
//	guard   说"拦不拦"——运行期，看这一次调用的真实参数，产出可执行的动作。
//
// 一条硬不变式：**guard 只会把判定改得更严，绝不会改得更松。**
//
// 因为拦截比放行危险得多——拦错了客户当天就会把执行点关掉，那时保护强度归零。
// 所以覆盖规则（受保护目标、破坏性升级）只能单向收紧，永不放松策略。
// 这条不变式有测试盯着。
package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/suhui-organization/ratchet/internal/hook"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/policy"
)

// Decision 是执行点的动作，比三态多一档 ask。
//
// 三态里没有 ask，因为策略是给人读的；而执行点上有"交给人确认"这个动作，
// 加在下面一档是必要的——不然只剩"全放"和"全拦"两个选项，
// 而"全拦"会让 agent 变成废物（客户第一天就关掉）。
type Decision string

const (
	Allow Decision = "allow"
	Ask   Decision = "ask"
	Deny  Decision = "deny"
)

// 规则标识。每条判定都要能回答"是哪一条拦下的"——答不上来的判定不算数。
const (
	RuleGuardOff        = "guard.disabled"
	RuleSensitiveTarget = "guard.sensitive-target"
	RuleProtectedTarget = "guard.protected-target"
	RuleDestructive     = "guard.destructive"
	RulePolicyDeny      = "policy.deny"
	RulePolicyApprove   = "policy.approve"
	RulePolicyAllow     = "policy.allow"
	RulePolicyUnknown   = "policy.unregistered"
)

// severity 把动作排成一个全序，用来实现"只收紧"。
func severity(d Decision) int {
	switch d {
	case Allow:
		return 0
	case Ask:
		return 1
	}
	return 2
}

// tighten 取更严的那一个。
func tighten(a, b Decision) Decision {
	if severity(b) > severity(a) {
		return b
	}
	return a
}

// Match 是一次"参数值命中规则"的记录。
//
// Value 是**脱敏后的展示形式**（前 12 个字符）；完整值只留哈希。
// 这样一条拦截记录既能证明"确实拦了、拦的是什么"，又不会把令牌和文件内容写进日志。
type Match struct {
	ArgKey  string `json:"argKey"`
	Pattern string `json:"pattern"`
	Value   string `json:"value"`
	Hash    string `json:"hash"`
}

// Verdict 是一次调用被判定后的结果。
type Verdict struct {
	Decision Decision `json:"decision"`
	// Rule 是做出这条判定的规则标识。
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
	// Matches 是命中受保护目标的具体项（只有 deny 由目标命中引起时才非空）。
	Matches []Match `json:"matches,omitempty"`
	// Capability 是这次调用被判定的能力类别（给报告与排查用）。
	Capability string `json:"capability,omitempty"`
}

// Request 是一次待判定的调用。
type Request struct {
	Agent  string
	Server string
	Tool   string
	Args   map[string]string
	Values []hook.ArgValue
}

// Options 控制判定行为。
type Options struct {
	// FailClosed 让"策略加载失败"也变成拒绝。默认关闭。
	//
	// 默认关闭是一个刻意的取舍：策略文件缺失/写坏时，拒绝一切会让客户
	// 当场用不了自己的电脑——那是产品事故。所以默认放行**但大声告警并记录**，
	// 让"执行点没在保护"这件事至少是可见的、可查的。
	// 安全要求高的场景（无人值守、CI）应当打开它。
	FailClosed bool
}

// Decide 是执行点的唯一入口。
func Decide(p model.Policy, req Request, opts Options) Verdict {
	g := p.Guard
	if g != nil && !g.Enabled {
		// 显式关掉时只记录：仍然要给出依据，让报告里说得清"这段时间没人拦"。
		dec, _ := policy.Evaluate(p, req.Server, req.Tool, req.Args)
		return Verdict{
			Decision: Allow,
			Rule:     RuleGuardOff,
			Reason:   "执行点未启用（guard.enabled=false）：判定为 " + string(dec) + "，但不拦截",
		}
	}

	cap, capReason := classify(req)
	verdict := Verdict{Capability: string(cap)}

	// 先判"这次调用会不会写或删"。后面两条目标规则要靠它区分读与写：
	// 读一份 schema.sql 是正常干活，改它是不可逆的——同一个目标，两种性质。
	writes := writesOrDestroys(cap, req.Values)
	if tok, ok := destructiveInArgs(req.Values); ok {
		writes = true
		_ = tok
	}

	// —— 规则 1：凭据类目标。任何操作都拒，包括读。——
	//
	// 为什么读也拒：私钥和 .env 被读走与被删掉，付出的代价是一样的，
	// 而且读走之后本地没有任何痕迹。审计侧对 .env 的态度本来就是"哪怕是
	// 被标成 allow 的 read_file 也不放行"，执行点必须与之一致。
	if matches := scanTargets(sensitiveOf(g), req.Values); len(matches) > 0 {
		verdict.Decision = Deny
		verdict.Rule = RuleSensitiveTarget
		verdict.Matches = matches
		verdict.Reason = describeTargets("凭据类目标", matches)
		return verdict
	}

	// —— 规则 2：不可恢复的目标。只在写或删时拒。——
	//
	// 为什么按目标而不是按工具：一个被标成 allow 的 write_file 照样能把
	// /srv/backup 覆盖掉。按工具名放行、再指望它"不会去碰那个目录"，
	// 是把安全性押在 agent 的自觉上。
	if writes {
		if matches := scanTargets(protectedOf(g), req.Values); len(matches) > 0 {
			verdict.Decision = Deny
			verdict.Rule = RuleProtectedTarget
			verdict.Matches = matches
			verdict.Reason = describeTargets("不可恢复的目标", matches)
			return verdict
		}
	}

	// —— 规则 3：策略的三态判定 ——
	dec, violations := policy.Evaluate(p, req.Server, req.Tool, req.Args)
	switch dec {
	case model.Deny:
		verdict.Decision = Deny
		verdict.Rule = RulePolicyDeny
		verdict.Reason = denyReason(p, req, violations)
	case model.Approve:
		verdict.Decision = actionFor(onApproveOf(g), Ask)
		verdict.Rule = RulePolicyApprove
		verdict.Reason = "策略把 " + key(req) + " 判为需人工审批（" + capReason + "）"
	default:
		verdict.Decision = Allow
		verdict.Rule = RulePolicyAllow
		verdict.Reason = "策略放行 " + key(req)
	}

	// —— 规则 4：破坏性升级。只加严，不放松。——
	destructive := cap == policy.CapDestructive
	destructiveWhy := capReason
	if !destructive {
		// 工具名中性、破坏性藏在参数里——最常见的一类。
		// `shell` / `Bash` / `run_command` 被判成"执行"（→ approve），
		// 但它要跑的那句可能正是 `rm -rf /srv/app`。只看工具名会把它
		// 当成"一条要审批的命令"，而它其实是不可逆的删除。
		if tok, ok := destructiveInArgs(req.Values); ok {
			destructive = true
			destructiveWhy = "参数里出现破坏性动作「" + tok + "」"
		}
	}
	if destructive && !allSafe(g, req.Values) {
		if s := tighten(verdict.Decision, Ask); s != verdict.Decision {
			verdict.Decision = s
			verdict.Rule = RuleDestructive
			verdict.Reason = "破坏性操作（" + destructiveWhy + "），且目标不在安全清单内：" + verdict.Reason
		}
		verdict.Capability = string(policy.CapDestructive)
	}
	return verdict
}

// writesOrDestroys 判断这次调用是否会改动状态。
//
// 读放行、写拦下，是"不可恢复目标"那条规则的全部依据：
// 目标清单里既有 schema.sql 这种每天要读的东西，也有备份卷这种不能碰的东西，
// 区分它们只能靠"这次要干什么"，不能靠"碰的是哪个路径"。
func writesOrDestroys(cap policy.Capability, values []hook.ArgValue) bool {
	switch cap {
	case policy.CapWrite, policy.CapDestructive:
		return true
	case policy.CapExecute:
		// 执行本身中性（`ls` 也是执行），但如果参数里带破坏性动词就是写。
		_, ok := destructiveInArgs(values)
		return ok
	}
	return false
}

// scanTargets 检查每个参数值是否命中一组目标模式。
//
// 两级匹配：
//  1. 整个值直接当路径匹配（`path=/srv/backup/prod.sql`）
//  2. 把值当命令行拆开，逐个 token 匹配（`command=rm -rf /srv/backup`）
//
// 只做第 1 级会漏掉"目标藏在命令字符串里"这个最常见的形态，
// 而那次删除正是最需要拦住的。
func scanTargets(patterns []string, values []hook.ArgValue) []Match {
	if len(patterns) == 0 {
		return nil
	}
	var out []Match
	seen := map[string]bool{}
	for _, v := range values {
		for _, candidate := range candidatesOf(v.Value) {
			for _, pattern := range patterns {
				if !policy.MatchGlob(pattern, candidate) {
					continue
				}
				mk := v.Key + "\x00" + pattern + "\x00" + hash(candidate)
				if seen[mk] {
					continue
				}
				seen[mk] = true
				out = append(out, Match{
					ArgKey:  v.Key,
					Pattern: pattern,
					Value:   short(candidate),
					Hash:    hash(candidate),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ArgKey != out[j].ArgKey {
			return out[i].ArgKey < out[j].ArgKey
		}
		return out[i].Pattern < out[j].Pattern
	})
	return out
}

// protectedOf / sensitiveOf 用自由函数而不是方法：model.Guard 是跨语言契约，
// 不该为了这里的便利往它上面挂行为。g 为 nil 时返回 nil（未配置 = 不启用该规则）。
func protectedOf(g *model.Guard) []string {
	if g == nil {
		return nil
	}
	return g.Protected
}

func sensitiveOf(g *model.Guard) []string {
	if g == nil {
		return nil
	}
	return g.Sensitive
}

// describeTargets 拼一句人能看懂、且点名了具体目标的拒绝理由。
func describeTargets(kind string, matches []Match) string {
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, m.ArgKey+"="+m.Value+" 命中 "+m.Pattern)
	}
	return "拒绝：命中" + kind + "（" + strings.Join(parts, "；") + "）"
}

func key(req Request) string { return model.Key(req.Server, req.Tool) }

func classify(req Request) (policy.Capability, string) {
	cap, m := policy.Classify(model.ToolObservation{Server: req.Server, Tool: req.Tool})
	return cap, m
}

// onApproveOf 取 approve 档的动作，缺省是"交给人确认"。
func onApproveOf(g *model.Guard) string {
	if g == nil || g.OnApprove == "" {
		return policy.GuardAsk
	}
	return g.OnApprove
}

// actionFor 把配置里的动作字符串映射成 Decision。
// 认不出来的取值回落到 fallback——配置写错时不能变成"放行"。
func actionFor(raw string, fallback Decision) Decision {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case policy.GuardAllow:
		return Allow
	case policy.GuardDeny:
		return Deny
	case policy.GuardAsk, "":
		return Ask
	}
	return fallback
}

func denyReason(p model.Policy, req Request, violations []policy.Violation) string {
	if len(violations) > 0 {
		parts := make([]string, 0, len(violations))
		for _, v := range violations {
			parts = append(parts, v.ArgKey+"="+short(v.Value)+" 命中 "+v.Rule)
		}
		return "参数约束拒绝：" + strings.Join(parts, "；")
	}
	if r := p.Rationale[key(req)]; r != "" {
		return "策略拒绝 " + key(req) + "：" + r
	}
	return "策略拒绝 " + key(req) + "（未登记的工具一律拒绝）"
}

// allSafe 判断这次调用的目标是否**全部**落在安全清单里。
//
// 必须全部：一次调用里只要有一个目标不在安全清单内，就不能算安全。
// （`rm -rf build/ /srv/backup` 这种混合形态，取"全"才拦得住。）
func allSafe(g *model.Guard, values []hook.ArgValue) bool {
	if g == nil || len(g.SafeTargets) == 0 {
		return false
	}
	checked := false
	for _, v := range values {
		for _, candidate := range candidatesOf(v.Value) {
			if !looksLikeTarget(candidate) {
				continue
			}
			checked = true
			if !anyMatch(g.SafeTargets, candidate) {
				return false
			}
		}
	}
	return checked
}

func anyMatch(patterns []string, value string) bool {
	for _, p := range patterns {
		if policy.MatchGlob(p, value) {
			return true
		}
	}
	return false
}

// candidatesOf 从一个参数值里取出所有值得拿去匹配的候选串。
//
// 整串 + 命令拆分后的每个 token。拆分只在看起来像命令时才做，
// 避免把一句自然语言拆得七零八落造成误报。
func candidatesOf(value string) []string {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	out := []string{v}
	if !looksLikeCommand(v) {
		return out
	}
	for _, tok := range strings.FieldsFunc(v, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ';', '|', '&', '>', '<', '(', ')', '"', '\'', '`', ',', '=':
			return true
		}
		return false
	}) {
		if tok != "" && tok != v {
			out = append(out, tok)
		}
	}
	return out
}

// looksLikeCommand 判断一个值是否像命令行（而不是自然语言）。
//
// 判据保守：含斜杠且含空格的，或者以常见命令词开头的。
// 判错会拆出假的 token 造成误拦，而误拦会让客户关掉执行点。
func looksLikeCommand(v string) bool {
	if !strings.ContainsAny(v, " \t;|&") {
		return false
	}
	head := v
	if i := strings.IndexFunc(v, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ';' || r == '|' || r == '&'
	}); i > 0 {
		head = v[:i]
	}
	switch strings.ToLower(head) {
	case "rm", "rmdir", "mv", "dd", "shred", "unlink",
		"sudo", "git", "npm", "npx", "pnpm", "yarn",
		"docker", "kubectl", "helm", "terraform", "aws", "gcloud", "az",
		"mysql", "psql", "mongo", "redis-cli", "sqlite3",
		"dropdb", "createdb", "systemctl", "service", "chmod", "chown",
		// SQL 侧的破坏性动词：`DROP TABLE users` 这类没有路径，只有动作
		"drop", "truncate", "delete", "alter":
		return true
	}
	return strings.Contains(v, " /") || strings.Contains(v, "./") || strings.Contains(v, "~/")
}

// destructiveInArgs 在参数值里找破坏性动作，返回命中的那个词。
//
// 只认"纯字母的词"，且只在看起来像命令/语句的值里找。这两条限制都是为了避免误报：
// 一个叫 `deleted_items.csv` 的文件名会被前缀匹配命中 `delete`，
// 那样每次读这个文件都要弹确认——客户会直接把这个功能关掉。
func destructiveInArgs(values []hook.ArgValue) (string, bool) {
	for _, v := range values {
		if !looksLikeCommand(v.Value) {
			continue
		}
		for _, tok := range candidatesOf(v.Value) {
			if !isPureWord(tok) {
				continue
			}
			if policy.IsDestructiveToken(tok) {
				return tok, true
			}
		}
	}
	return "", false
}

// isPureWord 判断一个词元是否全是字母——`rm`、`drop`、`remove` 是；
// `-rf`、`/srv/backup`、`deleted_items.csv` 不是。
func isPureWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// looksLikeTarget 判断一个候选串是否像"一个被操作的目标"。
// 用来决定它该不该参与安全清单判定——把 `-rf` 这种开关拿去比对毫无意义。
func looksLikeTarget(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return strings.Contains(s, "/") || s == "~" || s == "." || strings.HasPrefix(s, ".")
}

// short 是值的展示形式：前 12 个字符。够认出是哪个资产，不足以泄露内容。
func short(v string) string {
	r := []rune(v)
	if len(r) <= 12 {
		return v
	}
	return string(r[:12]) + "…"
}

// hash 是值的完整指纹。记录里用它证明"当时拦的就是这个值"，而不必写下这个值。
func hash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])[:16]
}

// HashValue 供外部（记录、报告）使用同一份哈希口径。
func HashValue(v string) string { return hash(v) }
