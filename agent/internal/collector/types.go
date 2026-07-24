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

const MaxSupportedDevices = 8

type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateCollecting   State = "collecting"
	StateReconnecting State = "reconnecting"
	StateDiskFull     State = "disk_full"
	StateError        State = "error"
)

type Rule struct {
	Name     string
	Keywords []string
	Pattern  string
	Severity string
	Module   string
}

type DeviceConfig struct {
	ID       string
	Name     string
	Serial   serialagent.SerialConfig
	Rules    []Rule
	MaxAge   time.Duration
	MaxBytes int64
}

type Config struct {
	MaxDevices     int
	EventCapacity  int
	SpoolDirectory string
	MaxDiskBytes   int64
	ProjectName    string
	Version        string
	Reconnect      serialagent.ReconnectConfig
	DiskCheckEvery time.Duration
}

type RuleHit struct {
	RuleName string `json:"rule_name"`
	Severity string `json:"severity"`
	Module   string `json:"module"`
	Count    uint64 `json:"count"`
}

type Event struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	TaskID     string    `json:"task_id,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
	Text       string    `json:"text,omitempty"`
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
