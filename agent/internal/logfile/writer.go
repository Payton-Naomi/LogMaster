package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
)

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Writer struct {
	dir string
	mu  sync.Mutex
}

func New(dir string) *Writer { return &Writer{dir: dir} }

func (w *Writer) Append(entry model.LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	device := unsafeName.ReplaceAllString(entry.DeviceSN, "_")
	dir := filepath.Join(w.dir, device)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, entry.CapturedAt.Local().Format("2006-01-02")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "[%s] %s\n", entry.CapturedAt.Local().Format("2006-01-02 15:04:05.000"), entry.Content)
	return err
}

func PathFor(dir, deviceSN string, day time.Time) string {
	return filepath.Join(dir, unsafeName.ReplaceAllString(deviceSN, "_"), day.Local().Format("2006-01-02")+".log")
}
