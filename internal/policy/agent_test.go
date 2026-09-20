package policy

import (
	"github.com/suhui-organization/ratchet/internal/model"
	"os"
	"strings"
	"testing"
)

func TestDefaultAgentIsTheShortHostname(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skip("这台机器取不到主机名")
	}
	short := host
	if idx := strings.Index(short, "."); idx > 0 {
		short = short[:idx]
	}
	if got := DefaultAgent(); got != short {
		t.Fatalf("DefaultAgent()=%q，期望 %q", got, short)
	}
}

func TestDefaultAgentIsNeverTheLiteralPlaceholder(t *testing.T) {
	// "agent" 这种占位名字对每台机器都一样，等于凭据里没有身份。
	// 取不到主机名时宁可写"没识别出来"，也不要写一个看起来正常的名字。
	if got := DefaultAgent(); got == "" {
		t.Fatal("不能返回空串")
	}
}

func TestDraftFillsInAnAgentWhenNoneIsGiven(t *testing.T) {
	p := Draft(model.Inventory{Format: "ratchet-inventory/v1"}, Options{})
	if p.Agent == "" || p.Agent == "agent" {
		t.Fatalf("没有 agent 名时应当补一个可识别的主机名，得到 %q", p.Agent)
	}
}

func TestDraftKeepsAnExplicitAgent(t *testing.T) {
	p := Draft(model.Inventory{Format: "ratchet-inventory/v1"}, Options{Agent: "worker-1"})
	if p.Agent != "worker-1" {
		t.Fatalf("显式给的 agent 不能被覆盖，得到 %q", p.Agent)
	}
}
