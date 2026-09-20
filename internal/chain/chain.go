// Package chain 把调用记录变成**可验证的事件流**（ASAS-5.3 / 6.6）。
//
// 为什么这一步不能省：只有 `calls.jsonl` 的时候，"这段时间没有异常调用"是一句
// 谁都能改的话——把文件删几行、改几行，没人看得出来。加一条哈希链之后，
// 改动会让下一个事件的前序哈希对不上，**断流本身就成了事件**。
//
// 纪律（与其它包一致）：
//
//	判定逻辑只有一份。**这里只负责造链，不负责验链**——验证在 Python 的
//	asas.rule_silence_auditable 里，那是收货方会用的同一份实现。
//	本包只提供 Digest()，用于造链时算下一个哈希。
//
// 哈希口径必须与验证器逐字一致：对**去掉 hash 字段后的事件**做 JSON 序列化，
// 键按字典序、无多余空格（Go 的 map 序列化天然如此），再取 sha256。
package chain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/suhui-organization/ratchet/internal/observe"
)

// Event 是事件流里的一条。seq 从 0 开始，单调 +1；prevHash 指向上一条的 hash。
//
// **只放元数据**：工具名、判定、参数名。参数值绝不进来——这是 ASAS-5.2，
// 由结构保证而不是靠自觉（这里压根没有放值的地方）。
type Event struct {
	Seq      int      `json:"seq"`
	PrevHash string   `json:"prevHash"`
	TS       string   `json:"ts,omitempty"`
	Agent    string   `json:"agent,omitempty"`
	Server   string   `json:"server,omitempty"`
	Tool     string   `json:"tool,omitempty"`
	Decision string   `json:"decision,omitempty"`
	Outcome  string   `json:"outcome,omitempty"`
	ArgKeys  []string `json:"argKeys,omitempty"`
	Hash     string   `json:"hash,omitempty"`
}

// Digest 是验证器会重算的那个值。
//
// 实现上先把事件序列化再解回 map，是为了**不依赖结构体字段顺序**：
// 验证器（Python）是把 JSON 解成 dict 再 sort_keys 重算的，两边必须一致。
func Digest(event Event) (string, error) {
	event.Hash = ""
	raw, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	delete(generic, "hash") // omitempty 已经在上面去掉了，这里再兜一次底
	canonical, err := json.Marshal(generic)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// Build 从调用记录造链。
//
// **顺序就是输入顺序**：不排序、不去重。原因很简单——链的意义是"这段时间里
// 发生过什么"，重排会让同一条调用在不同的跑次里得到不同的哈希，那这个摘要
// 就没法作为锚点被引用。输入顺序由 calls.jsonl 的追加顺序决定（O_APPEND 保证）。
func Build(calls []observe.Call, agent string) ([]Event, error) {
	events := make([]Event, 0, len(calls))
	prev := ""
	for i, call := range calls {
		event := Event{
			Seq:      i,
			PrevHash: prev,
			TS:       call.TS,
			Agent:    firstNonEmpty(call.Agent, agent),
			Server:   call.Server,
			Tool:     call.Tool,
			Decision: call.Decision,
			Outcome:  call.Outcome,
			ArgKeys:  call.ArgKeys,
		}
		digest, err := Digest(event)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条事件算不出哈希：%w", i, err)
		}
		event.Hash = digest
		prev = digest
		events = append(events, event)
	}
	return events, nil
}

// Head 是这条链的摘要（最后一条的 hash）与长度。它是"只增摘要"里被引用、被比对的那个值。
func Head(events []Event) (string, int) {
	if len(events) == 0 {
		return "", 0
	}
	return events[len(events)-1].Hash, len(events)
}

// WriteJSONL 写事件流（每行一条）。
func WriteJSONL(path string, events []Event) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	encoder := json.NewEncoder(w)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return w.Flush()
}

// ReadJSONL 读回事件流。坏行报出来而不是跳过——被吞掉的坏行会让断流看起来正常。
func ReadJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(text), &event); err != nil {
			return nil, fmt.Errorf("第 %d 行不是合法事件：%w", line, err)
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

// SortedTools 只是给输出用：让人一眼看到这条链覆盖了哪些工具。
func SortedTools(events []Event) []string {
	set := map[string]bool{}
	for _, event := range events {
		if event.Tool != "" {
			set[event.Tool] = true
		}
	}
	out := make([]string, 0, len(set))
	for tool := range set {
		out = append(out, tool)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
