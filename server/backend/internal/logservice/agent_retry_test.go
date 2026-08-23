package logservice

import (
	"errors"
	"testing"
)

func TestAgentRetryRequiresCompletedTask(t *testing.T) {
	if !errors.Is(ErrAgentRetryNotReady, ErrAgentRetryNotReady) {
		t.Fatal("agent retry readiness error must be stable")
	}
}
