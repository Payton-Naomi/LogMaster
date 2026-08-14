package serial

import (
	"testing"
	"time"
)

func TestExponentialBackoffAndJitterBounds(t *testing.T) {
	config := DefaultReconnectConfig()
	wantBase := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for attempt, base := range wantBase {
		low := ExponentialBackoff(attempt, config, 0)
		high := ExponentialBackoff(attempt, config, 1)
		if low != time.Duration(float64(base)*0.8) || high != time.Duration(float64(base)*1.2) {
			t.Fatalf("attempt %d: got [%s,%s] for base %s", attempt, low, high, base)
		}
	}
}

func TestReconnectManagerResetsAfterStableStream(t *testing.T) {
	manager := NewReconnectManager(DefaultReconnectConfig())
	manager.random = func() float64 { return 0.5 }
	if got := manager.FailureDelay(0); got != time.Second {
		t.Fatalf("first delay = %s", got)
	}
	if got := manager.FailureDelay(0); got != 2*time.Second {
		t.Fatalf("second delay = %s", got)
	}
	if got := manager.FailureDelay(60 * time.Second); got != time.Second {
		t.Fatalf("stable reset delay = %s", got)
	}
}
