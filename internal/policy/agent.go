package policy

import (
	"os"
	"strings"
)

// DefaultAgent 是没给 --agent 时用的 agent 名：**短主机名**。
//
// 为什么不是字面量 "agent"：那个值对每一台机器都一样，等于没有身份——
// 而这份凭据的第一条要求就是 agent 可枚举、可追责（ASAS-1.1）。

// 主机名是手边最稳的一个"这台机器是谁"，并且在同一个 agent 反复交付时保持不变
// （跨次比对链摘要要用它当键：名字每跑一次换一次，比对就永远失效）。
//
// 取不到主机名时也不退化成 "agent"：写一个明确说"没识别出身份"的值，
// 让读的人知道这里缺东西，而不是读到一个看起来很正常的名字。
func DefaultAgent() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "unidentified-agent"
	}
	// 去掉域名后缀：kind-1.internal.example → kind-1
	if idx := strings.Index(host, "."); idx > 0 {
		host = host[:idx]
	}
	return host
}
