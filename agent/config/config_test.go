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
		t.Fatalf("设备数量 = %d, 期望 1", len(cfg.Devices))
	}
	if cfg.Devices[0].Name != "/dev/ttyUSB0" {
		t.Fatalf("设备名称 = %s, 期望 /dev/ttyUSB0", cfg.Devices[0].Name)
	}
	if cfg.Ollama.Endpoint != "http://localhost:11434" {
		t.Fatalf("Ollama 端点 = %s", cfg.Ollama.Endpoint)
	}
	if cfg.Upload.Endpoint != "http://localhost:8080/api/logs" {
		t.Fatalf("上传端点 = %s", cfg.Upload.Endpoint)
	}
	if len(cfg.Rules) != 4 {
		t.Fatalf("规则 = %d, 期望 4", len(cfg.Rules))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("nonexistent_file.yaml")
	if err == nil {
		t.Fatal("Load() 应在文件不存在时返回错误")
	}
}

func TestDefaults(t *testing.T) {
	// 创建一个包含空值的最小配置 YAML
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
		t.Fatalf("Ollama 端点默认值 = %s, 期望 http://localhost:11434", cfg.Ollama.Endpoint)
	}
	if cfg.Ollama.Model != "qwen2.5:7b" {
		t.Fatalf("Ollama 模型默认值 = %s, 期望 qwen2.5:7b", cfg.Ollama.Model)
	}
	if cfg.Upload.Interval != 10 {
		t.Fatalf("上传间隔默认值 = %d, 期望 10", cfg.Upload.Interval)
	}
	if cfg.Upload.BatchSize != 100 {
		t.Fatalf("上传批量大小默认值 = %d, 期望 100", cfg.Upload.BatchSize)
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
		t.Fatalf("BaudRate = %d, 期望 9600", dev.BaudRate)
	}
	if dev.DataBits != 8 {
		t.Fatalf("DataBits = %d, 期望 8", dev.DataBits)
	}
	if dev.StopBits != 1 {
		t.Fatalf("StopBits = %d, 期望 1", dev.StopBits)
	}
	if dev.Parity != "none" {
		t.Fatalf("校验位 = %s, 期望 none", dev.Parity)
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
		t.Fatal("Load() 应在无效 YAML 时返回错误")
	}
}
