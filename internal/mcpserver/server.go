// Package mcpserver 把 ratchet 暴露成一个 stdio MCP server。
//
// 为什么值得单独做一个 server：官方 MCP Registry 只收 **server**，
// 而它是唯一会级联的目录（发布一次同步到 Smithery / PulseMCP / GitHub Registry）。
// 同时它也让 agent 自己能问"我现在能碰什么"，不必让人记命令。
//
// 只读：三个工具都不改任何状态，也不执行 MCP server 之外的进程。
package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/suhui-organization/ratchet/internal/discover"
	"github.com/suhui-organization/ratchet/internal/model"
	"github.com/suhui-organization/ratchet/internal/policy"
)

const protocolVersion = "2024-11-05"

// ServerVersion 是 initialize 响应里报出去的版本号。
//
// 为什么要做成可注入的：这里原先硬编码 "0.9.0"，而二进制已经是 0.12.0——
// 一个讲"可验证"的工具对外报了一个和实际不符的版本号，这比缺功能更伤信任
// （客户端、目录、报告都会引用它）。现在由 main 注入 cmd/ratchet 里那份唯一的版本，
// 两处再也不可能各说各话。
var ServerVersion = "dev"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// tools 是暴露的三个工具。名字统一带 ratchet_ 前缀，避免在 agent 的工具列表里重名。
func tools() []toolDef {
	str := func(desc string) map[string]any {
		return map[string]any{"type": "object", "properties": map[string]any{
			"value": map[string]any{"type": "string", "description": desc},
		}, "required": []string{"value"}}
	}
	_ = str
	return []toolDef{
		{
			Name: "ratchet_scan",
			Description: "List the MCP servers configured on this machine and flag the ones that " +
				"do not pin a version. Read-only: it parses config files and executes nothing.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"home": map[string]any{"type": "string", "description": "Directory to scan (default: the current user's home)"},
			}},
		},
		{
			Name: "ratchet_policy",
			Description: "Summarise a least-privilege policy file: how many tools are allowed, " +
				"require approval, or are denied, plus the reason behind each verdict.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"policy": map[string]any{"type": "string", "description": "Path to a policy JSON file"},
			}, "required": []string{"policy"}},
		},
		{
			Name: "ratchet_check",
			Description: "Decide whether one hypothetical tool call would be allowed, denied, or " +
				"need approval under a policy — including parameter-level constraints.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"policy": map[string]any{"type": "string"},
				"server": map[string]any{"type": "string"},
				"tool":   map[string]any{"type": "string"},
				"args":   map[string]any{"type": "object", "description": "Arguments to test"},
			}, "required": []string{"policy", "server", "tool"}},
		},
	}
}

// Serve 在 stdin/stdout 上跑一个最小的 MCP server，直到 stdin 关闭。
//
// stdout 只走协议（一行一个 JSON-RPC）；任何诊断都写 stderr——
// 混一行日志进 stdout，客户端就会断连。
func Serve(in io.Reader, out io.Writer, errw io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue // 不是 JSON 就不理它，别让一行噪音断掉连接
		}
		if req.ID == nil {
			continue // 通知不需要响应
		}
		result, rpcErr := dispatch(req.Method, req.Params)
		resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
		if rpcErr != nil {
			resp["error"] = map[string]any{"code": -32603, "message": rpcErr.Error()}
		} else {
			resp["result"] = result
		}
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintf(errw, "ratchet mcp: 写响应失败: %v\n", err)
			return err
		}
	}
	return scanner.Err()
}

func dispatch(method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "ratchet", "version": ServerVersion},
		}, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		return callTool(params)
	case "ping":
		return map[string]any{}, nil
	}
	return nil, fmt.Errorf("未知方法：%s", method)
}

func callTool(params json.RawMessage) (any, error) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	text, err := runTool(p.Name, p.Arguments)
	if err != nil {
		// 工具级错误按 MCP 约定返回 isError，而不是 JSON-RPC error——
		// 后者会让客户端以为整个连接坏了
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}, nil
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
	}, nil
}

func runTool(name string, args map[string]any) (string, error) {
	switch name {
	case "ratchet_scan":
		home, _ := args["home"].(string)
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		rep := discover.Scan(home, ".")
		var b strings.Builder
		servers := 0
		for _, h := range rep.Harnesses {
			servers += len(h.Servers)
		}
		fmt.Fprintf(&b, "harnesses=%d servers=%d unpinned=%d\n",
			len(rep.Harnesses), servers, len(rep.Unpinned()))
		for _, h := range rep.Harnesses {
			for _, s := range h.Servers {
				if s.Risk != "" {
					fmt.Fprintf(&b, "  UNPINNED %s/%s\n", h.ID, s.Name)
				}
			}
		}
		if len(rep.Unparsed) > 0 {
			fmt.Fprintf(&b, "unparsed configs: %d (reported, not guessed)\n", len(rep.Unparsed))
		}
		return b.String(), nil

	case "ratchet_policy":
		p, err := loadPolicy(args)
		if err != nil {
			return "", err
		}
		allow, approve, deny := 0, 0, 0
		for _, r := range p.Servers {
			allow += len(r.Allow)
			approve += len(r.Approve)
			deny += len(r.Deny)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "allow=%d approve=%d deny=%d default=%s\n", allow, approve, deny, p.DefaultDecision)
		if len(p.NeedsReview) > 0 {
			fmt.Fprintf(&b, "needs review: %s\n", strings.Join(p.NeedsReview, ", "))
		}
		for key := range p.Rationale {
			fmt.Fprintf(&b, "  %s: %s\n", key, p.Rationale[key])
		}
		return b.String(), nil

	case "ratchet_check":
		p, err := loadPolicy(args)
		if err != nil {
			return "", err
		}
		server, _ := args["server"].(string)
		tool, _ := args["tool"].(string)
		if server == "" || tool == "" {
			return "", fmt.Errorf("需要 server 与 tool")
		}
		argMap := map[string]string{}
		if raw, ok := args["args"].(map[string]any); ok {
			for k, v := range raw {
				if s, ok := v.(string); ok {
					argMap[k] = s
				}
			}
		}
		three := policy.Verdict(p, server, tool)
		decision, violations := policy.Evaluate(p, server, tool, argMap)
		var b strings.Builder
		fmt.Fprintf(&b, "decision=%s (three-state=%s)\n", decision, three)
		for _, v := range violations {
			fmt.Fprintf(&b, "  violation: %s=%q %s (rule %s)\n", v.ArgKey, v.Value, v.Reason, v.Rule)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("未知工具：%s", name)
}

func loadPolicy(args map[string]any) (model.Policy, error) {
	var p model.Policy
	path, _ := args["policy"].(string)
	if path == "" {
		return p, fmt.Errorf("需要 policy 参数（策略文件路径）")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, fmt.Errorf("读策略失败：%w", err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, fmt.Errorf("策略不是合法 JSON：%w", err)
	}
	return p, nil
}
