package upload

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
)

type Uploader interface {
	Upload(context.Context, model.UploadBatch) error
}

type HTTPUploader struct {
	url    string
	token  string
	client *http.Client
}

type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("upload returned HTTP %d: %s", e.StatusCode, e.Body)
}

func NewHTTP(url, token string, timeout time.Duration) *HTTPUploader {
	return &HTTPUploader{url: url, token: token, client: &http.Client{Timeout: timeout}}
}

func (u *HTTPUploader) Upload(ctx context.Context, batch model.UploadBatch) error {
	var body bytes.Buffer
	zipper := gzip.NewWriter(&body)
	if err := json.NewEncoder(zipper).Encode(batch); err != nil {
		return err
	}
	if err := zipper.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Idempotency-Key", batch.BatchID)
	if u.token != "" {
		req.Header.Set("Authorization", "Bearer "+u.token)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &StatusError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	var result model.UploadResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&result); err != nil {
		return fmt.Errorf("decode upload response: %w", err)
	}
	if !result.Accepted || result.BatchID != batch.BatchID || result.Received != len(batch.Logs) {
		return fmt.Errorf("invalid upload acknowledgement: accepted=%v batch_id=%q received=%d", result.Accepted, result.BatchID, result.Received)
	}
	return nil
}
