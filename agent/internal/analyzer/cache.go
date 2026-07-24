package analyzer

import (
	"context"
	"sync"
	"time"
)

type Cache interface {
	Get(context.Context, string) (AnalysisResponse, bool, error)
	Set(context.Context, string, AnalysisResponse, time.Time) error
}

type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	now     func() time.Time
}

type cacheEntry struct {
	response  AnalysisResponse
	expiresAt time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: make(map[string]cacheEntry), now: time.Now}
}

func (c *MemoryCache) Get(ctx context.Context, key string) (AnalysisResponse, bool, error) {
	if err := ctx.Err(); err != nil {
		return AnalysisResponse{}, false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return AnalysisResponse{}, false, nil
	}
	if !entry.expiresAt.After(c.now()) {
		delete(c.entries, key)
		return AnalysisResponse{}, false, nil
	}
	return cloneResponse(entry.response), true, nil
}

func (c *MemoryCache) Set(ctx context.Context, key string, response AnalysisResponse, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.entries[key] = cacheEntry{response: cloneResponse(response), expiresAt: expiresAt}
	c.mu.Unlock()
	return nil
}

func cloneResponse(response AnalysisResponse) AnalysisResponse {
	if response.Findings != nil {
		findings := make([]Finding, len(response.Findings))
		copy(findings, response.Findings)
		response.Findings = findings
	}
	return response
}

type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done     chan struct{}
	response AnalysisResponse
	err      error
}

func (g *flightGroup) Do(ctx context.Context, key string, fn func() (AnalysisResponse, error)) (AnalysisResponse, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*flightCall)
	}
	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		select {
		case <-call.done:
			return cloneResponse(call.response), call.err
		case <-ctx.Done():
			return AnalysisResponse{}, ctx.Err()
		}
	}
	call := &flightCall{done: make(chan struct{})}
	g.calls[key] = call
	g.mu.Unlock()

	call.response, call.err = fn()
	close(call.done)
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return cloneResponse(call.response), call.err
}
