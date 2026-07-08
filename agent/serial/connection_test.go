package serialagent

import (
	"errors"
	"testing"
)

// mockPort 实现 Port 接口，用于测试
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
	// 打开不存在的端口应返回错误
	_, err := OpenPort("/dev/nonexistent_serial_port_xyz", 9600)
	if err == nil {
		t.Fatal("OpenPort() 使用无效端口应返回错误")
	}
	t.Logf("收到预期错误: %v", err)
}

func TestClosePort(t *testing.T) {
	p := &mockPort{}
	err := p.Close()
	if err != nil {
		t.Fatalf("关闭端口时不应出错: %v", err)
	}
	if !p.closed {
		t.Fatal("Close() 应标记端口为已关闭")
	}
}

func TestClosePortTwice(t *testing.T) {
	p := &mockPort{}
	p.Close()
	err := p.Close()
	if err == nil {
		t.Fatal("第二次 Close() 应返回错误")
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
		t.Fatalf("Read() 读取 %d 字节, 期望 5", n)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("Read() 得到 %q, 期望 %q", string(buf[:n]), "hello")
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
		t.Fatalf("Write() 写入 %d 字节, 期望 5", n)
	}
	if string(m.writeBuf) != "world" {
		t.Fatalf("Write() 写入 %q, 期望 %q", string(m.writeBuf), "world")
	}
}

func TestReadWritePort(t *testing.T) {
	m := &mockPort{}

	// 写入数据
	data := []byte("ping")
	n, err := m.Write(data)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 4 {
		t.Fatalf("Write() 写入 %d 字节, 期望 4", n)
	}

	// 读取数据（通过设置 readBuf 模拟回显）
	m.readBuf = []byte("pong")
	buf := make([]byte, 10)
	n, err = m.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if string(buf[:n]) != "pong" {
		t.Fatalf("Read() 得到 %q, 期望 %q", string(buf[:n]), "pong")
	}
}