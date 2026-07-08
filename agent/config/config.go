package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the agent.
type Config struct {
	Devices []DeviceConfig `yaml:"devices"`
	Ollama  OllamaConfig   `yaml:"ollama"`
	Upload  UploadConfig   `yaml:"upload"`
	Rules   []RuleConfig   `yaml:"rules"`
}

// DeviceConfig holds serial port configuration for a device.
type DeviceConfig struct {
	Name     string `yaml:"name"`
	BaudRate int    `yaml:"baud_rate"`
	DataBits int    `yaml:"data_bits"`
	StopBits int    `yaml:"stop_bits"`
	Parity   string `yaml:"parity"`
}

// OllamaConfig holds Ollama service connection settings.
type OllamaConfig struct {
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

// RuleConfig holds a parsing rule configuration.
type RuleConfig struct {
	Name     string   `yaml:"name"`
	Keywords []string `yaml:"keywords"`
	Pattern  string   `yaml:"pattern"`
	Severity string   `yaml:"severity"`
	Category string   `yaml:"category"`
}

// UploadConfig holds HTTP upload settings.
type UploadConfig struct {
	Endpoint  string `yaml:"endpoint"`
	APIKey    string `yaml:"api_key"`
	Interval  int    `yaml:"interval_sec"`
	BatchSize int    `yaml:"batch_size"`
}

// Load reads and parses a YAML config file.
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

// applyDefaults fills in default values for optional fields.
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