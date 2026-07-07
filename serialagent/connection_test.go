package serialagent

import (
	"errors"
	"testing"
)

// mockPort implements Port interface for testing
type mockPort struct {
	closed   bool
	readBuf  []byte
	writeBuf []byte
}

func (m *mockPort) Close() error {
	if m.closed {
		return errors.New("already closed")
	}
	m.closed = true
	return nil
}

func (m *mockPort) Read(p []byte) (int, error) {
	if m.closed {
		return 0, errors.New("port is closed")
	}
	if len(m.readBuf) == 0 {
		return 0, nil
	}
	n := copy(p, m.readBuf)
	m.readBuf = m.readBuf[n:]
	return n, nil
}

func (m *mockPort) Write(p []byte) (int, error) {
	if m.closed {
		return 0, errors.New("port is closed")
	}
	m.writeBuf = append(m.writeBuf, p...)
	return len(p), nil
}

func TestOpenPortInvalidPort(t *testing.T) {
	// Opening a non-existent port should return an error
	_, err := OpenPort("/dev/nonexistent_serial_port_xyz", 9600)
	if err == nil {
		t.Fatal("OpenPort() with invalid port should return error")
	}
	t.Logf("Got expected error: %v", err)
}

func TestClosePort(t *testing.T) {
	p := &mockPort{}
	err := p.Close()
	if err != nil {
		t.Fatalf("Close() on open port should not error: %v", err)
	}
	if !p.closed {
		t.Fatal("Close() should mark port as closed")
	}
}

func TestClosePortTwice(t *testing.T) {
	p := &mockPort{}
	p.Close()
	err := p.Close()
	if err == nil {
		t.Fatal("Close() twice should return error")
	}
}

func TestReadFromPort(t *testing.T) {
	m := &mockPort{readBuf: []byte("hello")}
	buf := make([]byte, 10)
	n, err := m.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Read() got %d bytes, want 5", n)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("Read() got %q, want %q", string(buf[:n]), "hello")
	}
}

func TestWriteToPort(t *testing.T) {
	m := &mockPort{}
	data := []byte("world")
	n, err := m.Write(data)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 5 {
		t.Fatalf("Write() got %d bytes, want 5", n)
	}
	if string(m.writeBuf) != "world" {
		t.Fatalf("Write() wrote %q, want %q", string(m.writeBuf), "world")
	}
}

func TestReadWritePort(t *testing.T) {
	m := &mockPort{}

	// Write data
	data := []byte("ping")
	n, err := m.Write(data)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 4 {
		t.Fatalf("Write() wrote %d bytes, want 4", n)
	}

	// Read data (simulate echo by setting readBuf)
	m.readBuf = []byte("pong")
	buf := make([]byte, 10)
	n, err = m.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if string(buf[:n]) != "pong" {
		t.Fatalf("Read() got %q, want %q", string(buf[:n]), "pong")
	}
}