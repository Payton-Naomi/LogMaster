package serial

import (
	"strings"
	"testing"
	"time"
)

func TestDecoderLineEndingsAndCrossFrameHalfLine(t *testing.T) {
	decoder, err := NewDecoder(EncodingUTF8)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(10, 0)
	first := decoder.Push([]byte("first\r\nsec"), at)
	second := decoder.Push([]byte("ond\rthird\n"), at.Add(time.Second))
	if len(first) != 1 || first[0].Text != "first" {
		t.Fatalf("unexpected first lines: %#v", first)
	}
	if len(second) != 2 || second[0].Text != "second" || second[1].Text != "third" {
		t.Fatalf("unexpected second lines: %#v", second)
	}
	if decoder.PendingBytes() != 0 {
		t.Fatalf("unexpected pending bytes: %d", decoder.PendingBytes())
	}
}

func TestDecoderSuppressesLFFollowingCrossFrameCR(t *testing.T) {
	decoder, _ := NewDecoder(EncodingUTF8)
	first := decoder.Push([]byte("line\r"), time.Now())
	second := decoder.Push([]byte("\nnext\n"), time.Now())
	if len(first) != 1 || first[0].Text != "line" {
		t.Fatalf("unexpected first frame: %#v", first)
	}
	if len(second) != 1 || second[0].Text != "next" {
		t.Fatalf("cross-frame CRLF produced an extra line: %#v", second)
	}
}

func TestDecoderEncodingsAndInvalidByteCount(t *testing.T) {
	gb, _ := NewDecoder(EncodingGB18030)
	gbLines := gb.Push([]byte{0xd6, 0xd0, 0xce, 0xc4, '\n'}, time.Now())
	if len(gbLines) != 1 || gbLines[0].Text != "中文" {
		t.Fatalf("unexpected GB18030 decoding: %#v", gbLines)
	}

	utf, _ := NewDecoder(EncodingUTF8)
	utfLines := utf.Push([]byte{'a', 0xff, 'b', '\n'}, time.Now())
	if len(utfLines) != 1 || utfLines[0].Text != "a�b" || utf.InvalidBytes() != 1 {
		t.Fatalf("unexpected invalid UTF-8 handling: %#v invalid=%d", utfLines, utf.InvalidBytes())
	}

	ascii, _ := NewDecoder(EncodingASCII)
	asciiLines := ascii.Push([]byte{'a', 0x80, 0xff, '\n'}, time.Now())
	if len(asciiLines) != 1 || asciiLines[0].Text != "a��" || ascii.InvalidBytes() != 2 {
		t.Fatalf("unexpected invalid ASCII handling: %#v invalid=%d", asciiLines, ascii.InvalidBytes())
	}
	if formatted := string(asciiLines[0].Bytes()); !strings.HasSuffix(formatted, " a��\n") {
		t.Fatalf("formatted line lost text: %q", formatted)
	}
}

func TestDecoderFlushesOnlyPendingText(t *testing.T) {
	decoder, _ := NewDecoder(EncodingUTF8)
	decoder.Push([]byte("partial"), time.Now())
	line, ok := decoder.Flush(time.Now())
	if !ok || line.Text != "partial" {
		t.Fatalf("unexpected flush: %#v, %v", line, ok)
	}
	if _, ok := decoder.Flush(time.Now()); ok {
		t.Fatal("empty decoder flushed a line")
	}
}
