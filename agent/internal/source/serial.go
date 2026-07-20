package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/config"
	seriallib "go.bug.st/serial"
)

type Serial struct {
	cfg    config.DeviceConfig
	logger *slog.Logger
}

func NewSerial(cfg config.DeviceConfig, logger *slog.Logger) *Serial {
	return &Serial{cfg: cfg, logger: logger}
}

func (s *Serial) Run(ctx context.Context, emit func(string) error) error {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return nil
		}
		started := time.Now()
		err := s.readOnce(ctx, emit)
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(started) >= 10*time.Second {
			backoff = time.Second
		}
		s.logger.Warn("serial connection closed; retrying", "device", s.cfg.DeviceSN, "port", s.cfg.Port, "after", backoff, "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (s *Serial) readOnce(ctx context.Context, emit func(string) error) error {
	mode, err := serialMode(s.cfg)
	if err != nil {
		return err
	}
	port, err := seriallib.Open(s.cfg.Port, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", s.cfg.Port, err)
	}
	defer port.Close()
	if err := port.SetReadTimeout(s.cfg.ReadTimeout); err != nil {
		return fmt.Errorf("set read timeout: %w", err)
	}
	s.logger.Info("serial connected", "device", s.cfg.DeviceSN, "port", s.cfg.Port, "baud", s.cfg.BaudRate)

	splitter := NewLineSplitter(s.cfg.MaxLineBytes)
	buf := make([]byte, 4096)
	lastData := time.Now()
	for {
		if ctx.Err() != nil {
			if line := splitter.Flush(); line != "" {
				_ = emit(line)
			}
			return nil
		}
		n, readErr := port.Read(buf)
		if n > 0 {
			lastData = time.Now()
			for _, line := range splitter.Feed(buf[:n]) {
				if err := emit(line); err != nil {
					return err
				}
			}
		}
		if splitter.Pending() && time.Since(lastData) >= s.cfg.IdleFlush {
			if line := splitter.Flush(); line != "" {
				if err := emit(line); err != nil {
					return err
				}
			}
		}
		if readErr != nil {
			if line := splitter.Flush(); line != "" {
				_ = emit(line)
			}
			if errors.Is(readErr, io.EOF) {
				return readErr
			}
			return fmt.Errorf("read %s: %w", s.cfg.Port, readErr)
		}
	}
}

func serialMode(cfg config.DeviceConfig) (*seriallib.Mode, error) {
	mode := &seriallib.Mode{BaudRate: cfg.BaudRate, DataBits: cfg.DataBits}
	switch strings.ToLower(cfg.Parity) {
	case "none":
		mode.Parity = seriallib.NoParity
	case "odd":
		mode.Parity = seriallib.OddParity
	case "even":
		mode.Parity = seriallib.EvenParity
	case "mark":
		mode.Parity = seriallib.MarkParity
	case "space":
		mode.Parity = seriallib.SpaceParity
	default:
		return nil, fmt.Errorf("unsupported parity %q", cfg.Parity)
	}
	if cfg.StopBits == 1 {
		mode.StopBits = seriallib.OneStopBit
	} else {
		mode.StopBits = seriallib.TwoStopBits
	}
	return mode, nil
}
