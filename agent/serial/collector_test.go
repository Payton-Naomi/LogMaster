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
		t.Fatal("LogLine.Device mismatch")
	}
	if line.Content != "test log" {
		t.Fatal("LogLine.Content mismatch")
	}
}

func TestCollectorNew(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	if c.Lines() == nil {
		t.Fatal("Lines() returned nil channel")
	}
}

func TestCollectorStopWithoutStart(t *testing.T) {
	c := NewCollector()
	c.Stop() // should not panic
}

func TestCollectorStartInvalidPort(t *testing.T) {
	c := NewCollector()
	err := c.Start("/dev/nonexistent_serial_xyz", 9600)
	if err == nil {
		t.Fatal("Start() should return error for invalid port")
	}
	// Ensure Stop() still works after failed start
	c.Stop()
}

func TestCollectorStartStop(t *testing.T) {
	c := NewCollector()
	// Start with invalid port should fail, stop should not panic
	c.Start("/dev/nonexistent", 9600)
	c.Stop()
}

func TestCollectorMultipleStop(t *testing.T) {
	c := NewCollector()
	c.Stop()
	c.Stop() // should not panic on double stop
}
