package backend

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"logmaster-agent/agent/internal/spool"
)

const maxResponseBytes = 1 << 20

type FailureKind string

const (
	Retryable FailureKind = "retryable"
	Uncertain FailureKind = "uncertain"
	Rejected  FailureKind = "rejected"
	Pause     FailureKind = "pause"
	Split     FailureKind = "split"
)

type Failure struct {
	Kind       FailureKind
	StatusCode int
	RetryAfter time.Duration
	Message    string
	Err        error
}

func (e *Failure) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Failure) Unwrap() error { return e.Err }

type Config struct {
	BaseURL       string
	HealthPath    string
	InspectPath   string
	UploadPath    string
	Timeout       time.Duration
	Authorization string
	Gzip          bool
}

type Client struct {
	baseURL       string
	healthPath    string
	inspectPath   string
	uploadPath    string
	authorization string
	gzip          bool
	http          *http.Client
}

type APIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type UploadAccepted struct {
	UploadID  string `json:"upload_id"`
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	FileCount int    `json:"file_count"`
}

func New(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), healthPath: cfg.HealthPath,
		inspectPath: cfg.InspectPath, uploadPath: cfg.UploadPath,
		authorization: cfg.Authorization,
		gzip:          cfg.Gzip,
		http:          &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+c.healthPath, nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope APIResponse[struct {
		Status string `json:"status"`
	}]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 || envelope.Data.Status != "ok" {
		return fmt.Errorf("backend health rejected: HTTP %d code=%d status=%q", resp.StatusCode, envelope.Code, envelope.Data.Status)
	}
	return nil
}

func (c *Client) Inspect(ctx context.Context, file spool.File) error {
	batch := spool.Batch{Files: []spool.File{file}}
	reader, contentType, sent, writeDone := multipartBody(batch, false, c.gzip)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.inspectPath, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if c.gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return transportFailure(err, sent.Load())
	}
	defer resp.Body.Close()
	if err := <-writeDone; err != nil {
		return &Failure{Kind: Uncertain, Message: "inspect request body failed", Err: err}
	}
	var envelope APIResponse[json.RawMessage]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: "inspect response invalid", Err: err}
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		return classifyResponse(resp.StatusCode, envelope.Message)
	}
	return nil
}

func (c *Client) Upload(ctx context.Context, batch spool.Batch) (UploadAccepted, error) {
	reader, contentType, sent, writeDone := multipartBody(batch, true, c.gzip)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.uploadPath, reader)
	if err != nil {
		return UploadAccepted{}, err
	}
	req.Header.Set("Content-Type", contentType)
	if c.gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return UploadAccepted{}, transportFailure(err, sent.Load())
	}
	defer resp.Body.Close()
	if err := <-writeDone; err != nil {
		return UploadAccepted{}, &Failure{Kind: Uncertain, Message: "upload request body failed", Err: err}
	}
	var envelope APIResponse[UploadAccepted]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: "upload acknowledgement invalid", Err: err}
	}
	if resp.StatusCode != http.StatusAccepted || envelope.Code != 0 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: fmt.Sprintf("unexpected upload acknowledgement: HTTP %d code=%d", resp.StatusCode, envelope.Code)}
		}
		failure := classifyResponse(resp.StatusCode, envelope.Message)
		if typed, ok := failure.(*Failure); ok && typed.StatusCode == http.StatusTooManyRequests {
			typed.RetryAfter = RetryAfter(resp.Header.Get("Retry-After"), 0)
		}
		return UploadAccepted{}, failure
	}
	accepted := envelope.Data
	if strings.TrimSpace(accepted.UploadID) == "" || strings.TrimSpace(accepted.TaskID) == "" || accepted.FileCount != len(batch.Files) {
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: fmt.Sprintf("incomplete upload acknowledgement: upload_id=%q task_id=%q file_count=%d expected=%d", accepted.UploadID, accepted.TaskID, accepted.FileCount, len(batch.Files))}
	}
	return accepted, nil
}

func (c *Client) authorize(req *http.Request) {
	if c.authorization != "" {
		req.Header.Set("Authorization", "Bearer "+c.authorization)
	}
}

type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (r countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n.Add(int64(n))
	return n, err
}

func multipartBody(batch spool.Batch, includeFields, gzipEnabled bool) (io.Reader, string, *atomic.Int64, <-chan error) {
	pr, pw := io.Pipe()
	var target io.Writer = pw
	var zipper *gzip.Writer
	if gzipEnabled {
		zipper = gzip.NewWriter(pw)
		target = zipper
	}
	mw := multipart.NewWriter(target)
	var sent atomic.Int64
	done := make(chan error, 1)
	go func() {
		var writeErr error
		defer func() {
			if closeErr := mw.Close(); writeErr == nil {
				writeErr = closeErr
			}
			if zipper != nil {
				if closeErr := zipper.Close(); writeErr == nil {
					writeErr = closeErr
				}
			}
			if writeErr != nil {
				_ = pw.CloseWithError(writeErr)
			} else {
				_ = pw.Close()
			}
			done <- writeErr
			close(done)
		}()
		if includeFields {
			if writeErr = mw.WriteField("project_name", batch.ProjectName); writeErr != nil {
				return
			}
			if writeErr = mw.WriteField("version", batch.Version); writeErr != nil {
				return
			}
		}
		for _, file := range batch.Files {
			part, err := mw.CreateFormFile("file", filepath.Base(file.Path))
			if err != nil {
				writeErr = err
				return
			}
			source, err := os.Open(file.Path)
			if err != nil {
				writeErr = err
				return
			}
			_, writeErr = io.Copy(part, source)
			closeErr := source.Close()
			if writeErr == nil {
				writeErr = closeErr
			}
			if writeErr != nil {
				return
			}
		}
	}()
	return countingReader{r: pr, n: &sent}, mw.FormDataContentType(), &sent, done
}

func transportFailure(err error, sentBytes int64) error {
	kind := Retryable
	message := "upload failed before request body was sent"
	if sentBytes > 0 {
		kind = Uncertain
		message = "upload result is unknown after request body transmission"
	}
	return &Failure{Kind: kind, Message: message, Err: err}
}

func classifyResponse(status int, message string) error {
	failure := &Failure{StatusCode: status, Message: strings.TrimSpace(message)}
	if failure.Message == "" {
		failure.Message = http.StatusText(status)
	}
	switch status {
	case http.StatusBadRequest:
		failure.Kind = Rejected
	case http.StatusUnauthorized, http.StatusForbidden:
		failure.Kind = Pause
	case http.StatusRequestEntityTooLarge:
		failure.Kind = Split
	case http.StatusTooManyRequests:
		failure.Kind = Retryable
	default:
		if status == http.StatusInternalServerError || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout {
			failure.Kind = Retryable
		} else {
			failure.Kind = Rejected
		}
	}
	return failure
}

func RetryAfter(header string, fallback time.Duration) time.Duration {
	header = strings.TrimSpace(header)
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return fallback
}

func decodeJSON(reader io.Reader, target any) error {
	limited := io.LimitReader(reader, maxResponseBytes+1)
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("response contains trailing data")
	}
	return nil
}
