package policy

import (
	"path"
	"strings"

	"github.com/suhui-organization/ratchet/internal/model"
)

// SensitivePathDeny 是**编译期**给"带路径参数的工具"默认加上的黑名单。
//
// 为什么是黑名单而不是白名单：白名单必须知道"这个工具只该访问哪些目录"，
// 那是使用者的业务判断，工具猜不出来。黑名单是普适的——不管你在哪个项目，
// 读 .env 和私钥都不该被自动放行。所以编译期只加黑名单，
// 白名单留给使用者自己写（`policy check` 可以拿真实调用试）。
var SensitivePathDeny = []string{
	"**/.env", "**/.env.*",
	"**/.ssh/**", "**/id_rsa", "**/id_ed25519",
	"**/.aws/**", "**/.kube/config",
	"**/.git-credentials", "**/.npmrc", "**/.netrc",
	"**/*credentials*", "**/*secret*",
}

// pathArgHints 是"这个参数名大概装的是路径"的词典。
//
// 只认常见命名——认不出来就不加约束，而不是硬猜。
// 猜错会让一个正常调用被拒，那比不加约束更糟：用户会直接放弃这套规则。
var pathArgHints = []string{
	"path", "paths", "file", "filename", "filepath", "dir", "directory",
	"folder", "cwd", "root", "target", "source", "src", "dest", "destination",
}

// LooksLikePathArg 判断某个参数名是否像路径。
func LooksLikePathArg(key string) bool {
	k := strings.ToLower(key)
	for _, hint := range pathArgHints {
		if k == hint || strings.HasSuffix(k, "_"+hint) || strings.HasPrefix(k, hint+"_") {
			return true
		}
	}
	return false
}

// looksLikePathValue 判断一个值是否**看起来**是路径。
//
// 只对看起来像路径的值做 allow 判定：把一个普通字符串拿去匹配路径白名单，
// 会把正常调用判成违规。判断不出来就不判——这是刻意的。
func looksLikePathValue(v string) bool {
	if v == "" {
		return false
	}
	switch {
	case strings.HasPrefix(v, "/"), strings.HasPrefix(v, "./"), strings.HasPrefix(v, "../"):
		return true
	case strings.HasPrefix(v, "~/"):
		return true
	case strings.Contains(v, "/"): // a/b 这种也算
		return true
	case strings.HasPrefix(v, "\\"): // Windows 绝对路径
		return true
	}
	return false
}

// globMatch 匹配路径模式。
//
// `**` 跨目录，`*` 不跨；用 path.Match 逐段比，避免手写正则。
// 末尾的 `/**` 表示"这个目录下的一切"，所以也要匹配目录本身。
func globMatch(pattern, value string) bool {
	if pattern == "" {
		return false
	}
	// 统一分隔符：Windows 传来的路径也要能匹配
	value = strings.ReplaceAll(value, "\\", "/")
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		if ok, _ := path.Match(base, value); ok {
			return true
		}
		return matchSegments(base+"/**", value)
	}
	return matchSegments(pattern, value)
}

// matchSegments 展开 `**` 递归匹配。
func matchSegments(pattern, value string) bool {
	pi := strings.Index(pattern, "**")
	if pi < 0 {
		ok, _ := path.Match(pattern, value)
		return ok
	}
	prefix := pattern[:pi]
	suffix := strings.TrimPrefix(pattern[pi+2:], "/")
	// 前缀必须是 value 的前缀（去掉尾随分隔符）
	pfx := strings.TrimSuffix(strings.TrimSuffix(prefix, "/"), "/")
	if pfx != "" {
		if !strings.HasPrefix(value, pfx+"/") && value != pfx {
			return false
		}
		value = strings.TrimPrefix(strings.TrimPrefix(value, pfx), "/")
	}
	if suffix == "" {
		return true
	}
	// 在任意一层边界上尝试匹配后缀（`**` 可跨任意层）
	if matchSegments(suffix, value) {
		return true
	}
	for i := 0; i < len(value); i++ {
		if value[i] == '/' && matchSegments(suffix, value[i+1:]) {
			return true
		}
	}
	return false
}

// Violation 是一次参数级违规。
type Violation struct {
	Tool   string
	ArgKey string
	Value  string
	Rule   string // 命中的规则，报告里要能说清是哪一条拒的
	Reason string
}

// Verdict 查一次调用在**三态层面**的结论（不含参数约束）。
//
// 策略文件里只有三个列表，没有"查一次调用"的入口——而网关、报告、
// 试跑工具都需要同一个答案，所以判定只能有一份。
// 未登记的工具落到 defaultDecision（编译期固定为 deny）。
func Verdict(p model.Policy, server, tool string) model.Decision {
	rules, ok := p.Servers[server]
	if !ok {
		return defaultOf(p)
	}
	if contains(rules.Deny, tool) {
		return model.Deny
	}
	if contains(rules.Approve, tool) {
		return model.Approve
	}
	if contains(rules.Allow, tool) {
		return model.Allow
	}
	return defaultOf(p)
}

func defaultOf(p model.Policy) model.Decision {
	if p.DefaultDecision == "" {
		return model.Deny // 缺省即拒绝：策略文件写坏了也不该放行
	}
	return p.DefaultDecision
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Evaluate 是给使用者的最终入口：先看参数约束，再看三态。
//
// 顺序不可颠倒：命中黑名单的调用不能因为"工具本身是 allow"就放行——
// 读 `.env` 的正是那个被标成 allow 的 read_file。
func Evaluate(p model.Policy, server, tool string, args map[string]string) (model.Decision, []Violation) {
	v := CheckArgs(p, server, tool, args)
	if len(v) > 0 {
		return model.Deny, v
	}
	return Verdict(p, server, tool), nil
}

// CheckArgs 对一次调用的参数做约束判定。返回 nil 表示没有违规（放行到三态判定）。
//
// 这是**纯函数**：同样的策略、工具、参数，永远得到同样的结论。
// 判定逻辑只有这一份——网关、报告、`policy check` 都调它，不许各写一份。
func CheckArgs(p model.Policy, server, tool string, args map[string]string) []Violation {
	rules, ok := p.Servers[server]
	if !ok {
		return nil
	}
	c, ok := rules.Constraints[tool]
	if !ok || c.Paths == nil {
		return nil
	}

	var out []Violation
	// 按参数名排序，保证输出稳定（map 遍历顺序是随机的）
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, key := range keys {
		value := args[key]
		if !LooksLikePathArg(key) && !looksLikePathValue(value) {
			continue // 既不像路径参数、值也不像路径，不做路径判定
		}
		for _, deny := range c.Paths.Deny {
			if globMatch(deny, value) {
				out = append(out, Violation{
					Tool: tool, ArgKey: key, Value: value, Rule: deny,
					Reason: "命中敏感路径黑名单",
				})
			}
		}
		if len(c.Paths.Allow) > 0 && looksLikePathValue(value) {
			matched := false
			for _, allow := range c.Paths.Allow {
				if globMatch(allow, value) {
					matched = true
					break
				}
			}
			if !matched {
				out = append(out, Violation{
					Tool: tool, ArgKey: key, Value: value, Rule: strings.Join(c.Paths.Allow, ", "),
					Reason: "不在允许的路径范围内",
				})
			}
		}
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
