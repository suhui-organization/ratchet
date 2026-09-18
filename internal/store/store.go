// Package store 是调用记录的本地存放处：一行一次调用，只追加。
//
// 为什么是 JSONL：hook 是短命进程，每次调用都要能独立、原子地写一行。
// 没有数据库、没有锁、没有后台服务——被 agent 反复调用数十万次也不会成为瓶颈。
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// EnvHome 覆盖默认存放目录（测试与多环境用）。
const EnvHome = "RATCHET_HOME"

// DefaultHome 返回默认的数据目录：$RATCHET_HOME 或 ~/.ratchet。
func DefaultHome() string {
	if v := os.Getenv(EnvHome); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ratchet"
	}
	return filepath.Join(home, ".ratchet")
}

// CallsPath 是默认的调用记录文件。
func CallsPath() string { return filepath.Join(DefaultHome(), "calls.jsonl") }

// Append 追加一条调用记录，返回写入的路径。
//
// 用 O_APPEND 打开：多个 hook 进程同时写时，每次写入是原子的，
// 不会出现两行互相插进对方中间导致的坏行。
func Append(path string, call observe.Call) error {
	if path == "" {
		path = CallsPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if call.TS == "" {
		call.TS = time.Now().UTC().Format(time.RFC3339)
	}
	line, err := json.Marshal(call)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}
