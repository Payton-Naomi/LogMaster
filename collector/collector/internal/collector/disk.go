package collector

import (
	"io/fs"
	"path/filepath"
	"sync"
	"time"
)

type diskGuard struct {
	directory      string
	limit          int64
	warningPercent int
	interval       time.Duration
	mu             sync.Mutex
	nextCheck      time.Time
	exceeded       bool
	state          DiskState
	lastErr        error
	measure        func(string) (int64, error)
}

func newDiskGuard(directory string, limit int64, warningPercent int, interval time.Duration) *diskGuard {
	if interval <= 0 {
		interval = time.Second
	}
	if warningPercent < 1 || warningPercent > 99 {
		warningPercent = 80
	}
	return &diskGuard{directory: directory, limit: limit, warningPercent: warningPercent, interval: interval, measure: directoryBytes, state: DiskNormal}
}

// State returns the cached protection level.  A zero limit disables protection.
func (g *diskGuard) State(now time.Time) (DiskState, error) {
	if g.limit <= 0 {
		return DiskNormal, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if now.Before(g.nextCheck) {
		return g.state, g.lastErr
	}
	used, err := g.measure(g.directory)
	g.nextCheck = now.Add(g.interval)
	if err != nil {
		// Keep the last known protection level. A transient WalkDir error must not
		// disconnect an otherwise healthy serial port.
		g.lastErr = err
		return g.state, err
	}
	g.lastErr = nil
	percent := float64(used) / float64(g.limit)
	warning := float64(g.warningPercent) / 100
	readOnly := float64(min(g.warningPercent+10, 99)) / 100
	switch {
	case percent >= 1:
		g.state = DiskFull
	case percent >= readOnly:
		g.state = DiskReadOnly
	case percent >= warning:
		g.state = DiskWarning
	default:
		g.state = DiskNormal
	}
	g.exceeded = g.state == DiskFull
	return g.state, nil
}

func (g *diskGuard) Exceeded(now time.Time) (bool, error) {
	state, err := g.State(now)
	return state == DiskFull, err
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
