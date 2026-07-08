package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 保存 Agent 的所有配置。
type Config struct {
	Devices []DeviceConfig `yaml:"devices"`
	Ollama  OllamaConfig   `yaml:"ollama"`
	Upload  UploadConfig   `yaml:"upload"`
	Rules   []RuleConfig   `yaml:"rules"`
}

// DeviceConfig 保存设备的串口配置。
type DeviceConfig struct {
	Name     string `yaml:"name"`
	BaudRate int    `yaml:"baud_rate"`
	DataBits int    `yaml:"data_bits"`
	StopBits int    `yaml:"stop_bits"`
	Parity   string `yaml:"parity"`
}

// OllamaConfig 保存 Ollama 服务的连接设置。
type OllamaConfig struct {
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

// RuleConfig 保存解析规则配置。
type RuleConfig struct {
	Name     string   `yaml:"name"`
	Keywords []string `yaml:"keywords"`
	Pattern  string   `yaml:"pattern"`
	Severity string   `yaml:"severity"`
	Category string   `yaml:"category"`
}

// UploadConfig 保存 HTTP 上传设置。
type UploadConfig struct {
	Endpoint  string `yaml:"endpoint"`
	APIKey    string `yaml:"api_key"`
	Interval  int    `yaml:"interval_sec"`
	BatchSize int    `yaml:"batch_size"`
}

// Load 读取并解析 YAML 配置文件。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

// applyDefaults 为可选字段填充默认值。
func (c *Config) applyDefaults() {
	if c.Ollama.Endpoint == "" {
		c.Ollama.Endpoint = "http://localhost:11434"
	}
	if c.Ollama.Model == "" {
		c.Ollama.Model = "qwen2.5:7b"
	}
	if c.Upload.Interval == 0 {
		c.Upload.Interval = 10
	}
	if c.Upload.BatchSize == 0 {
		c.Upload.BatchSize = 100
	}
	for i := range c.Devices {
		if c.Devices[i].BaudRate == 0 {
			c.Devices[i].BaudRate = 9600
		}
		if c.Devices[i].DataBits == 0 {
			c.Devices[i].DataBits = 8
		}
		if c.Devices[i].StopBits == 0 {
			c.Devices[i].StopBits = 1
		}
		if c.Devices[i].Parity == "" {
			c.Devices[i].Parity = "none"
		}
	}
}