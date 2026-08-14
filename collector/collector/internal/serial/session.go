package serial

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type Frame struct {
	CapturedAt time.Time
	Data       []byte
}

type FrameEmitter func(context.Context, Frame) error

type Session struct {
	config  SerialConfig
	factory PortFactory
	emit    FrameEmitter
	now     func() time.Time
}

func NewSession(config SerialConfig, factory PortFactory, emit FrameEmitter) (*Session, error) {
	if factory == nil {
		return nil, errors.New("serial port factory is required")
	}
	if emit == nil {
		return nil, errors.New("serial frame emitter is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Session{config: config, factory: factory, emit: emit, now: time.Now}, nil
}

func (s *Session) Run(ctx context.Context) error {
	port, err := s.factory.Open(ctx, s.config)
	if err != nil {
		return err
	}
	defer port.Close()
	return s.readLoop(ctx, port)
}

func (s *Session) readLoop(ctx context.Context, port Port) (runErr error) {
	framer := NewIdleGapFramer(s.config.IdleGap, s.config.MaxFrameBytes)
	defer func() {
		if frame, ok := framer.Flush(); ok {
			flushCtx := context.WithoutCancel(ctx)
			if err := s.emit(flushCtx, Frame{CapturedAt: s.now(), Data: frame}); runErr == nil && err != nil {
				runErr = fmt.Errorf("flush serial frame: %w", err)
			}
		}
	}()

	buffer := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		n, readErr := port.Read(buffer)
		now := s.now()
		if n < 0 || n > len(buffer) {
			return fmt.Errorf("serial read returned invalid byte count %d", n)
		}
		if n > 0 {
			for _, frame := range framer.Push(now, buffer[:n]) {
				if err := s.emit(ctx, Frame{CapturedAt: now, Data: frame}); err != nil {
					return fmt.Errorf("emit serial frame: %w", err)
				}
			}
		}
		if frame, ok := framer.FlushIfIdle(now); ok {
			if err := s.emit(ctx, Frame{CapturedAt: now, Data: frame}); err != nil {
				return fmt.Errorf("emit idle serial frame: %w", err)
			}
		}
		if readErr != nil && !IsReadTimeout(readErr) {
			if errors.Is(readErr, io.EOF) {
				return io.EOF
			}
			return fmt.Errorf("read serial port %s: %w", s.config.PortName, readErr)
		}
	}
}

func IsReadTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	type timeout interface{ Timeout() bool }
	var timeoutErr timeout
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}
