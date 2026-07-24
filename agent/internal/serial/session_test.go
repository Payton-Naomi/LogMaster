package serial

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type fakeFactory struct {
	port Port
	err  error
}

func (f fakeFactory) Open(context.Context, SerialConfig) (Port, error) { return f.port, f.err }

type readResult struct {
	data []byte
	err  error
}

type fakePort struct {
	reads  []readResult
	index  int
	closed bool
	onRead func(int)
}

func (p *fakePort) Read(dst []byte) (int, error) {
	if p.onRead != nil {
		p.onRead(p.index)
	}
	if p.index >= len(p.reads) {
		return 0, io.EOF
	}
	result := p.reads[p.index]
	p.index++
	return copy(dst, result.data), result.err
}
func (*fakePort) Write(data []byte) (int, error) { return len(data), nil }
func (*fakePort) SetDTR(bool) error              { return nil }
func (*fakePort) SetRTS(bool) error              { return nil }
func (*fakePort) ResetInputBuffer() error        { return nil }
func (*fakePort) ResetOutputBuffer() error       { return nil }
func (p *fakePort) Close() error                 { p.closed = true; return nil }

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func validSerialConfig() SerialConfig {
	return SerialConfig{
		PortName: "COM3", BaudRate: 115200, DataBits: 8, StopBits: 1,
		Parity: ParityNone, Handshake: HandshakeNone, ReadTimeout: 100 * time.Millisecond,
		IdleGap: 10 * time.Millisecond, MaxFrameBytes: 10240, Encoding: EncodingUTF8,
	}
}

func TestSessionTreatsTimeoutAsPollingAndFlushesIdleFrame(t *testing.T) {
	port := &fakePort{reads: []readResult{{data: []byte("payload")}, {err: timeoutError{}}, {err: io.EOF}}}
	var frames []Frame
	session, err := NewSession(validSerialConfig(), fakeFactory{port: port}, func(_ context.Context, frame Frame) error {
		frames = append(frames, frame)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	times := []time.Time{time.Unix(1, 0), time.Unix(1, int64(11*time.Millisecond)), time.Unix(1, int64(12*time.Millisecond))}
	index := 0
	session.now = func() time.Time { value := times[index]; index++; return value }
	if err := session.Run(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if len(frames) != 1 || string(frames[0].Data) != "payload" {
		t.Fatalf("unexpected frames: %#v", frames)
	}
	if !port.closed {
		t.Fatal("port was not closed")
	}
}

func TestSessionFlushesPendingFrameOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	port := &fakePort{reads: []readResult{{data: []byte("tail")}, {}}}
	port.onRead = func(index int) {
		if index == 1 {
			cancel()
		}
	}
	var frames []Frame
	session, _ := NewSession(validSerialConfig(), fakeFactory{port: port}, func(_ context.Context, frame Frame) error {
		frames = append(frames, frame)
		return nil
	})
	if err := session.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || string(frames[0].Data) != "tail" {
		t.Fatalf("pending frame was not flushed: %#v", frames)
	}
}

func TestSerialConfigRejectsUnsupportedHandshake(t *testing.T) {
	config := validSerialConfig()
	config.Handshake = HandshakeRTSCTS
	if err := config.Validate(); !errors.Is(err, ErrUnsupportedHandshake) {
		t.Fatalf("expected unsupported handshake error, got %v", err)
	}
}
