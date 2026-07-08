package uploader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LogEntry represents a single log line to upload.
type LogEntry struct {
	Device    string `json:"device"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Severity  string `json:"severity"`
	Category  string `json:"category"`
	RuleName  string `json:"rule_name,omitempty"`
}

// Uploader handles HTTP batch uploads to the central server.
type Uploader struct {
	endpoint  string
	apiKey    string
	interval  int
	batchSize int
	client    *http.Client
	queue     []LogEntry
}

// New creates a new Uploader.
func New(endpoint, apiKey string, interval, batchSize int) *Uploader {
	return &Uploader{
		endpoint:  endpoint,
		apiKey:    apiKey,
		interval:  interval,
		batchSize: batchSize,
		client:    &http.Client{Timeout: 30 * time.Second},
		queue:     make([]LogEntry, 0, batchSize),
	}
}

// Enqueue adds a log entry to the upload queue.
func (u *Uploader) Enqueue(entry LogEntry) {
	u.queue = append(u.queue, entry)
}

// ShouldFlush returns true if the queue has reached batch size.
func (u *Uploader) ShouldFlush() bool {
	return len(u.queue) >= u.batchSize
}

// Flush uploads all queued entries and clears the queue on success.
func (u *Uploader) Flush() error {
	if len(u.queue) == 0 {
		return nil
	}
	if err := u.Upload(u.queue); err != nil {
		return err
	}
	u.queue = u.queue[:0]
	return nil
}

// Upload sends a batch of log entries to the central server.
func (u *Uploader) Upload(batch []LogEntry) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if u.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+u.apiKey)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}