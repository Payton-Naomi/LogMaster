package logservice

import (
	"errors"
	"testing"
)

func TestTaskRetryDisposition(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		manualQueued  bool
		alreadyQueued bool
		wantErr       bool
	}{
		{name: "failed task can be retried", status: "failed"},
		{name: "manual retry is idempotent while queued", status: "queued", manualQueued: true, alreadyQueued: true},
		{name: "initial queued task cannot retry", status: "queued", wantErr: true},
		{name: "running task cannot retry", status: "parsing", wantErr: true},
		{name: "completed task cannot retry", status: "completed", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			alreadyQueued, err := taskRetryDisposition(test.status, test.manualQueued)
			if alreadyQueued != test.alreadyQueued {
				t.Fatalf("alreadyQueued = %v, want %v", alreadyQueued, test.alreadyQueued)
			}
			if test.wantErr != errors.Is(err, ErrTaskNotRetryable) {
				t.Fatalf("error = %v, want ErrTaskNotRetryable = %v", err, test.wantErr)
			}
		})
	}
}
