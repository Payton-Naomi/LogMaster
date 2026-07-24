package segment

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriterSealsAtomicallyAndDeliversMetadata(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 7, 20, 8, 30, 0, 0, time.UTC)
	var completed []Completed
	writer, err := NewWriter(Config{
		Directory: directory, DeviceSN: "DVR/2026:0001", PortName: "COM3",
		SessionStart: now, MaxAge: 5 * time.Minute, MaxBytes: 32 << 20,
		Now: func() time.Time { return now },
	}, func(_ context.Context, metadata Completed) error {
		if _, err := os.Stat(metadata.Path); err != nil {
			t.Fatalf("callback ran before final file was visible: %v", err)
		}
		completed = append(completed, metadata)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	entries := []Entry{
		{Sequence: 12001, CapturedAt: now, Text: "INFO first"},
		{Sequence: 12884, CapturedAt: now.Add(time.Millisecond), Text: "ERROR second"},
	}
	for _, entry := range entries {
		if err := writer.Write(context.Background(), entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 {
		t.Fatalf("completed segments = %d", len(completed))
	}
	metadata := completed[0]
	if got, want := filepath.Base(metadata.Path), "DVR_2026_0001_COM3_20260720T083000Z_12001-12884.log"; got != want {
		t.Fatalf("filename = %q, want %q", got, want)
	}
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if metadata.SHA256 != fmt.Sprintf("%x", digest) || metadata.SizeBytes != int64(len(content)) {
		t.Fatalf("incorrect metadata: %#v", metadata)
	}
	if !strings.Contains(string(content), "INFO first") || !strings.Contains(string(content), "ERROR second") {
		t.Fatalf("segment content = %q", content)
	}
	if temporary, _ := filepath.Glob(filepath.Join(directory, "*.tmp")); len(temporary) != 0 {
		t.Fatalf("temporary files remain: %v", temporary)
	}
}

func TestWriterRotatesBySizeBeforeNextEntry(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 7, 20, 8, 30, 0, 0, time.UTC)
	lineSize := int64(len(formatEntry(Entry{Sequence: 1, CapturedAt: now, Text: "a"})))
	var completed []Completed
	writer, err := NewWriter(Config{
		Directory: directory, DeviceSN: "D", PortName: "COM3", SessionStart: now,
		MaxAge: time.Hour, MaxBytes: lineSize + 1, Now: func() time.Time { return now },
	}, func(_ context.Context, metadata Completed) error {
		completed = append(completed, metadata)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 1, CapturedAt: now, Text: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 2, CapturedAt: now, Text: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(completed) != 2 || completed[0].FirstSequence != 1 || completed[0].LastSequence != 1 || completed[1].FirstSequence != 2 {
		t.Fatalf("unexpected size rotations: %#v", completed)
	}
}

func TestWriterRotatesByAge(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 7, 20, 8, 30, 0, 0, time.UTC)
	clock := now
	var completed []Completed
	writer, err := NewWriter(Config{
		Directory: directory, DeviceSN: "D", PortName: "COM3", SessionStart: now,
		MaxAge: 5 * time.Minute, MaxBytes: 1 << 20, Now: func() time.Time { return clock },
	}, func(_ context.Context, metadata Completed) error {
		completed = append(completed, metadata)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 1, CapturedAt: now, Text: "first"}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(5 * time.Minute)
	if err := writer.Write(context.Background(), Entry{Sequence: 2, CapturedAt: clock, Text: "second"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(completed) != 2 || completed[0].LastSequence != 1 || completed[1].FirstSequence != 2 {
		t.Fatalf("unexpected age rotations: %#v", completed)
	}
}

func TestWriterRejectsSequenceRegression(t *testing.T) {
	now := time.Now()
	writer, err := NewWriter(Config{
		Directory: t.TempDir(), DeviceSN: "D", PortName: "P", SessionStart: now,
		MaxAge: time.Minute, MaxBytes: 1024,
	}, func(context.Context, Completed) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 2, CapturedAt: now, Text: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 2, CapturedAt: now, Text: "b"}); err == nil {
		t.Fatal("expected sequence regression error")
	}
	if err := writer.Rotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 1, CapturedAt: now, Text: "c"}); err == nil {
		t.Fatal("expected sequence regression after rotation")
	}
	_ = writer.Close(context.Background())
}

func TestWriterLeavesCompletedFileWhenDeliveryFails(t *testing.T) {
	now := time.Now()
	directory := t.TempDir()
	writer, err := NewWriter(Config{
		Directory: directory, DeviceSN: "D", PortName: "P", SessionStart: now,
		MaxAge: time.Minute, MaxBytes: 1024,
	}, func(context.Context, Completed) error { return fmt.Errorf("queue unavailable") })
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(context.Background(), Entry{Sequence: 1, CapturedAt: now, Text: "line"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(context.Background()); err == nil {
		t.Fatal("expected delivery error")
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("completed file was not retained: %v, %v", files, err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got, want := SanitizeFilename("A/B:C D"), "A_B_C_D"; got != want {
		t.Fatalf("SanitizeFilename = %q, want %q", got, want)
	}
	if got := SanitizeFilename(".."); got != "_" {
		t.Fatalf("unsafe dot name = %q", got)
	}
}

func TestRecoverSealsTemporaryAndRedeliversCompleted(t *testing.T) {
	directory := t.TempDir()
	tmp := filepath.Join(directory, "D_COM3_20260720T083000Z_7.log.tmp")
	if err := os.WriteFile(tmp, []byte("one\ntwo\npartial"), 0o600); err != nil {
		t.Fatal(err)
	}
	var delivered []Completed
	count, err := Recover(context.Background(), Config{Directory: directory, DeviceSN: "D", PortName: "COM3"}, func(_ context.Context, completed Completed) error {
		delivered = append(delivered, completed)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(delivered) != 1 || delivered[0].FirstSequence != 7 || delivered[0].LastSequence != 8 {
		t.Fatalf("unexpected recovery: count=%d delivered=%+v", count, delivered)
	}
	content, err := os.ReadFile(delivered[0].Path)
	if err != nil || string(content) != "one\ntwo\n" {
		t.Fatalf("recovered content = %q, %v", content, err)
	}
}
