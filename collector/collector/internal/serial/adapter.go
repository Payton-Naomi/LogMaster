package serial

import (
	"context"
	"fmt"

	seriallib "go.bug.st/serial"
)

type GoBugFactory struct{}

func (GoBugFactory) Open(ctx context.Context, cfg SerialConfig) (Port, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	mode, err := libraryMode(cfg)
	if err != nil {
		return nil, err
	}
	port, err := seriallib.Open(cfg.PortName, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial port %s: %w", cfg.PortName, err)
	}
	closeWith := func(setErr error, action string) (Port, error) {
		_ = port.Close()
		return nil, fmt.Errorf("%s on serial port %s: %w", action, cfg.PortName, setErr)
	}
	if err := port.SetReadTimeout(cfg.ReadTimeout); err != nil {
		return closeWith(err, "set read timeout")
	}
	if err := port.SetDTR(cfg.DTR); err != nil {
		return closeWith(err, "set DTR")
	}
	if err := port.SetRTS(cfg.RTS); err != nil {
		return closeWith(err, "set RTS")
	}
	return port, nil
}

func libraryMode(cfg SerialConfig) (*seriallib.Mode, error) {
	mode := &seriallib.Mode{BaudRate: cfg.BaudRate, DataBits: cfg.DataBits}
	switch cfg.Parity {
	case ParityNone:
		mode.Parity = seriallib.NoParity
	case ParityOdd:
		mode.Parity = seriallib.OddParity
	case ParityEven:
		mode.Parity = seriallib.EvenParity
	case ParityMark:
		mode.Parity = seriallib.MarkParity
	case ParitySpace:
		mode.Parity = seriallib.SpaceParity
	default:
		return nil, fmt.Errorf("unsupported serial parity %q", cfg.Parity)
	}
	switch cfg.StopBits {
	case 1:
		mode.StopBits = seriallib.OneStopBit
	case 2:
		mode.StopBits = seriallib.TwoStopBits
	default:
		return nil, fmt.Errorf("unsupported serial stop bits %d", cfg.StopBits)
	}
	return mode, nil
}
