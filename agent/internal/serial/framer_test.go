package serial

import (
	"bytes"
	"testing"
	"time"
)

func TestIdleGapFramerCopiesAndSplitsAtLimit(t *testing.T) {
	framer := NewIdleGapFramer(10*time.Millisecond, 4)
	input := []byte("abcdefghij")
	frames := framer.Push(time.Unix(1, 0), input)
	input[0] = 'X'
	if len(frames) != 2 || !bytes.Equal(frames[0], []byte("abcd")) || !bytes.Equal(frames[1], []byte("efgh")) {
		t.Fatalf("unexpected frames: %q", frames)
	}
	frame, ok := framer.Flush()
	if !ok || !bytes.Equal(frame, []byte("ij")) {
		t.Fatalf("unexpected trailing frame: %q, %v", frame, ok)
	}
}

func TestIdleGapFramerFlushesOnSilenceAndBeforeLatePush(t *testing.T) {
	start := time.Unix(1, 0)
	framer := NewIdleGapFramer(10*time.Millisecond, 10240)
	framer.Push(start, []byte("one"))
	if _, ok := framer.FlushIfIdle(start.Add(9 * time.Millisecond)); ok {
		t.Fatal("frame flushed before idle gap")
	}
	frame, ok := framer.FlushIfIdle(start.Add(10 * time.Millisecond))
	if !ok || string(frame) != "one" {
		t.Fatalf("unexpected idle frame: %q, %v", frame, ok)
	}

	framer.Push(start.Add(time.Second), []byte("two"))
	frames := framer.Push(start.Add(time.Second+11*time.Millisecond), []byte("three"))
	if len(frames) != 1 || string(frames[0]) != "two" {
		t.Fatalf("late push did not close preceding frame: %q", frames)
	}
}
