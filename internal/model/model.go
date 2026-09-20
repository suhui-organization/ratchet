// Package model 定义 Ratchet 的两份契约：输入（工具清单）与输出（最小权限策略）。
//
// 契约单独成一个包，是因为它们要跨语言用：Go 侧产出策略，
// Python 侧（交付物与验证器）要读懂同一份结构。
package model

// Decision 是策略对一次工具调用的三态判定。
//
// 语义必须与网关一致：Deny 优先于 Approve，Approve 优先于 Allow。
type Decision string

const (
	Allow   Decision = "allow"
	Approve Decision = "approve"
	Deny    Decision = "deny"
)

// ToolObservation 是一个 agent 能够使用的工具。
//
// Calls > 0 表示这个工具是从**真实调用**里观测到的；为 0 表示它只是
// 静态清单里存在（有能力，但没被用过）。两者的可信度不同，策略文件里会区分。
type ToolObservation struct {
	Server      string `json:"server"`
	Tool        string `json:"tool"`
	Description string `json:"description,omitempty"`
	Calls       int    `json:"calls,omitempty"`
	// ArgKeys 是这个工具被调用时出现过的**参数名**（如 path、command）。
	//
	// 只记键名，不记值：键名不是秘密，值通常是文件内容、命令行或令牌。
	// 有了键名才知道"这个工具要传路径"，才谈得上参数级约束。
	ArgKeys []string `json:"argKeys,omitempty"`
}

// Inventory 是策略编译的输入。
//
// 当前由用户/上游扫描器提供（见 examples/inventory.json）；
// 下一阶段由 `ratchet scan` 直接生成。
type Inventory struct {
	Format string            `json:"format"`
	Agent  string            `json:"agent,omitempty"`
	Tools  []ToolObservation `json:"tools"`
	// Servers 是 server 级事实（ASAS-3.1/3.2 的输入）：定版状态、被声明的包引用、来源配置。
	// 只记这些元数据，不记任何环境变量值。
	Servers []ServerFact `json:"servers,omitempty"`
}

// ServerFact 是扫描侧对一条 MCP server 定义观察到的事实。
//
// Pinned 为 nil 表示"不适用或未判定"——ASAS-A 里它必须显式声明为 unknown，
// 不允许悄悄当成"已定版"。
type ServerFact struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	Source  string `json:"source,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Pinned  *bool  `json:"pinned,omitempty"`
	Risk    string `json:"risk,omitempty"`
}

// ServerRules 是一个 MCP server 下的工具分组。
type ServerRules struct {
	Allow   []string `json:"allow"`
	Approve []string `json:"approve"`
	Deny    []string `json:"deny"`
	// Constraints 是参数级约束：工具名 → 约束。缺省表示这个工具只按三态判定。
	Constraints map[string]ToolConstraints `json:"constraints,omitempty"`
}

// ToolConstraints 约束一次调用的**参数**。
//
// 语义（fail-closed）：
//   - Paths.Deny 命中任一参数值 → 拒绝
//   - Paths.Allow 非空时，每个被识别为路径的值都必须命中其中之一，否则拒绝
//   - 识别不出路径的值不参与 allow 判定（无法判断就不假装判断得出来）
type ToolConstraints struct {
	Paths *PathRules `json:"paths,omitempty"`
}

// PathRules 是路径白/黑名单，用 glob（`*` 不跨目录、`**` 跨目录）。
type PathRules struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// Policy 是编译产物。
type Policy struct {
	Version string `json:"version"`
	Agent   string `json:"agent,omitempty"`
	// DefaultDecision 固定为 deny：未登记的一律拒绝（fail-closed）。
	DefaultDecision Decision               `json:"defaultDecision"`
	Servers         map[string]ServerRules `json:"servers"`
	// Rationale 是"每条判定为什么这么判"，key 为 "server/tool"。
	// 没有它，用户无法判断一份自动生成的策略是否可信——也就不会去维护它。
	Rationale map[string]string `json:"rationale"`
	// NeedsReview 列出判定依据不足、需要人确认的工具（当前策略把它们置为 approve）。
	NeedsReview []string `json:"needsReview"`
	GeneratedAt string   `json:"generatedAt,omitempty"`
	Generator   string   `json:"generator,omitempty"`
}

// Key 是工具在 Rationale / NeedsReview 里的键。
func Key(server, tool string) string { return server + "/" + tool }
