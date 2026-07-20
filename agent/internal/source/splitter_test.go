package source

import (
	"bytes"
	"testing"
)

func TestLineSplitterSupportsLineEndings(t *testing.T) {
	s := NewLineSplitter(1024)
	got := s.Feed([]byte("one\r\ntwo\nthree\rfour\r\n"))
	want := []string{"one", "two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLineSplitterPreservesSplitUTF8(t *testing.T) {
	s := NewLineSplitter(1024)
	raw := []byte("设备正常\r\n")
	if got := s.Feed(raw[:4]); len(got) != 0 {
		t.Fatalf("unexpected early lines: %#v", got)
	}
	got := s.Feed(raw[4:])
	if len(got) != 1 || got[0] != "设备正常" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineSplitterKeepsTrailingCR(t *testing.T) {
	s := NewLineSplitter(1024)
	if got := s.Feed([]byte("partial\r")); len(got) != 0 {
		t.Fatalf("unexpected lines: %#v", got)
	}
	got := s.Feed([]byte("\n"))
	if len(got) != 1 || got[0] != "partial" {
		t.Fatalf("got %#v", got)
	}
}

func TestLineSplitterFlushesRemainder(t *testing.T) {
	s := NewLineSplitter(1024)
	s.Feed([]byte("tail"))
	if got := s.Flush(); got != "tail" {
		t.Fatalf("got %q", got)
	}
	if s.Pending() {
		t.Fatal("splitter still has pending bytes")
	}
}

func TestLineSplitterBoundsLongLines(t *testing.T) {
	s := NewLineSplitter(4)
	got := s.Feed(bytes.Repeat([]byte{'x'}, 9))
	if len(got) != 2 || got[0] != "xxxx" || got[1] != "xxxx" {
		t.Fatalf("got %#v", got)
	}
	if tail := s.Flush(); tail != "x" {
		t.Fatalf("tail=%q", tail)
	}
}
