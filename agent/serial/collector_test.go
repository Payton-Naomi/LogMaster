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
