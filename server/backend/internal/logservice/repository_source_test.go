package logservice

import (
	"strings"
	"testing"
)

func TestUploadSourcePredicate(t *testing.T) {
	tests := map[string]string{
		"":          "",
		"collector": ` AND u.created_by_open_id = 'logmaster-internal-collector'`,
		"uploaded":  ` AND u.created_by_open_id <> 'logmaster-internal-collector'`,
		"invalid":   "",
	}
	for sourceType, expected := range tests {
		if actual := uploadSourcePredicate(sourceType); actual != expected {
			t.Fatalf("source %q predicate = %q, want %q", sourceType, actual, expected)
		}
	}
}

func TestCollectedSessionLookupRestrictsOwner(t *testing.T) {
	if !strings.Contains(collectorSessionByQueryCodeSQL, "created_by_open_id = $2") {
		t.Fatalf("collector session lookup must restrict the session owner: %q", collectorSessionByQueryCodeSQL)
	}
}
