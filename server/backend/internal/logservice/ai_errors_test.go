package logservice

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyAIError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "none", want: ""},
		{name: "timeout", err: context.DeadlineExceeded, want: "timeout"},
		{name: "cancelled", err: context.Canceled, want: "cancelled"},
		{name: "authentication", err: errors.New("upstream returned HTTP 401 unauthorized"), want: "authentication"},
		{name: "rate limit", err: errors.New("HTTP 429 rate limit exceeded"), want: "rate_limit"},
		{name: "quota", err: errors.New("daily AI token quota exceeded"), want: "quota"},
		{name: "bad response", err: errors.New("decode agent response: invalid character"), want: "invalid_response"},
		{name: "upstream", err: errors.New("dial tcp: connection refused"), want: "upstream"},
		{name: "unknown", err: errors.New("model failed"), want: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyAIError(test.err); got != test.want {
				t.Fatalf("classifyAIError() = %q, want %q", got, test.want)
			}
		})
	}
}
