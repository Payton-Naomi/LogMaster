package ai

import (
	"strings"
	"testing"
)

// mockClient 实现 OllamaClient 接口，用于测试
type mockClient struct {
	response string
}

func (m *mockClient) Generate(prompt string) (string, error) {
	return m.response, nil
}

func TestAnalyzerAnalyze(t *testing.T) {
	mc := &mockClient{
		response: `{"anomaly_type":"crash","severity":"ERROR","root_cause":"nil pointer","suggestion":"add nil check"}`,
	}
	a := NewAnalyzer(mc, "qwen2.5:7b")

	diag, err := a.Analyze("panic: runtime error: nil pointer dereference")
	if err != nil {
		t.Fatalf("Analyze() error: %v", err)
	}
	if diag.AnomalyType != "crash" {
		t.Fatalf("AnomalyType = %s, 期望 crash", diag.AnomalyType)
	}
	if diag.Severity != "ERROR" {
		t.Fatalf("Severity = %s, 期望 ERROR", diag.Severity)
	}
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("panic: runtime error")
	if !strings.Contains(prompt, "panic: runtime error") {
		t.Fatal("prompt 应包含日志行")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Fatal("prompt 应要求 JSON 输出")
	}
}