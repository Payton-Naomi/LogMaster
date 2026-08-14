package serial

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrPortNotFound         = errors.New("serial port not found")
	ErrAmbiguousPort        = errors.New("serial port match is ambiguous")
	ErrUnsupportedHandshake = errors.New("SERIAL_UNSUPPORTED_HANDSHAKE")
)

type Port interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	SetDTR(bool) error
	SetRTS(bool) error
	ResetInputBuffer() error
	ResetOutputBuffer() error
	Close() error
}

type PortFactory interface {
	Open(context.Context, SerialConfig) (Port, error)
}

type PortDiscovery interface {
	List(context.Context) ([]PortDescriptor, error)
}

type Parity string

const (
	ParityNone  Parity = "none"
	ParityOdd   Parity = "odd"
	ParityEven  Parity = "even"
	ParityMark  Parity = "mark"
	ParitySpace Parity = "space"
)

type Handshake string

const (
	HandshakeNone    Handshake = "none"
	HandshakeRTSCTS  Handshake = "rts_cts"
	HandshakeXONXOFF Handshake = "xon_xoff"
)

type Encoding string

const (
	EncodingUTF8    Encoding = "utf-8"
	EncodingGB18030 Encoding = "gb18030"
	EncodingASCII   Encoding = "ascii"
)

type State string

const (
	StateDisabled    State = "DISABLED"
	StateWaitingPort State = "WAITING_PORT"
	StateConnecting  State = "CONNECTING"
	StateStreaming   State = "STREAMING"
	StateBackoff     State = "BACKOFF"
	StateStopping    State = "STOPPING"
)

type SerialConfig struct {
	PortName      string
	BaudRate      int
	DataBits      int
	StopBits      int
	Parity        Parity
	Handshake     Handshake
	DTR           bool
	RTS           bool
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleGap       time.Duration
	MaxFrameBytes int
	Encoding      Encoding
}

func (c SerialConfig) Validate() error {
	if strings.TrimSpace(c.PortName) == "" {
		return errors.New("serial port name is required")
	}
	if c.BaudRate < 300 || c.BaudRate > 4_000_000 {
		return fmt.Errorf("serial baud rate %d is outside 300..4000000", c.BaudRate)
	}
	if c.DataBits < 5 || c.DataBits > 8 {
		return fmt.Errorf("serial data bits %d is not one of 5, 6, 7, 8", c.DataBits)
	}
	if c.StopBits != 1 && c.StopBits != 2 {
		return fmt.Errorf("serial stop bits %d is not 1 or 2", c.StopBits)
	}
	switch c.Parity {
	case ParityNone, ParityOdd, ParityEven, ParityMark, ParitySpace:
	default:
		return fmt.Errorf("unsupported serial parity %q", c.Parity)
	}
	if c.Handshake != HandshakeNone {
		return fmt.Errorf("%w: %s", ErrUnsupportedHandshake, c.Handshake)
	}
	if c.ReadTimeout <= 0 {
		return errors.New("serial read timeout must be positive")
	}
	if c.IdleGap < time.Millisecond || c.IdleGap > 2*time.Second {
		return fmt.Errorf("serial idle gap %s is outside 1ms..2s", c.IdleGap)
	}
	if c.MaxFrameBytes < 256 || c.MaxFrameBytes > 1_048_576 {
		return fmt.Errorf("serial max frame bytes %d is outside 256..1048576", c.MaxFrameBytes)
	}
	switch c.Encoding {
	case EncodingUTF8, EncodingGB18030, EncodingASCII:
	default:
		return fmt.Errorf("unsupported serial encoding %q", c.Encoding)
	}
	return nil
}

type PortDescriptor struct {
	Name         string
	VID          string
	PID          string
	USBSerial    string
	Location     string
	Manufacturer string
	Product      string
	IsUSB        bool
}

type PortMatch struct {
	USBSerial string
	VID       string
	PID       string
	Location  string
	PortName  string
}
