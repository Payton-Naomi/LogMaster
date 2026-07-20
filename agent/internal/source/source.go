package source

import (
	"context"
	"fmt"
	"log/slog"

	"logmaster-agent/agent/internal/config"
)

type Source interface {
	Run(context.Context, func(string) error) error
}

func New(cfg config.DeviceConfig, logger *slog.Logger) (Source, error) {
	switch cfg.Source {
	case "serial":
		return NewSerial(cfg, logger), nil
	case "mock":
		return NewMock(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported source %q", cfg.Source)
	}
}
