package ai

import (
	"strings"
	"testing"
)

// mockClient implements OllamaClient for testing
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
		t.Fatalf("AnomalyType = %s, want crash", diag.AnomalyType)
	}
	if diag.Severity != "ERROR" {
		t.Fatalf("Severity = %s, want ERROR", diag.Severity)
	}
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("panic: runtime error")
	if !strings.Contains(prompt, "panic: runtime error") {
		t.Fatal("prompt should contain the log line")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Fatal("prompt should request JSON output")
	}
}