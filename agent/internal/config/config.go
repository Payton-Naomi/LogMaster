package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AgentID   string         `yaml:"agent_id"`
	ProjectID string         `yaml:"project_id"`
	Upload    UploadConfig   `yaml:"upload"`
	Storage   StorageConfig  `yaml:"storage"`
	Devices   []DeviceConfig `yaml:"devices"`
}

type UploadConfig struct {
	URL         string        `yaml:"url"`
	Interval    time.Duration `yaml:"-"`
	BatchSize   int           `yaml:"batch_size"`
	Timeout     time.Duration `yaml:"-"`
	Token       string        `yaml:"token"`
	RawInterval string        `yaml:"interval"`
	RawTimeout  string        `yaml:"timeout"`
}

type StorageConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
	LogDir     string `yaml:"log_dir"`
}

type DeviceConfig struct {
	DeviceSN        string        `yaml:"device_sn"`
	Source          string        `yaml:"source"`
	Port            string        `yaml:"port"`
	BaudRate        int           `yaml:"baud_rate"`
	DataBits        int           `yaml:"data_bits"`
	StopBits        int           `yaml:"stop_bits"`
	Parity          string        `yaml:"parity"`
	FlowControl     string        `yaml:"flow_control"`
	Encoding        string        `yaml:"encoding"`
	ReadTimeout     time.Duration `yaml:"-"`
	IdleFlush       time.Duration `yaml:"-"`
	MaxLineBytes    int           `yaml:"max_line_bytes"`
	MockInterval    time.Duration `yaml:"-"`
	RawReadTimeout  string        `yaml:"read_timeout"`
	RawIdleFlush    string        `yaml:"idle_flush"`
	RawMockInterval string        `yaml:"mock_interval"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if token := os.Getenv("LOGMASTER_TOKEN"); token != "" {
		cfg.Upload.Token = token
	}
	if err := cfg.applyDefaults(); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c *Config) applyDefaults() error {
	var err error
	if c.Upload.RawInterval == "" {
		c.Upload.Interval = 5 * time.Minute
	} else if c.Upload.Interval, err = time.ParseDuration(c.Upload.RawInterval); err != nil {
		return fmt.Errorf("upload.interval: %w", err)
	}
	if c.Upload.RawTimeout == "" {
		c.Upload.Timeout = 15 * time.Second
	} else if c.Upload.Timeout, err = time.ParseDuration(c.Upload.RawTimeout); err != nil {
		return fmt.Errorf("upload.timeout: %w", err)
	}
	if c.Upload.BatchSize == 0 {
		c.Upload.BatchSize = 500
	}
	for i := range c.Devices {
		d := &c.Devices[i]
		d.Source = strings.ToLower(d.Source)
		if d.Source == "" {
			d.Source = "serial"
		}
		if d.BaudRate == 0 {
			d.BaudRate = 115200
		}
		if d.DataBits == 0 {
			d.DataBits = 8
		}
		if d.StopBits == 0 {
			d.StopBits = 1
		}
		if d.Parity == "" {
			d.Parity = "none"
		}
		if d.FlowControl == "" {
			d.FlowControl = "none"
		}
		if d.Encoding == "" {
			d.Encoding = "utf-8"
		}
		if d.MaxLineBytes == 0 {
			d.MaxLineBytes = 1024 * 1024
		}
		if d.RawReadTimeout == "" {
			d.ReadTimeout = 200 * time.Millisecond
		} else if d.ReadTimeout, err = time.ParseDuration(d.RawReadTimeout); err != nil {
			return fmt.Errorf("devices[%d].read_timeout: %w", i, err)
		}
		if d.RawIdleFlush == "" {
			d.IdleFlush = 100 * time.Millisecond
		} else if d.IdleFlush, err = time.ParseDuration(d.RawIdleFlush); err != nil {
			return fmt.Errorf("devices[%d].idle_flush: %w", i, err)
		}
		if d.RawMockInterval == "" {
			d.MockInterval = 500 * time.Millisecond
		} else if d.MockInterval, err = time.ParseDuration(d.RawMockInterval); err != nil {
			return fmt.Errorf("devices[%d].mock_interval: %w", i, err)
		}
	}
	return nil
}

func (c Config) Validate() error {
	if c.AgentID == "" || c.ProjectID == "" {
		return errors.New("agent_id and project_id are required")
	}
	if c.Upload.URL == "" {
		return errors.New("upload.url is required")
	}
	if c.Upload.Interval <= 0 || c.Upload.Timeout <= 0 || c.Upload.BatchSize <= 0 {
		return errors.New("upload interval, timeout, and batch_size must be positive")
	}
	if c.Storage.SQLitePath == "" || c.Storage.LogDir == "" {
		return errors.New("storage.sqlite_path and storage.log_dir are required")
	}
	if len(c.Devices) == 0 {
		return errors.New("at least one device is required")
	}
	seen := map[string]bool{}
	for i, d := range c.Devices {
		if d.DeviceSN == "" {
			return fmt.Errorf("devices[%d].device_sn is required", i)
		}
		if seen[d.DeviceSN] {
			return fmt.Errorf("duplicate device_sn %q", d.DeviceSN)
		}
		seen[d.DeviceSN] = true
		if d.Source != "serial" && d.Source != "mock" {
			return fmt.Errorf("devices[%d].source must be serial or mock", i)
		}
		if d.Source == "serial" && d.Port == "" {
			return fmt.Errorf("devices[%d].port is required", i)
		}
		if d.DataBits < 5 || d.DataBits > 8 || (d.StopBits != 1 && d.StopBits != 2) {
			return fmt.Errorf("devices[%d] has unsupported serial mode", i)
		}
		if strings.ToLower(d.Encoding) != "utf-8" && strings.ToLower(d.Encoding) != "utf8" {
			return fmt.Errorf("devices[%d].encoding currently supports utf-8 only", i)
		}
		if strings.ToLower(d.FlowControl) != "none" {
			return fmt.Errorf("devices[%d].flow_control currently supports none only", i)
		}
		if d.MaxLineBytes <= 0 || d.ReadTimeout <= 0 || d.IdleFlush <= 0 {
			return fmt.Errorf("devices[%d] timeout and line limits must be positive", i)
		}
	}
	return nil
}
