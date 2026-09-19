// Package policy 把工具清单编译成最小权限策略。
package policy

import (
	"sort"
	"time"

	"github.com/suhui-organization/ratchet/internal/model"
)

const (
	// PolicyVersion 是策略文件格式版本。改了字段语义就要改它。
	PolicyVersion = "1"
	// Generator 写进策略文件，便于事后分辨"这份策略是谁生成的"。
	Generator = "ratchet"
)

// Options 控制编译行为。
type Options struct {
	// StrictUnknown 把无法判定能力的工具判为 deny（默认 approve + 待确认）。
	StrictUnknown bool
	// OnlyObserved 只授予**被观测到调用过**的工具。
	// 这是最小权限最严格的一档：配置里挂着但从未用过的工具不进策略
	// （未登记的工具默认被拒绝，所以它们等于被收掉了）。
	OnlyObserved bool
	// Agent 覆盖清单里的 agent 名。
	Agent string
	// Locale 决定产物里的文字（判定依据）。零值 = en-US。
	Locale Locale
	// Now 注入时间，便于测试产出稳定输出。
	Now time.Time
}

// Draft 把清单编译成策略。
//
// 确定性是硬要求：同一份输入必须产出**逐字节相同**的策略。
// 否则用户没法把策略放进版本控制、也没法 diff "这次比上次收紧了什么"——
// 而那正是这个产品要说清楚的事。所以这里对 server 名与工具名都显式排序，
// 不依赖 map 遍历顺序。
func Draft(inv model.Inventory, opts Options) model.Policy {
	type entry struct {
		server string
		tool   string
		dec    model.Decision
		reason string
		review bool
	}

	// 先合并重复项，再判定。
	//
	// 顺序很重要：如果一边判定一边合并，先出现的那条已经写好了依据，
	// 后面更大的调用次数就补不进去了（实测踩过：依据里写着"观测到 1 次"，
	// 而实际是 5 次）。先合并再判定，依据一定是基于最终数据算的。
	merged := make(map[string]model.ToolObservation, len(inv.Tools))
	for _, t := range inv.Tools {
		if t.Server == "" || t.Tool == "" {
			continue
		}
		key := model.Key(t.Server, t.Tool)
		cur, ok := merged[key]
		if !ok {
			merged[key] = t
			continue
		}
		if t.Calls > cur.Calls {
			cur.Calls = t.Calls
		}
		if cur.Description == "" {
			cur.Description = t.Description
		}
		merged[key] = cur
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	loc := opts.Locale
	if loc == "" {
		loc = LocaleEN
	}

	entries := make([]entry, 0, len(keys))
	for _, key := range keys {
		t := merged[key]
		// 最小权限的字面含义：不授予没有观察到的能力。
		if opts.OnlyObserved && t.Calls == 0 {
			continue
		}
		cap, match := classifyMatch(t)
		dec, review := Decide(cap, opts.StrictUnknown)
		reason := match.describe(loc)
		// 观测到的调用次数是有用的上下文：写进依据里，报告和界面都要显示
		if t.Calls > 0 {
			if loc == LocaleZH {
				reason += "；观测到 " + itoa(t.Calls) + " 次调用"
			} else {
				reason += "; observed " + itoa(t.Calls) + " call(s)"
			}
		} else {
			if loc == LocaleZH {
				reason += "；未观测到调用（仅清单中存在）"
			} else {
				reason += "; never observed (present in the inventory only)"
			}
		}
		entries = append(entries, entry{server: t.Server, tool: t.Tool, dec: dec, reason: reason, review: review})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].server != entries[j].server {
			return entries[i].server < entries[j].server
		}
		return entries[i].tool < entries[j].tool
	})

	servers := make(map[string]model.ServerRules)
	rationale := make(map[string]string, len(entries))
	needsReview := make([]string, 0)
	for _, e := range entries {
		rules := servers[e.server]
		switch e.dec {
		case model.Allow:
			rules.Allow = append(rules.Allow, e.tool)
		case model.Approve:
			rules.Approve = append(rules.Approve, e.tool)
		case model.Deny:
			rules.Deny = append(rules.Deny, e.tool)
		}
		servers[e.server] = rules
		rationale[model.Key(e.server, e.tool)] = e.reason
		if e.review {
			needsReview = append(needsReview, model.Key(e.server, e.tool))
		}
	}
	// 参数级约束：给"带路径参数"的工具默认挂上敏感路径黑名单。
	//
	// 只挂黑名单、不挂白名单——白名单是业务判断（这个工具只该访问哪些目录），
	// 工具猜不出来；黑名单是普适的（读 .env 和私钥不该被自动放行）。
	// 白名单由使用者自己写，`policy check` 可以拿真实调用试。
	for _, t := range merged {
		hasPathArg := false
		for _, k := range t.ArgKeys {
			if LooksLikePathArg(k) {
				hasPathArg = true
				break
			}
		}
		if !hasPathArg {
			continue
		}
		rules := servers[t.Server]
		if rules.Constraints == nil {
			rules.Constraints = map[string]model.ToolConstraints{}
		}
		rules.Constraints[t.Tool] = model.ToolConstraints{
			Paths: &model.PathRules{Deny: append([]string(nil), SensitivePathDeny...)},
		}
		servers[t.Server] = rules
		rationale[model.Key(t.Server, t.Tool)] += pathConstraintNote(loc)
	}
	sort.Strings(needsReview)

	agent := inv.Agent
	if opts.Agent != "" {
		agent = opts.Agent
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	return model.Policy{
		Version:         PolicyVersion,
		Agent:           agent,
		DefaultDecision: model.Deny,
		Servers:         servers,
		Rationale:       rationale,
		NeedsReview:     needsReview,
		GeneratedAt:     now.UTC().Format(time.RFC3339),
		Generator:       Generator,
	}
}

// Counts 汇总策略规模，给 CLI 摘要与报告用。
func Counts(p model.Policy) (servers, tools, allow, approve, deny int) {
	for _, rules := range p.Servers {
		servers++
		allow += len(rules.Allow)
		approve += len(rules.Approve)
		deny += len(rules.Deny)
	}
	return servers, allow + approve + deny, allow, approve, deny
}

// itoa 避免为一处数字格式化引入 strconv 依赖以外的负担（保持本文件自足）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// pathConstraintNote 在依据里说明"这个工具额外挂了路径黑名单"。
//
// 依据里写清楚，用户才知道为什么不光要看三态、还要看参数级规则。
func pathConstraintNote(loc Locale) string {
	if loc == LocaleZH {
		return "；已加路径黑名单（.env / .ssh / 凭据等，见 constraints.paths.deny）"
	}
	return "; path deny-list applied (.env, .ssh, credentials — see constraints.paths.deny)"
}
