package logservice

import (
	"sync"
	"time"
)

const (
	searchCacheTTL        = 15 * time.Minute
	searchCacheMaxBytes   = 32 << 20
	searchCacheEntryBytes = 4 << 20
)

type logSearchCacheKey struct {
	UploadID      string
	Files         string
	Keyword       string
	CaseSensitive bool
}

type logSearchCacheEntry struct {
	matches   []logSearchMatch
	bytes     int
	expiresAt time.Time
}

type logSearchInflight struct {
	done    chan struct{}
	matches []logSearchMatch
	err     error
}

type logSearchCache struct {
	mu       sync.Mutex
	entries  map[logSearchCacheKey]logSearchCacheEntry
	inflight map[logSearchCacheKey]*logSearchInflight
	bytes    int
	now      func() time.Time
}

func newLogSearchCache() *logSearchCache {
	return &logSearchCache{entries: make(map[logSearchCacheKey]logSearchCacheEntry), inflight: make(map[logSearchCacheKey]*logSearchInflight), now: time.Now}
}

func (c *logSearchCache) getOrLoad(key logSearchCacheKey, load func() ([]logSearchMatch, error)) ([]logSearchMatch, error) {
	c.mu.Lock()
	c.removeExpiredLocked()
	if entry, ok := c.entries[key]; ok {
		matches := entry.matches
		c.mu.Unlock()
		return matches, nil
	}
	if pending, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-pending.done
		return pending.matches, pending.err
	}
	pending := &logSearchInflight{done: make(chan struct{})}
	c.inflight[key] = pending
	c.mu.Unlock()

	matches, err := load()
	entryBytes := searchMatchesSize(matches)
	c.mu.Lock()
	if err == nil && entryBytes <= searchCacheEntryBytes {
		c.ensureCapacityLocked(entryBytes)
		c.entries[key] = logSearchCacheEntry{matches: matches, bytes: entryBytes, expiresAt: c.now().Add(searchCacheTTL)}
		c.bytes += entryBytes
	}
	pending.matches, pending.err = matches, err
	delete(c.inflight, key)
	close(pending.done)
	c.mu.Unlock()
	return matches, err
}

func (c *logSearchCache) removeExpiredLocked() {
	now := c.now()
	for key, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			delete(c.entries, key)
			c.bytes -= entry.bytes
		}
	}
}

func (c *logSearchCache) ensureCapacityLocked(incoming int) {
	for c.bytes+incoming > searchCacheMaxBytes && len(c.entries) > 0 {
		var oldestKey logSearchCacheKey
		var oldest logSearchCacheEntry
		for key, entry := range c.entries {
			if oldest.expiresAt.IsZero() || entry.expiresAt.Before(oldest.expiresAt) {
				oldestKey, oldest = key, entry
			}
		}
		delete(c.entries, oldestKey)
		c.bytes -= oldest.bytes
	}
}

func searchMatchesSize(matches []logSearchMatch) int {
	size := 0
	for _, match := range matches {
		size += len(match.RelativePath) + len(match.Content) + 32
	}
	return size
}
