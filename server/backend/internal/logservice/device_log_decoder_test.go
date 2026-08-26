package logservice

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeDeviceLogContent(t *testing.T) {
	plain := bytes.Repeat([]byte("2026-08-26 15:00:00 INFO device log line\n"), 80)
	encoded := encodeDeviceLogForTest(plain, 0)
	decoded, changed := decodeDeviceLogContent(encoded)
	if !changed {
		t.Fatal("encoded device log was not detected")
	}
	if !bytes.Equal(decoded, plain) {
		t.Fatal("decoded content differs from original")
	}
}

func TestDecodeMixedDeviceLogContent(t *testing.T) {
	prefix := bytes.Repeat([]byte("2026-08-26 INFO plaintext\n"), 200)
	suffix := bytes.Repeat([]byte("2026-08-26 ERROR encoded\n"), 80)
	plain := append(append([]byte(nil), prefix...), suffix...)
	encoded := append(append([]byte(nil), prefix...), encodeDeviceLogForTest(suffix, len(prefix))...)
	boundary := encodedLogBoundary(encoded)
	decoded, changed := decodeDeviceLogContent(encoded)
	if !changed || !bytes.Equal(decoded, plain) {
		t.Fatalf("mixed device log was not restored; boundary=%d want=%d", boundary, len(prefix))
	}
}

func TestDecodeDeviceLogFilesKeepsOriginalDirectUpload(t *testing.T) {
	root := t.TempDir()
	originalPath := filepath.Join(root, "original", "exported-device.log")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o750); err != nil {
		t.Fatal(err)
	}
	plain := bytes.Repeat([]byte("2026-08-26 INFO device log line\n"), 80)
	encoded := encodeDeviceLogForTest(plain, 0)
	if err := os.WriteFile(originalPath, encoded, 0o640); err != nil {
		t.Fatal(err)
	}
	files := []LogFile{{RelativePath: "original/exported-device.log"}}
	count, err := decodeDeviceLogFiles(root, files)
	if err != nil || count != 1 {
		t.Fatalf("decode count=%d err=%v", count, err)
	}
	if files[0].RelativePath != "decoded/original/exported-device.log" {
		t.Fatalf("decoded path = %q", files[0].RelativePath)
	}
	if original, err := os.ReadFile(originalPath); err != nil || !bytes.Equal(original, encoded) {
		t.Fatal("original upload was changed")
	}
	decoded, err := os.ReadFile(filepath.Join(root, "decoded", "original", "exported-device.log"))
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatal("decoded copy was not written")
	}
}

func TestDecodeDeviceLogContentKeepsPlaintext(t *testing.T) {
	plain := bytes.Repeat([]byte("ordinary text log\n"), 80)
	decoded, changed := decodeDeviceLogContent(plain)
	if changed || !bytes.Equal(decoded, plain) {
		t.Fatal("plain text log should remain unchanged")
	}
}

func encodeDeviceLogForTest(plain []byte, baseOffset int) []byte {
	encoded := make([]byte, len(plain))
	previous := byte(baseOffset)
	for index, value := range plain {
		offset := baseOffset + index
		encoded[index] = value ^ deviceLogXORKey[offset&7] ^ byte(offset*0x9d) ^ previous
		previous = encoded[index]
	}
	return encoded
}
