package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/suhui-organization/ratchet/internal/guard"
	"github.com/suhui-organization/ratchet/internal/hook"
)

// allHarnesses 是当前支持的执行点目标。加新 agent 时把这里补上，
// 下面这些不变式就自动覆盖到它。
func allHarnesses() []hook.Harness {
	return []hook.Harness{
		hook.HarnessClaude, hook.HarnessCodex, hook.HarnessCodeBuddy,
		hook.HarnessQwen, hook.HarnessQoder, hook.HarnessGemini,
		hook.HarnessAntigravity, hook.HarnessCursor, hook.HarnessCrush,
		hook.HarnessDroid, hook.HarnessCline, hook.HarnessOpenCode,
		hook.HarnessGeneric,
	}
}

func verdict(d guard.Decision, rule, reason string) guard.Verdict {
	return guard.Verdict{Decision: d, Rule: rule, Reason: reason}
}

// 最重要的一条：**放行时绝不输出 allow。**
//
// 在 Claude Code / CodeBuddy 上，allow 的语义是"绕过权限系统直接执行"。
// 输出它等于替客户跳过他自己的确认——静默地降低了客户的安全姿态。
func TestAllowIsAlwaysSilent(t *testing.T) {
	for _, h := range allHarnesses() {
		e := emissionFor(h, verdict(guard.Allow, "policy.allow", "策略放行"))
		if e.Payload != nil {
			t.Fatalf("%s：放行时必须沉默，却输出了 %#v", h, e.Payload)
		}
		if e.ExitCode != 0 {
			t.Fatalf("%s：放行不该带退出码 %d", h, e.ExitCode)
		}
		if out, _ := json.Marshal(e); strings.Contains(string(out), `"allow"`) {
			t.Fatalf("%s：输出里出现了 allow —— 这会绕过客户自己的权限系统", h)
		}
	}
}

// 不支持 ask 的 agent 上，ask 必须降级成沉默，而不是硬拒绝。
//
// 降级成拒绝的后果是具体的：策略把"写入"判为 approve，于是每一次
// write_file 都会变成硬失败，客户当天就把执行点关掉。
func TestAskDegradesToSilenceOnAgentsWithoutAsk(t *testing.T) {
	for _, h := range []hook.Harness{
		hook.HarnessCodex, hook.HarnessGemini, hook.HarnessAntigravity,
		hook.HarnessCursor, hook.HarnessCrush, hook.HarnessCline, hook.HarnessOpenCode,
	} {
		e := emissionFor(h, verdict(guard.Ask, "policy.approve", "策略判为需人工审批"))
		if e.Payload != nil {
			t.Fatalf("%s 不支持 ask，不能输出 JSON（会被判成钩子失败并继续执行）：%#v", h, e.Payload)
		}
		if e.ExitCode == 2 {
			t.Fatalf("%s：ask 不该降级成阻断，那会把正常写入变成硬失败", h)
		}
		if e.Stderr == "" {
			t.Fatalf("%s：降级这件事必须说出来，不能静默", h)
		}
	}
}

// 支持 ask 的 agent 上，ask 要真的发出去——否则"停下来等人看"就没了。
func TestAskIsEmittedWhereSupported(t *testing.T) {
	for _, h := range []hook.Harness{
		hook.HarnessClaude, hook.HarnessCodeBuddy, hook.HarnessQwen, hook.HarnessQoder,
		hook.HarnessDroid,
	} {
		e := emissionFor(h, verdict(guard.Ask, "policy.approve", "需人工审批"))
		if e.Payload == nil {
			t.Fatalf("%s 支持 ask，却什么都没输出", h)
		}
		if !strings.Contains(mustJSON(t, e.Payload), `"ask"`) {
			t.Fatalf("%s 的输出里没有 ask：%s", h, mustJSON(t, e.Payload))
		}
	}
}

// 拒绝必须用**各家自己认识的形状**。
//
// 形状错了的表现是"静默不拦"：日志里有记录、看起来装上了，但 agent 没收到拒绝。
func TestDenyUsesEachAgentsOwnShape(t *testing.T) {
	cases := []struct {
		h    hook.Harness
		want string
	}{
		{hook.HarnessClaude, `"hookSpecificOutput"`},
		{hook.HarnessCodeBuddy, `"hookSpecificOutput"`},
		{hook.HarnessQwen, `"hookSpecificOutput"`},
		{hook.HarnessCodex, `"hookSpecificOutput"`},
		{hook.HarnessQoder, `"decision":"deny"`},
		{hook.HarnessGemini, `"decision":"block"`},
		{hook.HarnessAntigravity, `"decision":"block"`},
		{hook.HarnessCursor, `"permission":"deny"`},
		{hook.HarnessCrush, `"decision":"deny"`},
		{hook.HarnessDroid, `"hookSpecificOutput"`},
		{hook.HarnessCline, `"cancel":true`},
		{hook.HarnessOpenCode, `"decision":"deny"`},
	}
	for _, c := range cases {
		e := emissionFor(c.h, verdict(guard.Deny, "guard.protected-target", "拒绝：命中受保护目标"))
		if e.Payload == nil {
			t.Fatalf("%s：拒绝必须输出", c.h)
		}
		got := mustJSON(t, e.Payload)
		if !strings.Contains(got, c.want) {
			t.Fatalf("%s 的拒绝形状不对\n想要包含：%s\n实际：%s", c.h, c.want, got)
		}
	}
}

// **一律不用退出码阻断。**
//
// 这是踩过的一个真坑：Gemini CLI 认退出码 2，但被它取代的反重力 CLI
// 对非零退出码**只写日志、不阻断**——给 Gemini 写的"JSON + 退出码 2"
// 搬到反重力上就成了"看起来装了、其实拦不住"。
// 两家都认 stdout 的 JSON，所以统一走退出码 0 + JSON 一份输出喂两家。
//
// 另外两个退出码陷阱也一并钉住：Qwen 的退出码 2 会**丢弃 stdout**（理由就没了），
// 而 Crush 一类的 agent 里 allow 会跳过用户确认。
func TestDenyNeverUsesExitCodes(t *testing.T) {
	for _, h := range allHarnesses() {
		e := emissionFor(h, verdict(guard.Deny, "guard.protected-target", "拒绝：命中受保护目标"))
		if e.ExitCode != 0 {
			t.Fatalf("%s 的拒绝用了退出码 %d——换一家 agent 这条就拦不住了", h, e.ExitCode)
		}
		if e.Payload == nil {
			t.Fatalf("%s 的拒绝必须输出 JSON（那是唯一跨 agent 都认的阻断信号）", h)
		}
	}
}

// Gemini 的事件名不是 PreToolUse，写错了 agent 不认。
func TestEventNamesPerHarness(t *testing.T) {
	if hookEventNameFor(hook.HarnessGemini) != "BeforeTool" {
		t.Fatalf("Gemini 的事件名应当是 BeforeTool，得到 %s", hookEventNameFor(hook.HarnessGemini))
	}
	for _, h := range []hook.Harness{
		hook.HarnessClaude, hook.HarnessCodeBuddy, hook.HarnessQwen,
		hook.HarnessQoder, hook.HarnessCodex, hook.HarnessAntigravity, hook.HarnessDroid,
	} {
		if hookEventNameFor(h) != "PreToolUse" {
			t.Fatalf("%s 的事件名应当是 PreToolUse", h)
		}
	}
	if hookEventNameFor(hook.HarnessCursor) != "preToolUse" {
		t.Fatalf("Cursor 的事件名是小驼峰 preToolUse，得到 %s", hookEventNameFor(hook.HarnessCursor))
	}
}

// Cursor 的拒绝要分两份理由：user_message 给人看，agent_message 给 agent 看。
// 只给一份的话，要么用户看不懂为什么被拦，要么 agent 不知道该怎么改。
func TestCursorDenyCarriesBothMessages(t *testing.T) {
	e := emissionFor(hook.HarnessCursor, verdict(guard.Deny, "guard.protected-target", "拒绝：命中受保护目标"))
	got := mustJSON(t, e.Payload)
	for _, want := range []string{`"permission":"deny"`, `"user_message"`, `"agent_message"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("Cursor 的拒绝缺少 %s：%s", want, got)
		}
	}
}

// Cline 的钩子是可执行文件，靠 cancel 布尔值决定放不放行。
func TestClineDenyUsesCancelFlag(t *testing.T) {
	e := emissionFor(hook.HarnessCline, verdict(guard.Deny, "guard.protected-target", "拒绝"))
	got := mustJSON(t, e.Payload)
	if !strings.Contains(got, `"cancel":true`) || !strings.Contains(got, `"errorMessage"`) {
		t.Fatalf("Cline 的拒绝形状不对：%s", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
