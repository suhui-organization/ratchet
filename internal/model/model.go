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
}

// Inventory 是策略编译的输入。
//
// 当前由用户/上游扫描器提供（见 examples/inventory.json）；
// 下一阶段由 `ratchet scan` 直接生成。
type Inventory struct {
	Format string            `json:"format"`
	Agent  string            `json:"agent,omitempty"`
	Tools  []ToolObservation `json:"tools"`
}

// ServerRules 是一个 MCP server 下的工具分组。
type ServerRules struct {
	Allow   []string `json:"allow"`
	Approve []string `json:"approve"`
	Deny    []string `json:"deny"`
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
