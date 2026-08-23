package logservice

import (
	"context"
	"errors"
	"strings"
)

func classifyAIError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "401"), strings.Contains(message, "unauthorized"),
		strings.Contains(message, "api key"), strings.Contains(message, "authentication"):
		return "authentication"
	case strings.Contains(message, "429"), strings.Contains(message, "rate limit"):
		return "rate_limit"
	case strings.Contains(message, "quota"):
		return "quota"
	case strings.Contains(message, "deadline"), strings.Contains(message, "timeout"), strings.Contains(message, "timed out"):
		return "timeout"
	case strings.Contains(message, "decode"), strings.Contains(message, "invalid response"),
		strings.Contains(message, "parse agent response"), strings.Contains(message, "parse task overview"):
		return "invalid_response"
	case strings.Contains(message, "connection"), strings.Contains(message, "dial tcp"),
		strings.Contains(message, "502"), strings.Contains(message, "503"), strings.Contains(message, "upstream"):
		return "upstream"
	default:
		return "unknown"
	}
}
