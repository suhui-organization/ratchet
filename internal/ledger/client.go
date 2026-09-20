// Package ledger 是控制面台账的只读/写客户端（ASAS-8.3 / 8.5）。
//
// 为什么这一层要单独存在：应急的时候要问的问题只有几个——"谁能碰 X"、
// "把这个人拿下会连累谁"。这两个问题的答案都在控制面，所以这里只做传输与解码，
// **不在这里重新实现判定**（和 asas.py 的纪律一致：规则只有一份）。
package ledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 连一个 ASAS 控制面。
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New 建客户端。超时默认 15 秒：应急时"卡住"比"失败"更糟——失败至少能换个办法。
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Grant 是一条"某 agent 对某资产有判定"的记录。
type Grant struct {
	Org           string `json:"org"`
	Agent         string `json:"agent"`
	Verdict       string `json:"verdict"`
	Basis         string `json:"basis"`
	AttestationID string `json:"attestationId"`
	StoredFor     string `json:"storedFor"`
	Contained     bool   `json:"contained"`
	Chain         []struct {
		Parent  string `json:"parent"`
		Child   string `json:"child"`
		Expires string `json:"expires"`
	} `json:"chain"`
}

// Observed 是"确实被看到用过"的记录（授权面之外的另一个面）。
type Observed struct {
	Org           string `json:"org"`
	Tool          string `json:"tool"`
	Count         int    `json:"count"`
	AttestationID string `json:"attestationId"`
}

// ReachAnswer 是 GET /reach 的答复。
//
// Unanswerable 非空时**不要**把 Granted 为空读成"没人能碰"：那是"台账没覆盖"。
// 两个结论对应急响应完全不同，所以这里保留原文而不再加工。
type ReachAnswer struct {
	Subject      string     `json:"subject"`
	Granted      []Grant    `json:"granted"`
	Denied       []Grant    `json:"denied"`
	Observed     []Observed `json:"observed"`
	Contained    []string   `json:"contained"`
	Unanswerable []string   `json:"unanswerable"`
}

// ContainResult 是 POST /contain 的答复。
type ContainResult struct {
	Agent      string `json:"agent"`
	Reason     string `json:"reason"`
	RecordedAt string `json:"recordedAt"`
	DryRun     bool   `json:"dryRun"`
	Contained  []string
	Cascaded   []struct {
		Agent string `json:"agent"`
		Via   string `json:"via"`
		Depth int    `json:"depth"`
	} `json:"cascaded"`
}

// ContainmentState 是 GET /contain：遏制记录 + 完整的委派图。
type ContainmentState struct {
	Containment []struct {
		Agent      string `json:"agent"`
		Via        string `json:"via"`
		Reason     string `json:"reason"`
		RecordedAt string `json:"recordedAt"`
	} `json:"containment"`
	Graph []struct {
		Parent string `json:"parent"`
		Child  string `json:"child"`
		Org    string `json:"org"`
	} `json:"graph"`
}

func (c *Client) get(path string, out any) error {
	resp, err := c.HTTP.Get(c.BaseURL + path)
	if err != nil {
		return fmt.Errorf("控制面不可达（%s）：%w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	return decode(resp, out)
}

func (c *Client) post(path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Post(c.BaseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("控制面不可达（%s）：%w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	return decode(resp, out)
}

func decode(resp *http.Response, out any) error {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var problem struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &problem) == nil && problem.Error != "" {
			return fmt.Errorf("控制面返回 %d：%s", resp.StatusCode, problem.Error)
		}
		return fmt.Errorf("控制面返回 %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("控制面返回的不是预期 JSON：%w", err)
	}
	return nil
}

// Reach 回答"谁曾能触达 subject"。
func (c *Client) Reach(subject string) (ReachAnswer, error) {
	var answer ReachAnswer
	err := c.get("/reach?subject="+url.QueryEscape(subject), &answer)
	return answer, err
}

// Contain 吊销一个 agent（默认级联到它的下级）。dryRun=true 只拿名单，不动手。
func (c *Client) Contain(agent, reason string, dryRun bool) (ContainResult, error) {
	var result ContainResult
	err := c.post("/contain", map[string]any{"agent": agent, "reason": reason, "dryRun": dryRun}, &result)
	return result, err
}

// State 取遏制记录与委派图。
func (c *Client) State() (ContainmentState, error) {
	var state ContainmentState
	err := c.get("/contain", &state)
	return state, err
}
