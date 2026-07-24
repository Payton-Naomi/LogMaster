package serial

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type DecodedLine struct {
	CapturedAt time.Time
	Text       string
}

func (line DecodedLine) Bytes() []byte {
	return []byte(fmt.Sprintf("[%s] %s\n", line.CapturedAt.UTC().Format("2006-01-02T15:04:05.000Z"), line.Text))
}

type Decoder struct {
	encoding     Encoding
	pending      []byte
	skipLF       bool
	invalidBytes uint64
}

func NewDecoder(encoding Encoding) (*Decoder, error) {
	switch encoding {
	case EncodingUTF8, EncodingGB18030, EncodingASCII:
		return &Decoder{encoding: encoding}, nil
	default:
		return nil, fmt.Errorf("unsupported serial encoding %q", encoding)
	}
}

func (d *Decoder) Push(frame []byte, capturedAt time.Time) []DecodedLine {
	var lines []DecodedLine
	for _, current := range frame {
		if d.skipLF {
			d.skipLF = false
			if current == '\n' {
				continue
			}
		}
		switch current {
		case '\r':
			lines = append(lines, d.emit(capturedAt))
			d.skipLF = true
		case '\n':
			lines = append(lines, d.emit(capturedAt))
		default:
			d.pending = append(d.pending, current)
		}
	}
	return lines
}

func (d *Decoder) Flush(capturedAt time.Time) (DecodedLine, bool) {
	d.skipLF = false
	if len(d.pending) == 0 {
		return DecodedLine{}, false
	}
	return d.emit(capturedAt), true
}

func (d *Decoder) PendingBytes() int { return len(d.pending) }

func (d *Decoder) InvalidBytes() uint64 { return d.invalidBytes }

func (d *Decoder) emit(capturedAt time.Time) DecodedLine {
	text, invalid := decodeText(d.encoding, d.pending)
	d.invalidBytes += uint64(invalid)
	d.pending = d.pending[:0]
	return DecodedLine{CapturedAt: capturedAt, Text: text}
}

func decodeText(encoding Encoding, source []byte) (string, int) {
	switch encoding {
	case EncodingASCII:
		var builder strings.Builder
		invalid := 0
		for _, value := range source {
			if value <= 0x7f {
				builder.WriteByte(value)
				continue
			}
			builder.WriteRune(utf8.RuneError)
			invalid++
		}
		return builder.String(), invalid
	case EncodingGB18030:
		decoded, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), source)
		if err != nil {
			decoded = bytes.ToValidUTF8(decoded, []byte(string(utf8.RuneError)))
		}
		return string(decoded), bytes.Count(decoded, []byte(string(utf8.RuneError)))
	default:
		if utf8.Valid(source) {
			return string(source), 0
		}
		return replaceInvalidUTF8(source)
	}
}

func replaceInvalidUTF8(source []byte) (string, int) {
	var builder strings.Builder
	invalid := 0
	for len(source) > 0 {
		r, size := utf8.DecodeRune(source)
		if r == utf8.RuneError && size == 1 {
			builder.WriteRune(utf8.RuneError)
			invalid++
			source = source[1:]
			continue
		}
		builder.WriteRune(r)
		source = source[size:]
	}
	return builder.String(), invalid
}
