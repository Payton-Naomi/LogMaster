package logservice

import (
	"testing"
	"time"
)

func TestLogSearchCacheReusesResultsAndCoalesces(t *testing.T) {
	cache := newLogSearchCache()
	key := logSearchCacheKey{UploadID: "upload", Files: "1:sha", Keyword: "error"}
	calls := 0
	load := func() ([]logSearchMatch, error) {
		calls++
		return []logSearchMatch{{FileID: 1, Content: "error line"}}, nil
	}
	if _, err := cache.getOrLoad(key, load); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.getOrLoad(key, load); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("loader called %d times, want 1", calls)
	}
}

func TestLogSearchCacheExpires(t *testing.T) {
	cache := newLogSearchCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	key := logSearchCacheKey{UploadID: "upload", Files: "1:sha", Keyword: "error"}
	calls := 0
	load := func() ([]logSearchMatch, error) { calls++; return []logSearchMatch{{Content: "error"}}, nil }
	_, _ = cache.getOrLoad(key, load)
	now = now.Add(searchCacheTTL + time.Second)
	_, _ = cache.getOrLoad(key, load)
	if calls != 2 {
		t.Fatalf("loader called %d times after expiry, want 2", calls)
	}
}
