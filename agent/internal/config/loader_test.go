package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const minimalYAML = `schema_version: 1
agent:
  id: agent-test
backend:
  base_url: http://localhost:8080/api
  project_name: DR2860
  version: V1.0.0
serial:
  ports:
    - device_sn: DVR-1
      port_name: COM3
ai:
  mode: rules
`

func TestLoadBytesAppliesDocumentDefaults(t *testing.T) {
	cfg, err := LoadBytes([]byte(minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Listen != "0.0.0.0:9000" || cfg.Agent.AnalysisPath != "/analyze" || cfg.Agent.MaxRequestBytes != 5*1024*1024 || cfg.Agent.AnalysisConcurrency != 2 {
		t.Fatalf("unexpected agent defaults: %+v", cfg.Agent)
	}
	if cfg.Backend.RequestTimeout != 180*time.Second || cfg.Backend.UploadInterval != 5*time.Minute || cfg.Backend.UploadConcurrency != 2 || !cfg.Backend.UploadGzip {
		t.Fatalf("unexpected backend defaults: %+v", cfg.Backend)
	}
	port := cfg.Serial.Ports[0]
	if !port.Enabled || port.IdleGap != 10*time.Millisecond || port.MaxFrameBytes != 10*1024 || port.SegmentMaxAge != 5*time.Minute || port.SegmentMaxBytes != 32*1024*1024 {
		t.Fatalf("unexpected serial defaults: %+v", port)
	}
	if cfg.Spool.MaxDiskBytes != 20*1024*1024*1024 || cfg.Spool.UploadedRetention != 24*time.Hour {
		t.Fatalf("unexpected spool defaults: %+v", cfg.Spool)
	}
	if cfg.AI.Mode != "rules" || cfg.AI.Timeout != 50*time.Second || cfg.AI.MaxFindings != 20 || cfg.AI.OllamaURL != "" || cfg.AI.QwenBaseURL != "" {
		t.Fatalf("unexpected AI defaults: %+v", cfg.AI)
	}
}

func TestLoadParsesDurationsAndDoesNotLoadSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	yamlText := strings.Replace(minimalYAML, "  mode: rules\n", "  mode: rules\n  timeout: 45s\n", 1)
	yamlText = strings.Replace(yamlText, "  base_url: http://localhost:8080/api\n", "  base_url: http://localhost:8080/api\n  request_timeout: 2m30s\n  upload_interval: 17s\n", 1)
	yamlText = strings.Replace(yamlText, "      port_name: COM3\n", "      port_name: COM3\n      idle_gap: 25ms\n      segment_max_age: 90s\n", 1)
	if err := os.WriteFile(path, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_ANALYSIS_TOKEN", "must-not-enter-config")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend.RequestTimeout != 150*time.Second || cfg.Backend.UploadInterval != 17*time.Second || cfg.Serial.Ports[0].IdleGap != 25*time.Millisecond || cfg.Serial.Ports[0].SegmentMaxAge != 90*time.Second || cfg.AI.Timeout != 45*time.Second {
		t.Fatalf("durations were not parsed: %+v", cfg)
	}
	if strings.Contains(cfg.Agent.AnalysisTokenEnv, "must-not-enter-config") {
		t.Fatal("secret was copied into config")
	}
}

func TestLoadRejectsUnknownFieldAndBadDuration(t *testing.T) {
	if _, err := LoadBytes([]byte(minimalYAML + "unknown: true\n")); err == nil || !strings.Contains(err.Error(), "field unknown") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
	bad := strings.Replace(minimalYAML, "  base_url: http://localhost:8080/api\n", "  base_url: http://localhost:8080/api\n  request_timeout: forever\n", 1)
	if _, err := LoadBytes([]byte(bad)); err == nil || !strings.Contains(err.Error(), "backend.request_timeout") {
		t.Fatalf("expected duration field error, got %v", err)
	}
}

func TestValidateDocumentBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"base URL suffix", func(c *Config) { c.Backend.BaseURL = "http://localhost/api/v1" }, "must end with /api"},
		{"project length", func(c *Config) { c.Backend.ProjectName = strings.Repeat("x", 129) }, "project_name"},
		{"version length", func(c *Config) { c.Backend.Version = strings.Repeat("x", 65) }, "version"},
		{"baud low", func(c *Config) { c.Serial.Ports[0].BaudRate = 299 }, "baud_rate"},
		{"baud high", func(c *Config) { c.Serial.Ports[0].BaudRate = 4_000_001 }, "baud_rate"},
		{"data bits", func(c *Config) { c.Serial.Ports[0].DataBits = 9 }, "data_bits"},
		{"idle low", func(c *Config) { c.Serial.Ports[0].IdleGap = time.Microsecond }, "idle_gap"},
		{"idle high", func(c *Config) { c.Serial.Ports[0].IdleGap = 3 * time.Second }, "idle_gap"},
		{"frame low", func(c *Config) { c.Serial.Ports[0].MaxFrameBytes = 255 }, "max_frame_bytes"},
		{"frame high", func(c *Config) { c.Serial.Ports[0].MaxFrameBytes = 1_048_577 }, "max_frame_bytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateRejectsDuplicateDeviceAndPort(t *testing.T) {
	cfg := validConfig(t)
	second := cfg.Serial.Ports[0]
	second.PortName = "COM4"
	cfg.Serial.Ports = append(cfg.Serial.Ports, second)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate device_sn") {
		t.Fatalf("expected duplicate device error, got %v", err)
	}
	cfg = validConfig(t)
	second = cfg.Serial.Ports[0]
	second.DeviceSN = "DVR-2"
	second.PortName = "com3"
	cfg.Serial.Ports = append(cfg.Serial.Ports, second)
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate port_name") {
		t.Fatalf("expected duplicate port error, got %v", err)
	}
	cfg.Serial.Ports[1].Enabled = false
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate port_name") {
		t.Fatalf("disabled entries must still reserve their port name, got %v", err)
	}
}

func TestValidateAIConfigurationByMode(t *testing.T) {
	cfg := validConfig(t)
	cfg.AI.Mode = "rules_then_ollama"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ollama_url") {
		t.Fatalf("expected Ollama config error, got %v", err)
	}
	cfg.AI.OllamaURL, cfg.AI.OllamaModel = "http://127.0.0.1:11434", "local-model"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid Ollama mode: %v", err)
	}
	cfg.AI.Mode = "rules_then_ollama_then_qwen"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "qwen_base_url") {
		t.Fatalf("expected Qwen config error, got %v", err)
	}
	cfg.AI.QwenBaseURL, cfg.AI.QwenModel, cfg.AI.QwenAPIKeyEnv = "https://example.invalid/v1", "cloud-model", "QWEN_API_KEY"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid chained mode: %v", err)
	}
}

func TestManualWarningsNeverExposeEnvironmentValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.AnalysisTokenEnv, cfg.AI.QwenAPIKeyEnv = "TEST_ANALYSIS_TOKEN", "TEST_QWEN_KEY"
	t.Setenv("TEST_ANALYSIS_TOKEN", "analysis-super-secret")
	t.Setenv("TEST_QWEN_KEY", "qwen-super-secret")
	warnings := strings.Join(cfg.ManualWarnings(), "\n")
	if strings.Contains(warnings, "analysis-super-secret") || strings.Contains(warnings, "qwen-super-secret") {
		t.Fatalf("warnings leaked a secret: %s", warnings)
	}
	for _, expected := range []string{"backend.base_url", "serial.ports", "Ollama", "Qwen"} {
		if !strings.Contains(warnings, expected) {
			t.Fatalf("warnings did not mention %q: %s", expected, warnings)
		}
	}
}

func validConfig(t *testing.T) Config {
	t.Helper()
	cfg, err := LoadBytes([]byte(minimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
