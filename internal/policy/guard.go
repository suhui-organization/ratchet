package policy

import "github.com/suhui-organization/ratchet/internal/model"

// 拦截的锚点放在**目标**上，不放在工具名上。
//
// 为什么不列"禁止哪些工具"：同一个删除能力可以叫 delete_file，也可以叫 rm、
// drop_table，还可以整句塞进一个 Bash 字符串里。禁工具名会被绕过，
// 而**被删掉的东西不会因为工具改名就变得可以恢复**。
//
// 清单分两类，因为它们该在**不同时机**被拒：
//
//	Protected  不可恢复的数据 —— 只在**写或删**时拒（读一份 schema.sql 是正常干活）
//	Sensitive  凭据 —— **任何时候**都拒（读走和删掉在这里同样不可逆）
//
// 把两者合成一张表会有很实际的代价：`**/*.sql` 一旦"任何操作都拒"，
// agent 连读迁移脚本都被拦，客户当天就会关掉执行点。
var (
	// DefaultProtected 是不可恢复的目标。
	//
	// 刻意**不用** `*prod*` 这类子串形式：它会把 product_service.go 一起拦掉。
	// 生产环境按**路径段**匹配（`**/prod/**`），只认整段等于 prod 的那种。
	DefaultProtected = []string{
		// —— 备份与转储：PocketOS 那类事故里，被删掉的正是这个 ——
		"**/backup/**", "**/backups/**",
		"**/*.bak", "**/*.dump", "**/*.sql",
		// —— 数据库的数据目录 ——
		"**/pgdata/**", "**/postgres/**", "**/mysql/**", "**/mongodb/**",
		// —— 生产环境（按路径段，不是子串）——
		"**/prod/**", "**/production/**", "**/.env.production", "**/*.production",
		// —— 版本控制：删掉 .git 等于删掉全部历史，本地没有第二份 ——
		"**/.git", "**/.git/**",
		// —— 系统关键路径 ——
		"/etc/**", "/boot/**", "/var/lib/**", "/usr/**", "/bin/**", "/sbin/**",
		// —— 根与家目录本身：`rm -rf /` 与 `rm -rf ~` 是终局动作 ——
		"/", "/*", "~",
	}

	// DefaultSafeTargets 是**明确安全**的破坏目标：命中的破坏性调用不再升级为人工确认。
	//
	// 为什么必须留这一项：如果 agent 连删 node_modules、清 build 目录都要人点确认，
	// 客户会在第一天就把执行点关掉——那时保护强度归零，比有例外更糟。
	// 这里只放"能重建、且重建成本低"的东西；`dist` 之类有交付风险的留给客户自己调。
	DefaultSafeTargets = []string{
		"**/node_modules", "**/node_modules/**",
		"**/__pycache__/**", "**/*.pyc",
		"**/.pytest_cache/**", "**/.mypy_cache/**", "**/.ruff_cache/**",
		"**/coverage/**", "**/.cache/**",
		"**/build/**", "**/target/**",
		"/tmp/**", "/var/tmp/**", "**/tmp/**",
	}
)

// GuardActions 是执行点允许的动作取值。
const (
	// GuardAsk 交给人确认（agent 会收到"需要批准"）。
	GuardAsk = "ask"
	// GuardDeny 直接拒绝。
	GuardDeny = "deny"
	// GuardAllow 直接放行（只记录）。
	GuardAllow = "allow"
)

// DefaultGuard 是编译策略时写进去的出厂拦截配置。
//
// 默认值取向：**破坏性的东西默认要人看一眼，不可恢复的东西直接拒。**
// 这两个默认值合起来才是"防误删"——只有 deny 会把 agent 变成废物，
// 只有 ask 会在没人看屏幕的时候（CI、无人值守）被自动批准。
func DefaultGuard() *model.Guard {
	return &model.Guard{
		Enabled:   true,
		OnApprove: GuardAsk,
		OnUnknown: GuardDeny,
		Protected: append([]string(nil), DefaultProtected...),
		// 凭据清单直接引用审计侧那一份：同一个 .env 在策略里和在执行点
		// 必须是同一套说法，各写一份迟早会漂移。
		Sensitive: append([]string(nil), SensitivePathDeny...),
		// SafeTargets 默认留空：出厂时不让执行点替客户做"什么算安全"的判断。
		// 把 DefaultSafeTargets 打开是一次明确的取舍，客户自己决定。
	}
}
