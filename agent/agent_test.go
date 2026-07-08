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

// mockOllamaClient 实现 ai.OllamaClient 接口，用于测试
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
		t.Fatal("New() 返回 nil")
	}
	if ag.cfg != cfg {
		t.Fatal("cfg 未设置")
	}
	if ag.collector == nil {
		t.Fatal("collector 为 nil")
	}
	if ag.engine == nil {
		t.Fatal("engine 为 nil")
	}
	if ag.analyzer == nil {
		t.Fatal("analyzer 为 nil")
	}
	if ag.uploader == nil {
		t.Fatal("uploader 为 nil")
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

	// 替换 analyzer 为返回诊断结果的 mock
	mc := &mockOllamaClient{response: `{"anomaly_type":"crash","severity":"ERROR","root_cause":"nil pointer","suggestion":"add nil check"}`}
	ag.analyzer = ai.NewAnalyzer(mc, "test")

	line := serialagent.LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "panic: runtime error: nil pointer dereference",
	}
	output := ag.processLine(line)

	// 验证规则匹配
	if output.RuleName != "crash" {
		t.Fatalf("RuleName = %s, 期望 crash", output.RuleName)
	}
	if output.Severity != "ERROR" {
		t.Fatalf("Severity = %s, 期望 ERROR", output.Severity)
	}
	if output.Category != "crash" {
		t.Fatalf("Category = %s, 期望 crash", output.Category)
	}

	// 验证 AI 分析已被触发
	if output.AI == nil {
		t.Fatal("ERROR 级别 AI 不应为 nil")
	}
	if output.AI.AnomalyType != "crash" {
		t.Fatalf("AI AnomalyType = %s, 期望 crash", output.AI.AnomalyType)
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
		t.Fatal("INFO 级别 AI 应为 nil")
	}
	if output.Severity != "INFO" {
		t.Fatalf("Severity = %s, 期望 INFO", output.Severity)
	}
	if output.Category != "unknown" {
		t.Fatalf("Category = %s, 期望 unknown", output.Category)
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
		t.Fatalf("Severity = %s, 期望 WARN", output.Severity)
	}
	if output.AI == nil {
		t.Fatal("WARN 级别 AI 不应为 nil")
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
		t.Fatal("无匹配时 RuleName 应为空")
	}
	if output.Severity != "INFO" {
		t.Fatalf("Severity = %s, 期望 INFO", output.Severity)
	}
	if output.Category != "unknown" {
		t.Fatalf("Category = %s, 期望 unknown", output.Category)
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
		t.Fatalf("Device = %s, 期望 /dev/ttyUSB0", output.Device)
	}
	if output.Content != "test log" {
		t.Fatalf("Content = %s, 期望 test log", output.Content)
	}
}

// 验证 uploader 和 rule 类型已导入
var _ uploader.LogEntry
var _ rule.Rule