package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func NewLogger(path, level string) (*slog.Logger, func() error, error) {
	if path == "" {
		return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: parseLevel(level)})), func() error { return nil }, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, nil, fmt.Errorf("open agent log: %w", err)
	}
	handler := slog.NewJSONHandler(io.MultiWriter(os.Stderr, file), &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler), file.Close, nil
}

func parseLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
