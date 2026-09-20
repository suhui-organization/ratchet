package ledger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReachDecodesBothFaces(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		json.NewEncoder(w).Encode(map[string]any{
			"subject": "filesystem",
			"granted": []map[string]any{
				{"agent": "root", "verdict": "allow", "basis": "matched",
					"attestationId": "acme-1", "contained": true},
			},
			"observed":     []map[string]any{{"tool": "filesystem", "count": 3, "attestationId": "acme-1"}},
			"contained":    []string{"root"},
			"denied":       []map[string]any{},
			"unanswerable": []string{},
		})
	}))
	defer srv.Close()

	answer, err := New(srv.URL).Reach("filesystem")
	if err != nil {
		t.Fatalf("Reach: %v", err)
	}
	// 带空格/特殊字符的 subject 必须被转义，否则查询会串味
	if gotPath != "/reach?subject=filesystem" {
		t.Fatalf("请求路径不对：%s", gotPath)
	}
	if len(answer.Granted) != 1 || answer.Granted[0].Agent != "root" || !answer.Granted[0].Contained {
		t.Fatalf("granted 解析不对：%+v", answer.Granted)
	}
	if len(answer.Observed) != 1 || answer.Observed[0].Count != 3 {
		t.Fatalf("observed 解析不对：%+v", answer.Observed)
	}
	if len(answer.Unanswerable) != 0 {
		t.Fatalf("unanswerable 应为空：%+v", answer.Unanswerable)
	}
}

func TestReachEscapesTheSubject(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Reach("a b&c=d"); err != nil {
		t.Fatalf("Reach: %v", err)
	}
	if !strings.Contains(gotPath, "a+b%26c%3Dd") && !strings.Contains(gotPath, "a%20b%26c%3Dd") {
		t.Fatalf("subject 没被转义：%s", gotPath)
	}
}

func TestContainSendsDryRunAndDecodesCascade(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&payload)
		json.NewEncoder(w).Encode(map[string]any{
			"agent": "root", "dryRun": true, "contained": []string{"root", "child"},
			"cascaded": []map[string]any{{"agent": "child", "via": "root", "depth": 1}},
		})
	}))
	defer srv.Close()

	result, err := New(srv.URL).Contain("root", "credential leak", true)
	if err != nil {
		t.Fatalf("Contain: %v", err)
	}
	if payload["dryRun"] != true || payload["agent"] != "root" || payload["reason"] != "credential leak" {
		t.Fatalf("请求体不对：%+v", payload)
	}
	if len(result.Cascaded) != 1 || result.Cascaded[0].Via != "root" || result.Cascaded[0].Depth != 1 {
		t.Fatalf("级联解析不对：%+v", result.Cascaded)
	}
}

func TestErrorsCarryTheControlPlaneMessage(t *testing.T) {
	// "subject is required" 这类信息必须原样带到用户面前：运维要的是能照着改的提示，
	// 而不是"HTTP 400"。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "subject is required"})
	}))
	defer srv.Close()

	_, err := New(srv.URL).Reach("x")
	if err == nil || !strings.Contains(err.Error(), "subject is required") {
		t.Fatalf("错误信息被吞了：%v", err)
	}
}

func TestUnreachableControlPlaneSaysSo(t *testing.T) {
	_, err := New("http://127.0.0.1:1").Reach("x")
	if err == nil {
		t.Fatal("连不上时必须报错")
	}
	if !strings.Contains(err.Error(), "控制面不可达") {
		t.Fatalf("要说明是连不上，而不是别的：%v", err)
	}
}
