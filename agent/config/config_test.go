package config

import (
	"os"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load("config.yaml.example")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Devices) != 1 {
		t.Fatalf("Devices = %d, want 1", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "/dev/ttyUSB0" {
		t.Fatalf("Device name = %s, want /dev/ttyUSB0", cfg.Devices[0].Name)
	}
	if cfg.Ollama.Endpoint != "http://localhost:11434" {
		t.Fatalf("Ollama endpoint = %s", cfg.Ollama.Endpoint)
	}
	if cfg.Upload.Endpoint != "http://localhost:8080/api/logs" {
		t.Fatalf("Upload endpoint = %s", cfg.Upload.Endpoint)
	}
	if len(cfg.Rules) != 4 {
		t.Fatalf("Rules = %d, want 4", len(cfg.Rules))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("nonexistent_file.yaml")
	if err == nil {
		t.Fatal("Load() should return error for missing file")
	}
}

func TestDefaults(t *testing.T) {
	// Create a minimal config YAML with empty values
	tmpFile := "test_minimal.yaml"
	data := `
devices: []
ollama:
  endpoint: ""
  model: ""
upload:
  endpoint: ""
  interval_sec: 0
  batch_size: 0
rules: []
`
	if err := os.WriteFile(tmpFile, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Ollama.Endpoint != "http://localhost:11434" {
		t.Fatalf("Ollama endpoint default = %s, want http://localhost:11434", cfg.Ollama.Endpoint)
	}
	if cfg.Ollama.Model != "qwen2.5:7b" {
		t.Fatalf("Ollama model default = %s, want qwen2.5:7b", cfg.Ollama.Model)
	}
	if cfg.Upload.Interval != 10 {
		t.Fatalf("Upload interval default = %d, want 10", cfg.Upload.Interval)
	}
	if cfg.Upload.BatchSize != 100 {
		t.Fatalf("Upload batch_size default = %d, want 100", cfg.Upload.BatchSize)
	}
}

func TestDeviceDefaults(t *testing.T) {
	tmpFile := "test_device_defaults.yaml"
	data := `
devices:
  - name: /dev/ttyUSB1
    baud_rate: 0
    data_bits: 0
    stop_bits: 0
    parity: ""
ollama:
  endpoint: http://localhost:11434
  model: qwen
upload:
  endpoint: http://test
  interval_sec: 10
  batch_size: 50
rules: []
`
	if err := os.WriteFile(tmpFile, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	dev := cfg.Devices[0]
	if dev.BaudRate != 9600 {
		t.Fatalf("BaudRate = %d, want 9600", dev.BaudRate)
	}
	if dev.DataBits != 8 {
		t.Fatalf("DataBits = %d, want 8", dev.DataBits)
	}
	if dev.StopBits != 1 {
		t.Fatalf("StopBits = %d, want 1", dev.StopBits)
	}
	if dev.Parity != "none" {
		t.Fatalf("Parity = %s, want none", dev.Parity)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile := "test_invalid.yaml"
	data := `this is not valid yaml: [unclosed`
	if err := os.WriteFile(tmpFile, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile)

	_, err := Load(tmpFile)
	if err == nil {
		t.Fatal("Load() should return error for invalid YAML")
	}
}
