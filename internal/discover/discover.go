// Package discover 在**不执行任何东西**的前提下，找出这台机器上有哪些 agent，
// 以及它们各自挂了哪些 MCP server。
//
// 两条纪律：
//
//  1. **只读配置**。本包不 spawn 任何进程、不连任何网络——连接 server 是
//     `internal/mcp` 的事，且必须由用户显式开启。
//  2. **环境变量只记键名，不记值**。MCP 配置里的 env 值通常就是密钥，
//     把值读进结构体再写进报告，等于自己制造一次泄露。
package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Server 是配置里的一条 MCP server 定义。
type Server struct {
	Name    string   `json:"name"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
	// EnvKeys 只记键名，值一律不读进结构体。
	EnvKeys []string `json:"envKeys,omitempty"`
	// Source 是命中的配置文件（报告里要能追回"这是从哪读到的"）。
	Source string `json:"source"`
	// Risk 是确定性判定的风险标记（例如 npx 拉未锁版本的包）。
	Risk string `json:"risk,omitempty"`
}

// Harness 是一个 agent 运行环境。
type Harness struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Configs 是命中的配置文件路径。
	Configs []string `json:"configs"`
	// Parsed 为 false 表示"发现了这个 harness，但我们不认识它的配置格式"。
	// 这种情况必须显式报告——静默漏掉等于给出虚假的安全感。
	Parsed  bool     `json:"parsed"`
	Servers []Server `json:"servers"`
}

// Report 是一次扫描的结果。
type Report struct {
	Home      string    `json:"home"`
	Workdir   string    `json:"workdir"`
	Harnesses []Harness `json:"harnesses"`
	// Unparsed 列出"文件存在但格式不认识/解析失败"的配置。
	Unparsed []string `json:"unparsed,omitempty"`
}

// probe 描述"去哪里找、用什么格式读"。
type probe struct {
	id      string
	name    string
	path    string // 相对 home；inCwd=true 时相对 workdir
	inCwd   bool
	format  string // "json" | "toml"
	dialect string // 解析方言；空表示只探测存在、不解析
}

// probes 是探测表。
//
// 只收录我们**确定**格式的方言。不确定的（各家 IDE 私有格式）宁可标成
// "发现了但没解析"，也不要猜——猜错会给出错误的能力面，比漏报更糟。
var probes = []probe{
	{id: "claude-code", name: "Claude Code", path: ".claude.json", format: "json", dialect: "mcpServers"},
	{id: "claude-code", name: "Claude Code", path: ".claude/settings.json", format: "json", dialect: "mcpServers"},
	{id: "cursor", name: "Cursor", path: ".cursor/mcp.json", format: "json", dialect: "mcpServers"},
	{id: "windsurf", name: "Windsurf", path: ".codeium/windsurf/mcp_config.json", format: "json", dialect: "mcpServers"},
	{id: "codex", name: "Codex", path: ".codex/config.toml", format: "toml", dialect: "mcp_servers"},
	{id: "vscode", name: "VS Code", path: ".config/Code/User/mcp.json", format: "json", dialect: "mcpServers"},
	// 项目级配置：最容易被忽略的一类，也是"打开工作区就自动生效"的那一类
	{id: "project", name: "项目级配置", path: ".mcp.json", inCwd: true, format: "json", dialect: "mcpServers"},
	{id: "project", name: "项目级配置", path: ".cursor/mcp.json", inCwd: true, format: "json", dialect: "mcpServers"},
	{id: "project", name: "项目级配置", path: ".vscode/mcp.json", inCwd: true, format: "json", dialect: "mcpServers"},
	// 只探测存在、不解析
	{id: "zed", name: "Zed", path: ".config/zed/settings.json", format: "json", dialect: ""},
}

// Scan 读配置，产出报告。它**不会**执行配置里的任何命令。
func Scan(home, workdir string) Report {
	report := Report{Home: home, Workdir: workdir}
	byID := map[string]*Harness{}
	var order []string

	for _, p := range probes {
		base := home
		if p.inCwd {
			base = workdir
		}
		full := filepath.Join(base, p.path)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		h, ok := byID[p.id]
		if !ok {
			h = &Harness{ID: p.id, Name: p.name, Parsed: true}
			byID[p.id] = h
			order = append(order, p.id)
		}
		h.Configs = append(h.Configs, full)

		if p.dialect == "" {
			// 发现了但没解析：必须标成 parsed=false。否则报告会让人以为
			// "这个 harness 没有任何 server"——那是虚假的安全感。
			h.Parsed = false
			report.Unparsed = append(report.Unparsed, full)
			continue
		}
		servers, err := parseConfig(full, p.format, p.dialect)
		if err != nil {
			report.Unparsed = append(report.Unparsed, full+"（解析失败："+err.Error()+"）")
			h.Parsed = false
			continue
		}
		h.Servers = append(h.Servers, servers...)
	}

	sort.Strings(order)
	for _, id := range order {
		h := byID[id]
		sort.Slice(h.Servers, func(i, j int) bool { return h.Servers[i].Name < h.Servers[j].Name })
		report.Harnesses = append(report.Harnesses, *h)
	}
	return report
}

// parseConfig 按方言取 server 列表。
func parseConfig(path, format, dialect string) ([]Server, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if format == "toml" {
		if err := toml.Unmarshal(raw, &root); err != nil {
			return nil, err
		}
	} else if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	node, _ := root[dialect].(map[string]any)
	servers := make([]Server, 0, len(node))
	for name, value := range node {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		server := Server{Name: name, Source: path}
		server.Command = stringOf(entry["command"])
		server.URL = stringOf(entry["url"])
		server.Args = stringsOf(entry["args"])
		// 只取键名：值可能是密钥
		if env, ok := entry["env"].(map[string]any); ok {
			for key := range env {
				server.EnvKeys = append(server.EnvKeys, key)
			}
			sort.Strings(server.EnvKeys)
		}
		server.Risk = assessRisk(server)
		servers = append(servers, server)
	}
	return servers, nil
}

// assessRisk 做**确定性**的风险标记，不做猜测。
//
// 目前只判一类：用 npx/uvx 拉包却没锁版本。这是供应链投毒最常见的入口，
// 而且判定完全确定——看得到就是看得到。
func assessRisk(s Server) string {
	base := filepath.Base(s.Command)
	switch base {
	case "npx", "uvx", "bunx", "pnpx":
	default:
		return ""
	}
	for _, arg := range s.Args {
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, ".") {
			continue
		}
		if pinned(arg) {
			continue
		}
		return "依赖未锁版本（" + arg + "）：同名包被替换时无法察觉"
	}
	return ""
}

// pinned 判断一个包参数是否锁定了具体版本。
//
// 真机数据逼出来的规则：`@latest` 看着像带版本，实际是"每次装都拉最新"——
// 它比不写版本号还危险，因为它看起来像是被钉住了。
func pinned(arg string) bool {
	idx := strings.LastIndex(arg, "@")
	if idx <= 0 {
		return false // 例如 "firecrawl-mcp"，没有任何版本
	}
	version := arg[idx+1:]
	switch version {
	case "", "latest", "next", "beta", "canary":
		return false
	}
	for _, op := range []string{"^", "~", ">", "<", "*", "x"} {
		if strings.HasPrefix(version, op) {
			return false
		}
	}
	return true
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

func stringsOf(v any) []string {
	if ss, ok := v.([]string); ok {
		return ss
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Unpinned 返回所有未锁版本依赖的 server 标识（报告摘要用）。
func (r Report) Unpinned() []string {
	var out []string
	for _, h := range r.Harnesses {
		for _, s := range h.Servers {
			if s.Risk != "" {
				out = append(out, h.ID+"/"+s.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}
