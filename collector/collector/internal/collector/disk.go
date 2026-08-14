package collector

import (
	"io/fs"
	"path/filepath"
	"sync"
	"time"
)

type diskGuard struct {
	directory string
	limit     int64
	interval  time.Duration
	mu        sync.Mutex
	nextCheck time.Time
	exceeded  bool
	measure   func(string) (int64, error)
}

func newDiskGuard(directory string, limit int64, interval time.Duration) *diskGuard {
	if interval <= 0 {
		interval = time.Second
	}
	return &diskGuard{directory: directory, limit: limit, interval: interval, measure: directoryBytes}
}

func (g *diskGuard) Exceeded(now time.Time) (bool, error) {
	if g.limit <= 0 {
		return false, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if now.Before(g.nextCheck) {
		return g.exceeded, nil
	}
	used, err := g.measure(g.directory)
	if err != nil {
		return false, err
	}
	g.exceeded = used >= g.limit
	g.nextCheck = now.Add(g.interval)
	return g.exceeded, nil
}

func directoryBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
