package segment

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var invalidFilenameByte = regexp.MustCompile(`[^A-Za-z0-9._-]`)

type Config struct {
	Directory    string
	DeviceSN     string
	PortName     string
	SessionStart time.Time
	MaxAge       time.Duration
	MaxBytes     int64
	FileMode     os.FileMode
	Now          func() time.Time
}

type Entry struct {
	Sequence   int64
	CapturedAt time.Time
	Text       string
}

type Completed struct {
	Path          string
	SHA256        string
	SizeBytes     int64
	DeviceSN      string
	PortName      string
	SessionStart  time.Time
	FirstSequence int64
	LastSequence  int64
	CreatedAt     time.Time
	CompletedAt   time.Time
}

type Deliver func(context.Context, Completed) error

type Writer struct {
	config  Config
	deliver Deliver

	mu           sync.Mutex
	current      *openSegment
	closed       bool
	lastSequence int64
}

type openSegment struct {
	file          *os.File
	buffer        *bufio.Writer
	hash          hash.Hash
	tmpPath       string
	createdAt     time.Time
	firstSequence int64
	lastSequence  int64
	sizeBytes     int64
}

func NewWriter(config Config, deliver Deliver) (*Writer, error) {
	if config.Directory == "" {
		return nil, errors.New("segment directory is required")
	}
	if config.DeviceSN == "" {
		return nil, errors.New("segment device serial is required")
	}
	if config.PortName == "" {
		return nil, errors.New("segment port name is required")
	}
	if config.SessionStart.IsZero() {
		return nil, errors.New("segment session start is required")
	}
	if config.MaxAge <= 0 {
		return nil, errors.New("segment max age must be positive")
	}
	if config.MaxBytes <= 0 {
		return nil, errors.New("segment max bytes must be positive")
	}
	if deliver == nil {
		return nil, errors.New("segment delivery callback is required")
	}
	if config.FileMode == 0 {
		config.FileMode = 0o644
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if err := os.MkdirAll(config.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("create segment directory: %w", err)
	}
	return &Writer{config: config, deliver: deliver}, nil
}

func (w *Writer) Write(ctx context.Context, entry Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("segment writer is closed")
	}
	if entry.Sequence <= 0 {
		return errors.New("segment sequence must be positive")
	}
	if entry.CapturedAt.IsZero() {
		entry.CapturedAt = w.config.Now()
	}
	line := formatEntry(entry)
	now := w.config.Now()
	if w.lastSequence > 0 && entry.Sequence <= w.lastSequence {
		return fmt.Errorf("segment sequence %d does not follow %d", entry.Sequence, w.lastSequence)
	}
	if w.current != nil {
		ageExpired := now.Sub(w.current.createdAt) >= w.config.MaxAge
		sizeExceeded := w.current.sizeBytes > 0 && w.current.sizeBytes+int64(len(line)) > w.config.MaxBytes
		if ageExpired || sizeExceeded {
			if err := w.finalizeLocked(ctx, now); err != nil {
				return err
			}
		}
	}
	if w.current == nil {
		if err := w.openLocked(entry.Sequence, now); err != nil {
			return err
		}
	}
	written, err := w.current.buffer.Write(line)
	if err != nil {
		return fmt.Errorf("write segment: %w", err)
	}
	if written != len(line) {
		return fmt.Errorf("write segment: short write %d of %d", written, len(line))
	}
	if err := w.current.buffer.Flush(); err != nil {
		return fmt.Errorf("flush segment: %w", err)
	}
	if err := w.current.file.Sync(); err != nil {
		return fmt.Errorf("sync segment: %w", err)
	}
	_, _ = w.current.hash.Write(line)
	w.current.sizeBytes += int64(len(line))
	w.current.lastSequence = entry.Sequence
	w.lastSequence = entry.Sequence
	if w.current.sizeBytes >= w.config.MaxBytes {
		return w.finalizeLocked(ctx, now)
	}
	return nil
}

func (w *Writer) Rotate(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("segment writer is closed")
	}
	return w.finalizeLocked(ctx, w.config.Now())
}

func (w *Writer) Close(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.finalizeLocked(ctx, w.config.Now())
}

func (w *Writer) openLocked(firstSequence int64, now time.Time) error {
	base := fmt.Sprintf("%s_%s_%s_%d.log.tmp",
		SanitizeFilename(w.config.DeviceSN),
		SanitizeFilename(w.config.PortName),
		w.config.SessionStart.UTC().Format("20060102T150405Z"),
		firstSequence,
	)
	tmpPath := filepath.Join(w.config.Directory, base)
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, w.config.FileMode)
	if err != nil {
		return fmt.Errorf("create segment %s: %w", tmpPath, err)
	}
	w.current = &openSegment{
		file:          file,
		buffer:        bufio.NewWriterSize(file, 64*1024),
		hash:          sha256.New(),
		tmpPath:       tmpPath,
		createdAt:     now,
		firstSequence: firstSequence,
		lastSequence:  firstSequence - 1,
	}
	return nil
}

func (w *Writer) finalizeLocked(ctx context.Context, completedAt time.Time) error {
	current := w.current
	if current == nil {
		return nil
	}
	if current.lastSequence < current.firstSequence {
		_ = current.file.Close()
		return fmt.Errorf("segment %s contains no entries", current.tmpPath)
	}
	if err := current.buffer.Flush(); err != nil {
		_ = current.file.Close()
		return fmt.Errorf("flush segment %s: %w", current.tmpPath, err)
	}
	if err := current.file.Sync(); err != nil {
		_ = current.file.Close()
		return fmt.Errorf("sync segment %s: %w", current.tmpPath, err)
	}
	if err := current.file.Close(); err != nil {
		return fmt.Errorf("close segment %s: %w", current.tmpPath, err)
	}
	finalName := fmt.Sprintf("%s_%s_%s_%d-%d.log",
		SanitizeFilename(w.config.DeviceSN),
		SanitizeFilename(w.config.PortName),
		w.config.SessionStart.UTC().Format("20060102T150405Z"),
		current.firstSequence,
		current.lastSequence,
	)
	finalPath := filepath.Join(w.config.Directory, finalName)
	if err := os.Rename(current.tmpPath, finalPath); err != nil {
		return fmt.Errorf("seal segment %s: %w", current.tmpPath, err)
	}
	w.current = nil
	metadata := Completed{
		Path:          finalPath,
		SHA256:        fmt.Sprintf("%x", current.hash.Sum(nil)),
		SizeBytes:     current.sizeBytes,
		DeviceSN:      w.config.DeviceSN,
		PortName:      w.config.PortName,
		SessionStart:  w.config.SessionStart,
		FirstSequence: current.firstSequence,
		LastSequence:  current.lastSequence,
		CreatedAt:     current.createdAt,
		CompletedAt:   completedAt,
	}
	if err := w.deliver(ctx, metadata); err != nil {
		return fmt.Errorf("deliver completed segment %s: %w", finalPath, err)
	}
	return nil
}

func SanitizeFilename(value string) string {
	clean := invalidFilenameByte.ReplaceAllString(value, "_")
	if clean == "" || clean == "." || clean == ".." {
		return "_"
	}
	return clean
}

func formatEntry(entry Entry) []byte {
	return []byte(fmt.Sprintf("[%s] %s\n", entry.CapturedAt.UTC().Format("2006-01-02T15:04:05.000Z"), entry.Text))
}
