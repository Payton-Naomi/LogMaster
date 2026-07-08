package serialagent

import (
	"testing"
	"time"
)

func TestLogLine(t *testing.T) {
	line := LogLine{
		Device:    "/dev/ttyUSB0",
		Timestamp: time.Now(),
		Content:   "test log",
	}
	if line.Device != "/dev/ttyUSB0" {
		t.Fatal("LogLine.Device 不匹配")
	}
	if line.Content != "test log" {
		t.Fatal("LogLine.Content 不匹配")
	}
}

func TestCollectorNew(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() 返回 nil")
	}
	if c.Lines() == nil {
		t.Fatal("Lines() 返回 nil 通道")
	}
}

func TestCollectorStopWithoutStart(t *testing.T) {
	c := NewCollector()
	c.Stop() // 不应 panic
}

func TestCollectorStartInvalidPort(t *testing.T) {
	c := NewCollector()
	err := c.Start("/dev/nonexistent_serial_xyz", 9600, 8, 1, "none")
	if err == nil {
		t.Fatal("Start() 对无效端口应返回错误")
	}
	// 确保启动失败后 Stop() 仍然正常工作
	c.Stop()
}

func TestCollectorStartStop(t *testing.T) {
	c := NewCollector()
	// 使用无效端口启动应失败，停止不应 panic
	c.Start("/dev/nonexistent", 9600, 8, 1, "none")
	c.Stop()
}

func TestCollectorMultipleStop(t *testing.T) {
	c := NewCollector()
	c.Stop()
	c.Stop() // 重复停止不应 panic
}
