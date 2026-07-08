package uploader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LogEntry 表示一条待上传的日志。
type LogEntry struct {
	Device    string `json:"device"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	Severity  string `json:"severity"`
	Category  string `json:"category"`
	RuleName  string `json:"rule_name,omitempty"`
}

// Uploader 处理向中心服务器的 HTTP 批量上传。
type Uploader struct {
	endpoint  string
	apiKey    string
	interval  int
	batchSize int
	client    *http.Client
	queue     []LogEntry
}

// New 创建一个新的 Uploader。
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

// Enqueue 将一条日志加入上传队列。
func (u *Uploader) Enqueue(entry LogEntry) {
	u.queue = append(u.queue, entry)
}

// ShouldFlush 返回队列是否已达到批量大小。
func (u *Uploader) ShouldFlush() bool {
	return len(u.queue) >= u.batchSize
}

// Flush 上传所有队列中的日志，成功后清空队列。
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

// Upload 将一批日志发送到中心服务器。
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