package policy

import (
	"strings"

	"github.com/suhui-organization/ratchet/internal/model"
)

// Capability 是工具的能力类别。判定顺序即危险度顺序：
// 一个名字同时命中多类时，**取更危险的那一类**（不允许"既能读又能删"被判成读）。
type Capability string

const (
	CapDestructive Capability = "destructive"
	CapExecute     Capability = "execute"
	CapWrite       Capability = "write"
	CapNetwork     Capability = "network"
	CapRead        Capability = "read"
	CapUnknown     Capability = "unknown"
)

// keywordSets 按危险度从高到低排列，先命中者胜。
var keywordSets = []struct {
	cap   Capability
	words []string
}{
	{CapDestructive, []string{
		"delete", "remove", "drop", "destroy", "purge", "truncate", "wipe", "erase",
		"unlink", "rm", "rmdir", "revoke", "clear", "reset", "overwrite", "terminate",
		"kill", "format", "prune", "uninstall",
	}},
	{CapExecute, []string{
		"exec", "execute", "run", "shell", "bash", "sh", "zsh", "command", "cmd",
		"spawn", "eval", "system", "terminal", "script", "process",
	}},
	{CapWrite, []string{
		"write", "create", "update", "edit", "modify", "put", "patch", "move",
		"rename", "mkdir", "touch", "append", "insert", "save", "upload", "push",
		"commit", "send", "publish", "deploy", "install", "set", "add", "copy",
	}},
	{CapNetwork, []string{
		"http", "https", "url", "uri", "fetch", "curl", "request", "webhook",
		"browse", "scrape", "crawl", "download", "web", "remote", "api",
	}},
	{CapRead, []string{
		"read", "get", "list", "search", "find", "query", "show", "view", "stat",
		"glob", "grep", "cat", "head", "tail", "info", "status", "ls", "dir",
		"describe", "inspect", "peek", "load", "snapshot", "diff", "compare", "count",
		"return", "fetch",
	}},
}

// tokenize 把工具名/描述拆成小写词元。
//
// 同时处理 snake_case、kebab-case、点号、空格与 camelCase：
// `readFile`、`read_file`、`read-file`、`filesystem.read` 都要拆成同样的词。
// 拆不出来就会出现"名字里明明有 delete 却没被判成破坏性"这类静默漏判。
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	var withSpaces strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			withSpaces.WriteByte(' ')
		}
		withSpaces.WriteRune(r)
	}
	fields := strings.FieldsFunc(strings.ToLower(withSpaces.String()), func(r rune) bool {
		switch r {
		case '_', '-', '.', '/', ' ', ':', ',', '(', ')', '[', ']', '"', '\'':
			return true
		}
		return false
	})
	return fields
}

// match 在词表里找命中词。
//
// 英文描述里同一个概念有词形变化（remove / removes / removed / removing），
// 只做整词相等会漏掉绝大多数**描述**——实测就是这么漏的。
// 所以长词走前缀匹配（`delete` 命中 `deleted`、`deletion`），
// 短词（<=2 字符，如 `sh`）只做精确匹配，否则 `shell` 会被 `sh` 误命中。
//
// 前缀匹配的代价是少量过宽（`settings` 会命中 `set`）。这是刻意接受的：
// 过宽只会把工具推到 approve（更严），过窄则会把危险工具放行（更松）。
func match(tokens []string, words []string) (string, bool) {
	for _, tok := range tokens {
		for _, w := range words {
			if tok == w {
				return w, true
			}
			if len(w) >= 3 && len(tok) > len(w) && strings.HasPrefix(tok, w) {
				return w, true
			}
		}
	}
	return "", false
}

// Classify 判定一个工具的能力类别，并给出**人类可读的依据**。
//
// 先看名字，名字判不出来再看描述。两者都判不出来才算未知。
// 依据（reason）会写进策略文件：用户要能看懂"为什么这条被收紧了"。
func Classify(tool model.ToolObservation) (Capability, string) {
	nameTokens := tokenize(tool.Tool)
	for _, set := range keywordSets {
		if w, ok := match(nameTokens, set.words); ok {
			return set.cap, "名称命中「" + w + "」"
		}
	}
	descTokens := tokenize(tool.Description)
	for _, set := range keywordSets {
		if w, ok := match(descTokens, set.words); ok {
			return set.cap, "描述命中「" + w + "」"
		}
	}
	return CapUnknown, "名称与描述都未命中能力词表"
}

// Decide 把能力类别映射成判定。
//
// 映射规则（与产品对外的说法一一对应）：
//
//	破坏性 → deny        破坏不可逆，默认拒绝
//	执行   → approve     能跑任意命令，必须有人看着
//	写入   → approve     会改状态
//	出网   → approve     能外发数据
//	读取   → allow       只读，放行才有可用性
//	未知   → approve     见 D5：deny 会让第一份策略直接不可用；
//	                     approve 同样是 fail-closed（不能无人值守执行），
//	                     同时进 needsReview 清单等人确认
//
// strictUnknown 时未知降为 deny（`--strict-unknown`）。
func Decide(cap Capability, strictUnknown bool) (model.Decision, bool) {
	switch cap {
	case CapDestructive:
		return model.Deny, false
	case CapExecute, CapWrite, CapNetwork:
		return model.Approve, false
	case CapRead:
		return model.Allow, false
	default:
		if strictUnknown {
			return model.Deny, false
		}
		return model.Approve, true
	}
}

// CapabilityOfDecision 反向标注：给策略里已有的判定配一句说明（用于报告）。
func CapabilityOfDecision(d model.Decision) string {
	switch d {
	case model.Deny:
		return string(CapDestructive)
	case model.Approve:
		return string(CapWrite)
	case model.Allow:
		return string(CapRead)
	}
	return string(CapUnknown)
}
