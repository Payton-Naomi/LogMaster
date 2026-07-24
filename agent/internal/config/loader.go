package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	SchemaVersion int           `yaml:"schema_version"`
	Agent         AgentConfig   `yaml:"agent"`
	Backend       BackendConfig `yaml:"backend"`
	Serial        SerialConfig  `yaml:"serial"`
	Spool         SpoolConfig   `yaml:"spool"`
	AI            AIConfig      `yaml:"ai"`
}

type AgentConfig struct {
	ID                  string `yaml:"id"`
	Name                string `yaml:"name"`
	Listen              string `yaml:"listen"`
	AnalysisPath        string `yaml:"analysis_path"`
	AnalysisTokenEnv    string `yaml:"analysis_token_env"`
	MaxRequestBytes     int64  `yaml:"max_request_bytes"`
	AnalysisConcurrency int    `yaml:"analysis_concurrency"`
}

type BackendConfig struct {
	BaseURL             string        `yaml:"base_url"`
	HealthPath          string        `yaml:"health_path"`
	InspectPath         string        `yaml:"inspect_path"`
	UploadPath          string        `yaml:"upload_path"`
	RequestTimeout      time.Duration `yaml:"-"`
	ProjectName         string        `yaml:"project_name"`
	Version             string        `yaml:"version"`
	UploadInterval      time.Duration `yaml:"-"`
	InspectBeforeUpload bool          `yaml:"inspect_before_upload"`
	UploadConcurrency   int           `yaml:"upload_concurrency"`
	UploadGzip          bool          `yaml:"upload_gzip"`
}

type SerialConfig struct {
	DiscoverInterval time.Duration   `yaml:"-"`
	Reconnect        ReconnectConfig `yaml:"reconnect"`
	Ports            []PortConfig    `yaml:"ports"`
}

type ReconnectConfig struct {
	InitialDelay time.Duration `yaml:"-"`
	Multiplier   float64       `yaml:"multiplier"`
	MaxDelay     time.Duration `yaml:"-"`
	Jitter       float64       `yaml:"jitter"`
}

type PortConfig struct {
	Enabled         bool            `yaml:"enabled"`
	DeviceSN        string          `yaml:"device_sn"`
	PortName        string          `yaml:"port_name"`
	PortMatch       PortMatchConfig `yaml:"port_match"`
	BaudRate        int             `yaml:"baud_rate"`
	DataBits        int             `yaml:"data_bits"`
	StopBits        int             `yaml:"stop_bits"`
	Parity          string          `yaml:"parity"`
	Handshake       string          `yaml:"handshake"`
	DTR             bool            `yaml:"dtr"`
	RTS             bool            `yaml:"rts"`
	ReadTimeout     time.Duration   `yaml:"-"`
	WriteTimeout    time.Duration   `yaml:"-"`
	IdleGap         time.Duration   `yaml:"-"`
	MaxFrameBytes   int             `yaml:"max_frame_bytes"`
	Encoding        string          `yaml:"encoding"`
	LineEnding      string          `yaml:"line_ending"`
	SegmentMaxAge   time.Duration   `yaml:"-"`
	SegmentMaxBytes int64           `yaml:"segment_max_bytes"`
}

type PortMatchConfig struct {
	VID              string `yaml:"vid"`
	PID              string `yaml:"pid"`
	USBSerial        string `yaml:"usb_serial"`
	PhysicalLocation string `yaml:"physical_location"`
}

type SpoolConfig struct {
	Directory         string        `yaml:"directory"`
	SQLitePath        string        `yaml:"sqlite_path"`
	QueueCapacity     int           `yaml:"queue_capacity"`
	MaxDiskBytes      int64         `yaml:"max_disk_bytes"`
	UploadedRetention time.Duration `yaml:"-"`
}

type AIConfig struct {
	Mode          string        `yaml:"mode"`
	Timeout       time.Duration `yaml:"-"`
	MaxFindings   int           `yaml:"max_findings"`
	OllamaURL     string        `yaml:"ollama_url"`
	OllamaModel   string        `yaml:"ollama_model"`
	QwenBaseURL   string        `yaml:"qwen_base_url"`
	QwenAPIKeyEnv string        `yaml:"qwen_api_key_env"`
	QwenModel     string        `yaml:"qwen_model"`
}

func DefaultConfig() Config {
	return Config{
		SchemaVersion: SchemaVersion,
		Agent: AgentConfig{
			Listen: "0.0.0.0:9000", AnalysisPath: "/analyze", AnalysisTokenEnv: "AGENT_ANALYSIS_TOKEN",
			MaxRequestBytes: 5 * 1024 * 1024, AnalysisConcurrency: 2,
		},
		Backend: BackendConfig{
			HealthPath: "/health", InspectPath: "/logs/inspect", UploadPath: "/logs/upload",
			RequestTimeout: 180 * time.Second, UploadInterval: 5 * time.Minute, UploadConcurrency: 2, UploadGzip: true,
		},
		Serial: SerialConfig{
			DiscoverInterval: 5 * time.Second,
			Reconnect:        ReconnectConfig{InitialDelay: time.Second, Multiplier: 2, MaxDelay: 30 * time.Second, Jitter: 0.2},
		},
		Spool: SpoolConfig{
			Directory: "./data/spool", SQLitePath: "./data/agent.db", QueueCapacity: 2048,
			MaxDiskBytes: 20 * 1024 * 1024 * 1024, UploadedRetention: 24 * time.Hour,
		},
		AI: AIConfig{Mode: "rules", Timeout: 50 * time.Second, MaxFindings: 20},
	}
}

func DefaultPortConfig() PortConfig {
	return PortConfig{
		Enabled: true, BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: "none", Handshake: "none",
		ReadTimeout: 200 * time.Millisecond, WriteTimeout: time.Second, IdleGap: 10 * time.Millisecond,
		MaxFrameBytes: 10 * 1024, Encoding: "utf-8", LineEnding: "auto",
		SegmentMaxAge: 5 * time.Minute, SegmentMaxBytes: 32 * 1024 * 1024,
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return LoadBytes(data)
}

func LoadBytes(data []byte) (Config, error) {
	cfg := DefaultConfig()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Config{}, errors.New("parse config: multiple YAML documents are not supported")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func (c *BackendConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain BackendConfig
	raw := struct {
		*plain         `yaml:",inline"`
		RequestTimeout string `yaml:"request_timeout"`
		UploadInterval string `yaml:"upload_interval"`
	}{plain: (*plain)(c), RequestTimeout: c.RequestTimeout.String(), UploadInterval: c.UploadInterval.String()}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	var err error
	if c.RequestTimeout, err = parseDuration("backend.request_timeout", raw.RequestTimeout); err != nil {
		return err
	}
	c.UploadInterval, err = parseDuration("backend.upload_interval", raw.UploadInterval)
	return err
}

func (c *SerialConfig) UnmarshalYAML(node *yaml.Node) error {
	raw := struct {
		DiscoverInterval string             `yaml:"discover_interval"`
		Reconnect        rawReconnectConfig `yaml:"reconnect"`
		Ports            []yaml.Node        `yaml:"ports"`
	}{
		DiscoverInterval: c.DiscoverInterval.String(),
		Reconnect:        rawReconnectConfig{InitialDelay: c.Reconnect.InitialDelay.String(), Multiplier: c.Reconnect.Multiplier, MaxDelay: c.Reconnect.MaxDelay.String(), Jitter: c.Reconnect.Jitter},
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	var err error
	if c.DiscoverInterval, err = parseDuration("serial.discover_interval", raw.DiscoverInterval); err != nil {
		return err
	}
	if c.Reconnect.InitialDelay, err = parseDuration("serial.reconnect.initial_delay", raw.Reconnect.InitialDelay); err != nil {
		return err
	}
	c.Reconnect.Multiplier = raw.Reconnect.Multiplier
	if c.Reconnect.MaxDelay, err = parseDuration("serial.reconnect.max_delay", raw.Reconnect.MaxDelay); err != nil {
		return err
	}
	c.Reconnect.Jitter = raw.Reconnect.Jitter
	c.Ports = make([]PortConfig, len(raw.Ports))
	for i := range raw.Ports {
		c.Ports[i] = DefaultPortConfig()
		if err := raw.Ports[i].Decode(&c.Ports[i]); err != nil {
			return fmt.Errorf("serial.ports[%d]: %w", i, err)
		}
	}
	return nil
}

type rawReconnectConfig struct {
	InitialDelay string  `yaml:"initial_delay"`
	Multiplier   float64 `yaml:"multiplier"`
	MaxDelay     string  `yaml:"max_delay"`
	Jitter       float64 `yaml:"jitter"`
}

func (c *PortConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain PortConfig
	raw := struct {
		*plain        `yaml:",inline"`
		ReadTimeout   string `yaml:"read_timeout"`
		WriteTimeout  string `yaml:"write_timeout"`
		IdleGap       string `yaml:"idle_gap"`
		SegmentMaxAge string `yaml:"segment_max_age"`
	}{
		plain: (*plain)(c), ReadTimeout: c.ReadTimeout.String(), WriteTimeout: c.WriteTimeout.String(),
		IdleGap: c.IdleGap.String(), SegmentMaxAge: c.SegmentMaxAge.String(),
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	var err error
	if c.ReadTimeout, err = parseDuration("read_timeout", raw.ReadTimeout); err != nil {
		return err
	}
	if c.WriteTimeout, err = parseDuration("write_timeout", raw.WriteTimeout); err != nil {
		return err
	}
	if c.IdleGap, err = parseDuration("idle_gap", raw.IdleGap); err != nil {
		return err
	}
	if c.SegmentMaxAge, err = parseDuration("segment_max_age", raw.SegmentMaxAge); err != nil {
		return err
	}
	c.Parity = strings.ToLower(strings.TrimSpace(c.Parity))
	c.Handshake = strings.ToLower(strings.TrimSpace(c.Handshake))
	c.Encoding = strings.ToLower(strings.TrimSpace(c.Encoding))
	c.LineEnding = strings.ToLower(strings.TrimSpace(c.LineEnding))
	return nil
}

func (c *SpoolConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain SpoolConfig
	raw := struct {
		*plain            `yaml:",inline"`
		UploadedRetention string `yaml:"uploaded_retention"`
	}{plain: (*plain)(c), UploadedRetention: c.UploadedRetention.String()}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	var err error
	c.UploadedRetention, err = parseDuration("spool.uploaded_retention", raw.UploadedRetention)
	return err
}

func (c *AIConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain AIConfig
	raw := struct {
		*plain  `yaml:",inline"`
		Timeout string `yaml:"timeout"`
	}{plain: (*plain)(c), Timeout: c.Timeout.String()}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	var err error
	c.Timeout, err = parseDuration("ai.timeout", raw.Timeout)
	c.Mode = strings.ToLower(strings.TrimSpace(c.Mode))
	return err
}

func parseDuration(field, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	return parsed, nil
}

func (c Config) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %d", SchemaVersion)
	}
	if strings.TrimSpace(c.Agent.ID) == "" {
		return errors.New("agent.id is required")
	}
	if _, _, err := net.SplitHostPort(c.Agent.Listen); err != nil {
		return fmt.Errorf("agent.listen must be host:port: %w", err)
	}
	if !strings.HasPrefix(c.Agent.AnalysisPath, "/") {
		return errors.New("agent.analysis_path must start with /")
	}
	if c.Agent.AnalysisTokenEnv != "" && !environmentNamePattern.MatchString(c.Agent.AnalysisTokenEnv) {
		return errors.New("agent.analysis_token_env must be an environment variable name")
	}
	if c.Agent.MaxRequestBytes <= 0 || c.Agent.AnalysisConcurrency <= 0 {
		return errors.New("agent request limit and analysis concurrency must be positive")
	}
	if err := validateBackend(c.Backend); err != nil {
		return err
	}
	if err := validateSerial(c.Serial); err != nil {
		return err
	}
	if strings.TrimSpace(c.Spool.Directory) == "" || strings.TrimSpace(c.Spool.SQLitePath) == "" {
		return errors.New("spool.directory and spool.sqlite_path are required")
	}
	if c.Spool.QueueCapacity <= 0 || c.Spool.MaxDiskBytes <= 0 || c.Spool.UploadedRetention <= 0 {
		return errors.New("spool queue capacity, disk limit, and retention must be positive")
	}
	return validateAI(c.AI)
}

func validateBackend(c BackendConfig) error {
	baseURL := strings.TrimSpace(c.BaseURL)
	parsed, err := url.ParseRequestURI(baseURL)
	if baseURL == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("backend.base_url must be an absolute HTTP(S) URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(baseURL, "/api") {
		return errors.New("backend.base_url must end with /api and must not contain a query or fragment")
	}
	if utf8.RuneCountInString(c.ProjectName) == 0 || utf8.RuneCountInString(c.ProjectName) > 128 {
		return errors.New("backend.project_name is required and must not exceed 128 characters")
	}
	if utf8.RuneCountInString(c.Version) == 0 || utf8.RuneCountInString(c.Version) > 64 {
		return errors.New("backend.version is required and must not exceed 64 characters")
	}
	for name, path := range map[string]string{"health_path": c.HealthPath, "inspect_path": c.InspectPath, "upload_path": c.UploadPath} {
		if !strings.HasPrefix(path, "/") {
			return fmt.Errorf("backend.%s must start with /", name)
		}
	}
	if c.RequestTimeout <= 0 || c.UploadInterval <= 0 || c.UploadConcurrency <= 0 || c.UploadConcurrency > 16 {
		return errors.New("backend request timeout, upload interval, and upload concurrency (1..16) must be valid")
	}
	return nil
}

func validateSerial(c SerialConfig) error {
	if c.DiscoverInterval <= 0 {
		return errors.New("serial.discover_interval must be positive")
	}
	if c.Reconnect.InitialDelay <= 0 || c.Reconnect.MaxDelay <= 0 || c.Reconnect.InitialDelay > c.Reconnect.MaxDelay {
		return errors.New("serial reconnect delays must be positive and ordered")
	}
	if c.Reconnect.Multiplier < 1 || c.Reconnect.Jitter < 0 || c.Reconnect.Jitter > 1 {
		return errors.New("serial reconnect multiplier must be at least 1 and jitter between 0 and 1")
	}
	if len(c.Ports) == 0 {
		return errors.New("serial.ports must contain at least one port")
	}
	devices, ports := map[string]bool{}, map[string]bool{}
	for i, port := range c.Ports {
		prefix := fmt.Sprintf("serial.ports[%d]", i)
		device := strings.ToLower(strings.TrimSpace(port.DeviceSN))
		if port.Enabled && device == "" {
			return fmt.Errorf("%s.device_sn is required", prefix)
		}
		if device != "" && devices[device] {
			return fmt.Errorf("duplicate device_sn %q", port.DeviceSN)
		}
		if device != "" {
			devices[device] = true
		}
		portName := strings.ToLower(strings.TrimSpace(port.PortName))
		if portName != "" && ports[portName] {
			return fmt.Errorf("duplicate port_name %q", port.PortName)
		}
		if portName != "" {
			ports[portName] = true
		}
		if !port.Enabled {
			continue
		}
		if portName == "" && port.PortMatch == (PortMatchConfig{}) {
			return fmt.Errorf("%s requires port_name or port_match", prefix)
		}
		if port.BaudRate < 300 || port.BaudRate > 4_000_000 {
			return fmt.Errorf("%s.baud_rate must be between 300 and 4000000", prefix)
		}
		if port.DataBits < 5 || port.DataBits > 8 {
			return fmt.Errorf("%s.data_bits must be one of 5, 6, 7, 8", prefix)
		}
		if port.StopBits != 1 && port.StopBits != 2 {
			return fmt.Errorf("%s.stop_bits must be 1 or 2", prefix)
		}
		if !oneOf(port.Parity, "none", "odd", "even", "mark", "space") || !oneOf(port.Handshake, "none", "rtscts", "xonxoff") {
			return fmt.Errorf("%s parity or handshake is unsupported", prefix)
		}
		if port.ReadTimeout <= 0 || port.WriteTimeout <= 0 {
			return fmt.Errorf("%s read_timeout and write_timeout must be positive", prefix)
		}
		if port.IdleGap < time.Millisecond || port.IdleGap > 2*time.Second {
			return fmt.Errorf("%s.idle_gap must be between 1ms and 2s", prefix)
		}
		if port.MaxFrameBytes < 256 || port.MaxFrameBytes > 1_048_576 {
			return fmt.Errorf("%s.max_frame_bytes must be between 256 and 1048576", prefix)
		}
		if !oneOf(port.Encoding, "utf-8", "utf8", "gb18030", "ascii") || !oneOf(port.LineEnding, "auto", "crlf", "lf", "cr") {
			return fmt.Errorf("%s encoding or line_ending is unsupported", prefix)
		}
		if port.SegmentMaxAge <= 0 || port.SegmentMaxBytes <= 0 {
			return fmt.Errorf("%s segment limits must be positive", prefix)
		}
	}
	return nil
}

func validateAI(c AIConfig) error {
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	valid := map[string]bool{"rules": true, "rules_then_ollama": true, "rules_then_qwen": true, "rules_then_ollama_then_qwen": true}
	if !valid[mode] {
		return fmt.Errorf("ai.mode %q is unsupported", c.Mode)
	}
	if c.Timeout <= 0 || c.MaxFindings <= 0 || c.MaxFindings > 20 {
		return errors.New("ai.timeout must be positive and ai.max_findings must be between 1 and 20")
	}
	if strings.Contains(mode, "ollama") {
		if c.OllamaURL == "" || c.OllamaModel == "" {
			return errors.New("ai.ollama_url and ai.ollama_model are required when ai.mode uses Ollama")
		}
		if err := validateHTTPURL(c.OllamaURL); err != nil {
			return fmt.Errorf("ai.ollama_url: %w", err)
		}
	}
	if strings.Contains(mode, "qwen") {
		if c.QwenBaseURL == "" || c.QwenModel == "" || c.QwenAPIKeyEnv == "" {
			return errors.New("ai.qwen_base_url, ai.qwen_model, and ai.qwen_api_key_env are required when ai.mode uses Qwen")
		}
		if err := validateHTTPURL(c.QwenBaseURL); err != nil {
			return fmt.Errorf("ai.qwen_base_url: %w", err)
		}
		if !environmentNamePattern.MatchString(c.QwenAPIKeyEnv) {
			return errors.New("ai.qwen_api_key_env must be an environment variable name")
		}
	}
	return nil
}

func validateHTTPURL(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("must be an absolute HTTP(S) URL")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// ManualWarnings reports operator actions without ever reading or returning secrets.
func (c Config) ManualWarnings() []string {
	var warnings []string
	if c.Agent.ID == "" {
		warnings = append(warnings, "configure agent.id for this machine")
	}
	if c.Backend.BaseURL == "" {
		warnings = append(warnings, "configure backend.base_url")
	}
	if c.Backend.ProjectName == "" {
		warnings = append(warnings, "configure backend.project_name")
	}
	if c.Backend.Version == "" {
		warnings = append(warnings, "configure backend.version")
	}
	if len(c.Serial.Ports) == 0 {
		warnings = append(warnings, "configure serial.ports for the attached devices")
	}
	if c.Agent.AnalysisTokenEnv == "" {
		warnings = append(warnings, "configure agent.analysis_token_env and its environment variable")
	} else if _, ok := os.LookupEnv(c.Agent.AnalysisTokenEnv); !ok {
		warnings = append(warnings, fmt.Sprintf("set environment variable %s for Analyzer authentication", c.Agent.AnalysisTokenEnv))
	}
	if c.AI.OllamaURL == "" || c.AI.OllamaModel == "" {
		warnings = append(warnings, "optional Ollama integration is disabled; configure ai.ollama_url and ai.ollama_model to enable it")
	}
	if c.AI.QwenBaseURL == "" || c.AI.QwenModel == "" || c.AI.QwenAPIKeyEnv == "" {
		warnings = append(warnings, "optional Qwen integration is disabled; configure ai.qwen_base_url, ai.qwen_model, and ai.qwen_api_key_env to enable it")
	} else if _, ok := os.LookupEnv(c.AI.QwenAPIKeyEnv); !ok {
		warnings = append(warnings, fmt.Sprintf("set environment variable %s for Qwen", c.AI.QwenAPIKeyEnv))
	}
	return warnings
}
