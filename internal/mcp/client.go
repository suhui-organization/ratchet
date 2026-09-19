// Package mcp 是一个只做一件事的最小 MCP 客户端：连上一个 stdio server，
// 问它有哪些工具，然后关掉。
//
// **这个包会执行命令**（启动配置里写的 MCP server）。因此它只能被
// `ratchet scan --introspect` 这样显式开启的路径调用，绝不作为默认行为。
//
// 静态配置只能告诉我们"有哪些 server"；工具名必须问 server 才知道。
// 没有工具名，策略编译就没有输入——这就是为什么需要这一步。
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// ClientVersion 是握手时告诉对端 server 的自身版本。
// 与 mcpserver.ServerVersion 同理：写死的版本号会漂移（这里曾写死 0.1.0），
// 由 main 注入唯一的那一份版本。
var ClientVersion = "dev"

// Tool 是 server 暴露的一个工具。
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Options 控制一次 introspect。
type Options struct {
	Command string
	Args    []string
	// Env 为空时继承当前进程环境（harness 平时就是这么跑 server 的）。
	Env []string
	// Timeout 是整体上限：连"启动 + 握手 + 列工具"一起算。
	Timeout time.Duration
}

// ProtocolVersion 用 2024-11-05：这是各家 server 实现覆盖最广的一版。
const ProtocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ListTools 启动 server、握手、取回工具列表，最后一定把它关掉。
//
// 失败时返回错误而不是空列表：**"连不上"和"这个 server 没有工具"是两件事**，
// 混在一起会让使用者以为某个 server 是干净的。
func ListTools(opts Options) ([]Tool, error) {
	if opts.Command == "" {
		return nil, errors.New("server 没有 command（远程 server 暂不支持 introspect）")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// server 的日志走 stderr，不能混进协议通道；这里直接丢弃，
	// 免得把别人的日志当成我们的输出转述给用户。
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 server 失败：%w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	lines := make(chan string, 64)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		close(lines)
	}()

	send := func(req rpcRequest) error {
		raw, err := json.Marshal(req)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdin, "%s\n", raw)
		return err
	}

	// 1) initialize
	if err := send(rpcRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "ratchet", "version": "0.1.0"},
		},
	}); err != nil {
		return nil, fmt.Errorf("发送 initialize 失败：%w", err)
	}
	if _, err := await(ctx, lines, 1); err != nil {
		return nil, fmt.Errorf("initialize 没有成功：%w", err)
	}

	// 2) 握手完成通知（通知没有 id，不期待响应）
	if err := send(rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return nil, err
	}

	// 3) tools/list
	if err := send(rpcRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list", Params: map[string]any{}}); err != nil {
		return nil, fmt.Errorf("发送 tools/list 失败：%w", err)
	}
	raw, err := await(ctx, lines, 2)
	if err != nil {
		return nil, fmt.Errorf("tools/list 没有成功：%w", err)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("tools/list 返回的结构不认识：%w", err)
	}
	return result.Tools, nil
}

// await 等待指定 id 的响应；期间遇到的通知、日志、无关响应一律跳过。
func await(ctx context.Context, lines <-chan string, id int) (json.RawMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("超时")
		case line, ok := <-lines:
			if !ok {
				return nil, errors.New("server 提前退出")
			}
			var resp rpcResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				continue // 不是 JSON 的行直接跳过（有些 server 会往 stdout 打字）
			}
			if resp.ID == nil || *resp.ID != id {
				continue
			}
			if resp.Error != nil {
				return nil, fmt.Errorf("server 返回错误 %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, nil
		}
	}
}
