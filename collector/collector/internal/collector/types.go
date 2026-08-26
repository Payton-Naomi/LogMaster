package collector

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	serialagent "logmaster-agent/agent/internal/serial"
)

// MaxSupportedDevices is a safety ceiling for concurrently connected serial
// channels. It used to be 8, which real multi-device benches exceeded once USB
// hubs were involved (十几台机器同时抓). The number of actually usable ports
// is bounded by the OS (Windows COM1-COM256) and hub capacity, so keep the cap
// generous instead of enforcing a fixed small channel count.
const MaxSupportedDevices = 64

type State string

type DiskState string

const (
	DiskNormal   DiskState = "normal"
	DiskWarning  DiskState = "warning"
	DiskReadOnly DiskState = "read_only"
	DiskFull     DiskState = "full"
)

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateCollecting   State = "collecting"
	StateReconnecting State = "reconnecting"
	StateDiskFull     State = "disk_full"
	StateDiskWarning  State = "disk_warning"
	StateDiskReadOnly State = "disk_read_only"
	StateError        State = "error"
)

type ReconnectMode string

const (
	ReconnectAutomatic ReconnectMode = "automatic"
	ReconnectDisabled  ReconnectMode = "disabled"
)

type Rule struct {
	ID       string
	Name     string
	Keywords []string
	Pattern  string
	Severity string
	Module   string
}

type DeviceConfig struct {
	ID               string
	Name             string
	Serial           serialagent.SerialConfig
	Rules            []Rule
	MaxAge           time.Duration
	MaxBytes         int64
	Persist          bool
	UploadEnabled    bool
	PreviewOnly      bool
	LocalOnly        bool
	SessionID        string
	ProjectID        string
	ProjectName      string
	Version          string
	TestTaskID       string
	TestTaskName     string
	UploaderName     string
	UploaderEmail    string
	Remark           string
	ScenarioIDs      []string
	CollectorVersion string
	Timezone         string
	UploadSessionID  string
	QueryCode        string
	ConfigSnapshot   string
	StartedAt        time.Time
	ReconnectMode    ReconnectMode
	NoLogTimeout     time.Duration
}

func (c DeviceConfig) persistEnabled() bool { return c.Persist || !c.PreviewOnly }

func (c DeviceConfig) uploadEnabled() bool { return c.UploadEnabled || !c.LocalOnly }

type Config struct {
	MaxDevices            int
	EventCapacity         int
	SpoolDirectory        string
	MaxDiskBytes          int64
	StorageWarningPercent int
	ProjectName           string
	Version               string
	Reconnect             serialagent.ReconnectConfig
	DiskCheckEvery        time.Duration
}

type RuleHit struct {
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Severity string `json:"severity"`
	Module   string `json:"module"`
	Count    uint64 `json:"count"`
}

type Event struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	TaskID     string    `json:"task_id,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
	Text       string    `json:"text,omitempty"`
	Sequence   int64     `json:"sequence,omitempty"`
	State      State     `json:"state,omitempty"`
	Error      string    `json:"error,omitempty"`
	Hits       []RuleHit `json:"hits,omitempty"`
}

type DeviceState struct {
	DeviceID      string            `json:"device_id"`
	DeviceName    string            `json:"device_name"`
	PortName      string            `json:"port_name"`
	TaskID        string            `json:"task_id,omitempty"`
	State         State             `json:"state"`
	LastError     string            `json:"last_error,omitempty"`
	RuleCounts    map[string]uint64 `json:"rule_counts"`
	DroppedEvents uint64            `json:"dropped_events"`
	LinesReceived uint64            `json:"lines_received"`
	Reconnects    uint64            `json:"reconnects"`
	Enabled       bool              `json:"enabled"`
	Persisting    bool              `json:"persisting"`
	UploadEnabled bool              `json:"upload_enabled"`
	NoLogAlert    bool              `json:"no_log_alert"`
	LastLogAt     *time.Time        `json:"last_log_at,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
}

type compiledRule struct {
	rule  Rule
	re    *regexp.Regexp
	count atomic.Uint64
}

func compileRules(rules []Rule) ([]*compiledRule, error) {
	compiled := make([]*compiledRule, 0, len(rules))
	for _, rule := range rules {
		if strings.TrimSpace(rule.Name) == "" {
			return nil, errors.New("rule name is required")
		}
		if strings.TrimSpace(rule.ID) == "" {
			rule.ID = rule.Name
		}
		item := &compiledRule{rule: rule}
		if rule.Pattern != "" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return nil, err
			}
			item.re = re
		}
		compiled = append(compiled, item)
	}
	return compiled, nil
}

func (r *compiledRule) match(line string) bool {
	lower := strings.ToLower(line)
	for _, keyword := range r.rule.Keywords {
		if !strings.Contains(lower, strings.ToLower(keyword)) {
			return false
		}
	}
	return r.re == nil || r.re.MatchString(line)
}

type Discovery interface {
	List(context.Context) ([]serialagent.PortDescriptor, error)
}

type PortFactory interface {
	Open(context.Context, serialagent.SerialConfig) (serialagent.Port, error)
}
