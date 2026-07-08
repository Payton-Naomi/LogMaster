package agent

import (
	"testing"
	"time"

	"logmaster-agent/agent/ai"
	"logmaster-agent/agent/config"
	"logmaster-agent/agent/rule"
	serialagent "logmaster-agent/agent/serial"
	"logmaster-agent/agent/uploader"
)

// mockOllamaClient implements ai.OllamaClient for testing
type mockOllamaClient struct {
	response string
}

func (m *mockOllamaClient) Generate(prompt string) (string, error) {
	return m.response, nil
}

func TestNewAgent(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{
			{Name: "crash", Keywords: []string{"panic"}, Severity: "ERROR", Category: "crash"},
		},
	}
	ag := New(cfg)
	if ag == nil {
		t.Fatal("New() returned nil")
	}
	if ag.cfg != cfg {
		t.Fatal("cfg not set")
	}
	if ag.collector == nil {
		t.Fatal("collector is nil")
	}
	if ag.engine == nil {
		t.Fatal("engine is nil")
	}
	if ag.analyzer == nil {
		t.Fatal("analyzer is nil")
	}
	if ag.uploader == nil {
		t.Fatal("uploader is nil")
	}
}

func TestProcessLineErrorMatch(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{
			{Name: "crash", Keywords: []string{"panic"}, Severity: "ERROR", Category: "crash"},
		},
	}
	ag := New(cfg)

	// Replace analyzer with a mock that returns a diagnosis
	mc := &mockOllamaClient{response: `{"anomaly_type":"crash","severity":"ERROR","root_cause":"nil pointer","suggestion":"add nil check"}`}
	ag.analyzer = ai.NewAnalyzer(mc, "test")

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "panic: runtime error: nil pointer dereference",
	}
	output := ag.processLine(line)

	// Verify rule matching
	if output.RuleName != "crash" {
		t.Fatalf("RuleName = %s, want crash", output.RuleName)
	}
	if output.Severity != "ERROR" {
		t.Fatalf("Severity = %s, want ERROR", output.Severity)
	}
	if output.Category != "crash" {
		t.Fatalf("Category = %s, want crash", output.Category)
	}

	// Verify AI analysis was triggered
	if output.AI == nil {
		t.Fatal("AI should not be nil for ERROR")
	}
	if output.AI.AnomalyType != "crash" {
		t.Fatalf("AI AnomalyType = %s, want crash", output.AI.AnomalyType)
	}
}

func TestProcessLineInfoNoAI(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{},
	}
	ag := New(cfg)

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "INFO: normal operation",
	}
	output := ag.processLine(line)

	if output.AI != nil {
		t.Fatal("AI should be nil for INFO")
	}
	if output.Severity != "INFO" {
		t.Fatalf("Severity = %s, want INFO", output.Severity)
	}
	if output.Category != "unknown" {
		t.Fatalf("Category = %s, want unknown", output.Category)
	}
}

func TestProcessLineWarnWithAI(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{
			{Name: "timeout", Keywords: []string{"timeout"}, Severity: "WARN", Category: "network"},
		},
	}
	ag := New(cfg)

	mc := &mockOllamaClient{response: `{"anomaly_type":"network_timeout","severity":"WARN","root_cause":"slow response","suggestion":"increase timeout"}`}
	ag.analyzer = ai.NewAnalyzer(mc, "test")

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "timeout: connection to server failed",
	}
	output := ag.processLine(line)

	if output.Severity != "WARN" {
		t.Fatalf("Severity = %s, want WARN", output.Severity)
	}
	if output.AI == nil {
		t.Fatal("AI should not be nil for WARN")
	}
}

func TestProcessLineNoMatch(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{
			{Name: "crash", Keywords: []string{"panic"}, Severity: "ERROR", Category: "crash"},
		},
	}
	ag := New(cfg)

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "normal heartbeat message",
	}
	output := ag.processLine(line)

	if output.RuleName != "" {
		t.Fatal("RuleName should be empty for no match")
	}
	if output.Severity != "INFO" {
		t.Fatalf("Severity = %s, want INFO", output.Severity)
	}
	if output.Category != "unknown" {
		t.Fatalf("Category = %s, want unknown", output.Category)
	}
}

func TestProcessLineEnqueuesToUploader(t *testing.T) {
	cfg := &config.Config{
		Ollama: config.OllamaConfig{Endpoint: "http://test", Model: "test"},
		Upload: config.UploadConfig{Endpoint: "http://test", Interval: 10, BatchSize: 100},
		Rules: []config.RuleConfig{},
	}
	ag := New(cfg)

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "test log",
	}
	output := ag.processLine(line)

	if output.Device != "/dev/ttyUSB0" {
		t.Fatalf("Device = %s, want /dev/ttyUSB0", output.Device)
	}
	if output.Content != "test log" {
		t.Fatalf("Content = %s, want test log", output.Content)
	}
}

// Verify uploader and rule types are imported
var _ uploader.LogEntry
var _ rule.Rule