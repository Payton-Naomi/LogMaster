package serial

import (
	"math/rand"
	"sync"
	"time"
)

type ReconnectConfig struct {
	InitialDelay time.Duration
	Multiplier   float64
	MaxDelay     time.Duration
	Jitter       float64
	StableReset  time.Duration
}

func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		InitialDelay: time.Second,
		Multiplier:   2,
		MaxDelay:     30 * time.Second,
		Jitter:       0.2,
		StableReset:  60 * time.Second,
	}
}

type ReconnectManager struct {
	config   ReconnectConfig
	mu       sync.Mutex
	attempts int
	random   func() float64
}

func NewReconnectManager(config ReconnectConfig) *ReconnectManager {
	defaults := DefaultReconnectConfig()
	if config.InitialDelay <= 0 {
		config.InitialDelay = defaults.InitialDelay
	}
	if config.Multiplier < 1 {
		config.Multiplier = defaults.Multiplier
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = defaults.MaxDelay
	}
	if config.Jitter < 0 || config.Jitter > 1 {
		config.Jitter = defaults.Jitter
	}
	if config.StableReset <= 0 {
		config.StableReset = defaults.StableReset
	}
	return &ReconnectManager{config: config, random: rand.Float64}
}

func (m *ReconnectManager) FailureDelay(streamedFor time.Duration) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if streamedFor >= m.config.StableReset {
		m.attempts = 0
	}
	delay := ExponentialBackoff(m.attempts, m.config, m.random())
	m.attempts++
	return delay
}

func (m *ReconnectManager) Reset() {
	m.mu.Lock()
	m.attempts = 0
	m.mu.Unlock()
}

func (m *ReconnectManager) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

func ExponentialBackoff(attempt int, config ReconnectConfig, random float64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := float64(config.InitialDelay)
	for i := 0; i < attempt && delay < float64(config.MaxDelay); i++ {
		delay *= config.Multiplier
		if delay > float64(config.MaxDelay) {
			delay = float64(config.MaxDelay)
		}
	}
	if random < 0 {
		random = 0
	} else if random > 1 {
		random = 1
	}
	factor := 1 - config.Jitter + (2 * config.Jitter * random)
	return time.Duration(delay * factor)
}
