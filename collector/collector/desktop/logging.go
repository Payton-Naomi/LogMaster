package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// logMaxBytes caps a single collector.log file; once exceeded the file is
	// rotated into a timestamped backup.
	logMaxBytes = 10 << 20
	// logMaxBackups keeps the most recent rotated files, older ones are pruned.
	logMaxBackups = 5
)

// appLogDirectory returns the directory that holds the collector's own
// diagnostic logs (not the collected device logs).
func appLogDirectory(root string) string { return filepath.Join(root, "logs") }

// rotatingFile is a size-based rotating log writer. It is safe for concurrent
// use; rotation happens inline on the goroutine that crosses the size limit.
type rotatingFile struct {
	mu         sync.Mutex
	directory  string
	name       string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

func openRotatingFile(directory, name string) (*rotatingFile, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	writer := &rotatingFile{directory: directory, name: name, maxBytes: logMaxBytes, maxBackups: logMaxBackups}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingFile) open() error {
	path := filepath.Join(w.directory, w.name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file, w.size = file, info.Size()
	return nil
}

func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil || w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFile) rotate() error {
	if w.file != nil {
		_ = w.file.Sync()
		if err := w.file.Close(); err != nil {
			w.file = nil
			return err
		}
		w.file = nil
	}
	current := filepath.Join(w.directory, w.name)
	if _, err := os.Stat(current); err == nil {
		backup := filepath.Join(w.directory, fmt.Sprintf("%s-%s%s", strings.TrimSuffix(w.name, filepath.Ext(w.name)), time.Now().Format("20060102-150405"), filepath.Ext(w.name)))
		if err := os.Rename(current, backup); err != nil {
			return err
		}
	}
	w.prune()
	return w.open()
}

func (w *rotatingFile) prune() {
	pattern := filepath.Join(w.directory, strings.TrimSuffix(w.name, filepath.Ext(w.name))+"-*"+filepath.Ext(w.name))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= w.maxBackups {
		return
	}
	sort.Strings(matches) // timestamped names sort chronologically
	for _, path := range matches[:len(matches)-w.maxBackups] {
		_ = os.Remove(path)
	}
}

func (w *rotatingFile) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}

func (w *rotatingFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Sync()
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	w.file = nil
	return err
}

func (s *Service) OpenAppLogDirectory() error {
	return s.OpenLogFolder(appLogDirectory(s.rootDirectory))
}

// LogPanic records a recovered panic with its stack trace. The desktop
// application has no visible console, so without this the crash reason would
// be lost entirely; writing it to collector.log lets a crash be diagnosed
// after the fact (采集端闪退后可查日志定位原因).
func (s *Service) LogPanic(recovered any) {
	stack := make([]byte, 1<<16)
	length := runtime.Stack(stack, false)
	if s.logger != nil {
		s.logger.Error("fatal panic", "component", "desktop.panic", "error", fmt.Sprint(recovered), "stack", string(stack[:length]))
	}
	if s.logWriter != nil {
		_ = s.logWriter.Sync()
	}
}

// LogFrontendError records a crash or unhandled rejection reported by the
// WebView frontend (window error / promise rejection hooks). UI failures are
// part of the "闪退" symptom space and belong in the same diagnostic log.
func (s *Service) LogFrontendError(source, message, stack string) {
	if s.logger == nil {
		return
	}
	s.logger.Error("frontend error", "component", "frontend."+source, "error", message, "stack", stack)
}

// newDesktopLoggerWithWriter opens the rolling diagnostic log under root/logs
// and returns a text slog.Logger together with the underlying writer so the
// caller can close it on shutdown.
func newDesktopLoggerWithWriter(root string) (*slog.Logger, *rotatingFile, error) {
	writer, err := openRotatingFile(appLogDirectory(root), "collector.log")
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})), writer, nil
}

// newDesktopLogger opens the rolling diagnostic log under root/logs and
// returns a slog.Logger writing text lines into it. If the log file cannot be
// created the default stderr logger is returned so the application still runs.
func newDesktopLogger(root string) *slog.Logger {
	logger, _, err := newDesktopLoggerWithWriter(root)
	if err != nil {
		slog.Default().Error("open collector diagnostic log failed, falling back to stderr", "error", err)
		return slog.Default()
	}
	return logger
}
