package auth

import (
	"strings"
	"testing"
)

func TestSafeFeishuMessageRemovesResponseInjectionAndLimitsLength(t *testing.T) {
	message := " token=secret\n\n" + strings.Repeat("x", 300)
	got := safeFeishuMessage(message)
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("message contains a newline: %q", got)
	}
	if len(got) > 256 {
		t.Fatalf("message length = %d, want <= 256", len(got))
	}
}
