package logs

import (
	"strings"
	"testing"
	"time"
)

func TestUniqueFileNameAddsTimestampBeforeExtension(t *testing.T) {
	seen := map[string]bool{"app.log": true}
	used := func(name string) bool {
		key := strings.ToLower(name)
		if seen[key] {
			return true
		}
		seen[key] = true
		return false
	}
	now := time.Date(2026, time.July, 24, 16, 15, 30, 123456789, time.Local)

	first := uniqueFileName("app.log", used, now)
	second := uniqueFileName("app.log", used, now)

	if first != "app_20260724_161530_123.log" {
		t.Fatalf("first duplicate = %q", first)
	}
	if second != "app_20260724_161530_123_2.log" {
		t.Fatalf("second duplicate = %q", second)
	}
}

func TestUniqueFileNamePreservesTarGZExtension(t *testing.T) {
	used := func(name string) bool { return name == "logs.tar.gz" }
	now := time.Date(2026, time.July, 24, 16, 15, 30, 0, time.Local)

	if got := uniqueFileName("logs.tar.gz", used, now); got != "logs_20260724_161530_000.tar.gz" {
		t.Fatalf("uniqueFileName() = %q", got)
	}
}
