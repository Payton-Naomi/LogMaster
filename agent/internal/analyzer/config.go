package analyzer

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Mode string

const (
	ModeRules               Mode = "rules"
	ModeRulesThenOllama     Mode = "rules_then_ollama"
	ModeRulesThenQwen       Mode = "rules_then_qwen"
	ModeRulesOllamaThenQwen Mode = "rules_then_ollama_then_qwen"
)

type ModelConfig struct {
	BaseURL string
	Model   string
	APIKey  string
	Client  *http.Client
}

type Config struct {
	Path            string
	Token           string
	MaxRequestBytes int64
	Timeout         time.Duration
	MaxConcurrent   int
	MaxFindings     int
	CacheTTL        time.Duration
	Mode            Mode
	Ollama          ModelConfig
	Qwen            ModelConfig
}

func DefaultConfig() Config {
	return Config{
		Path:            "/analyze",
		MaxRequestBytes: 5 << 20,
		Timeout:         50 * time.Second,
		MaxConcurrent:   2,
		MaxFindings:     20,
		CacheTTL:        24 * time.Hour,
		Mode:            ModeRules,
	}
}

func (c *Config) applyDefaults() {
	defaults := DefaultConfig()
	if c.Path == "" {
		c.Path = defaults.Path
	}
	if c.MaxRequestBytes == 0 {
		c.MaxRequestBytes = defaults.MaxRequestBytes
	}
	if c.Timeout == 0 {
		c.Timeout = defaults.Timeout
	}
	if c.MaxConcurrent == 0 {
		c.MaxConcurrent = defaults.MaxConcurrent
	}
	if c.MaxFindings == 0 {
		c.MaxFindings = defaults.MaxFindings
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = defaults.CacheTTL
	}
	if c.Mode == "" {
		c.Mode = defaults.Mode
	}
}

func (c Config) Validate() error {
	if !strings.HasPrefix(c.Path, "/") || strings.ContainsAny(c.Path, "?#") {
		return errors.New("analysis path must be an absolute URL path")
	}
	if c.MaxRequestBytes <= 0 {
		return errors.New("analysis max request bytes must be positive")
	}
	if c.Timeout <= 0 {
		return errors.New("analysis timeout must be positive")
	}
	if c.MaxConcurrent <= 0 {
		return errors.New("analysis max concurrency must be positive")
	}
	if c.MaxFindings <= 0 || c.MaxFindings > 20 {
		return errors.New("analysis max findings must be between 1 and 20")
	}
	if c.CacheTTL < 24*time.Hour {
		return errors.New("analysis cache TTL must be at least 24 hours")
	}
	switch c.Mode {
	case ModeRules:
		return nil
	case ModeRulesThenOllama:
		return validateModelConfig("ollama", c.Ollama, false)
	case ModeRulesThenQwen:
		return validateModelConfig("qwen", c.Qwen, true)
	case ModeRulesOllamaThenQwen:
		if err := validateModelConfig("ollama", c.Ollama, false); err != nil {
			return err
		}
		return validateModelConfig("qwen", c.Qwen, true)
	default:
		return fmt.Errorf("unsupported analysis mode %q", c.Mode)
	}
}

func validateModelConfig(name string, cfg ModelConfig, needsKey bool) error {
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s base URL must be a valid HTTP URL", name)
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("%s model is required", name)
	}
	if needsKey && strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("%s API key is required", name)
	}
	return nil
}
