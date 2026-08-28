package backend

import (
	"bytes"
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
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

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

const BuiltinUploadToken = "logmaster-internal-collector-v1"

type Client struct {
	mu            sync.RWMutex
	baseURL       string
	healthPath    string
	inspectPath   string
	uploadPath    string
	authorization string
	gzip          bool
	http          *http.Client
}

// ApplyConfig updates connection settings for subsequent requests. Existing
// requests keep their own context and are not interrupted.
func (c *Client) ApplyConfig(cfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.HealthPath != "" {
		c.healthPath = cfg.HealthPath
	}
	if cfg.InspectPath != "" {
		c.inspectPath = cfg.InspectPath
	}
	if cfg.UploadPath != "" {
		c.uploadPath = cfg.UploadPath
	}
	if strings.TrimSpace(cfg.Authorization) != "" {
		c.authorization = strings.TrimSpace(cfg.Authorization)
	}
	c.gzip = cfg.Gzip
	timeout := c.http.Timeout
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	c.http = &http.Client{Timeout: timeout}
}

func (c *Client) snapshot() (string, string, string, string, bool, *http.Client) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL, c.healthPath, c.inspectPath, c.uploadPath, c.gzip, c.http
}

type APIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type UploadAccepted struct {
	UploadID        string `json:"upload_id"`
	TaskID          string `json:"task_id"`
	QueryCode       string `json:"query_code,omitempty"`
	Status          string `json:"status"`
	FileCount       int    `json:"file_count"`
	ClientRequestID string `json:"client_request_id,omitempty"`
}

type UploadSessionRequest struct {
	ClientRequestID     string   `json:"client_request_id"`
	DeviceID            string   `json:"device_id"`
	Name                string   `json:"name"`
	PortName            string   `json:"port_name"`
	BaudRate            int      `json:"baud_rate"`
	DataBits            int      `json:"data_bits"`
	StopBits            int      `json:"stop_bits"`
	Parity              string   `json:"parity"`
	Handshake           string   `json:"handshake"`
	DTR                 bool     `json:"dtr"`
	RTS                 bool     `json:"rts"`
	ProjectID           string   `json:"project_id"`
	ProjectName         string   `json:"project_name"`
	Version             string   `json:"version"`
	TestTaskID          string   `json:"test_task_id"`
	TestTaskName        string   `json:"test_task_name"`
	UploaderName        string   `json:"uploader_name"`
	UploaderEmail       string   `json:"uploader_email"`
	Remark              string   `json:"remark"`
	ScenarioIDs         []string `json:"scenario_ids"`
	KeywordProfileID    string   `json:"keyword_profile_id"`
	KeywordRuleIDs      []string `json:"keyword_rule_ids"`
	KeywordMatching     bool     `json:"keyword_matching_enabled"`
	SaveEnabled         bool     `json:"save_enabled"`
	UploadEnabled       bool     `json:"upload_enabled"`
	NoLogTimeoutSeconds int      `json:"no_log_timeout_seconds"`
	VID                 string   `json:"vid"`
	PID                 string   `json:"pid"`
	USBSerial           string   `json:"usb_serial"`
	Location            string   `json:"location"`
	CollectorVersion    string   `json:"collector_version"`
	Timezone            string   `json:"timezone"`
}

type UploadSessionAccepted struct {
	UploadSessionID string          `json:"upload_session_id"`
	QueryCode       string          `json:"query_code"`
	ClientRequestID string          `json:"client_request_id"`
	Status          string          `json:"status"`
	UploaderName    string          `json:"uploader_name"`
	UploaderEmail   string          `json:"uploader_email"`
	ConfigSnapshot  json.RawMessage `json:"config_snapshot"`
}

type StandardKeyword struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Keyword     string    `json:"keyword"`
	Scope       string    `json:"scope"`
	Level       string    `json:"level"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CollectorProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CollectorScenario struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	AllProjects bool     `json:"all_projects"`
	Projects    []string `json:"projects"`
	Keywords    []string `json:"keywords"`
}

type CollectorConfigSnapshot struct {
	Projects  []CollectorProject  `json:"projects"`
	Scenarios []CollectorScenario `json:"scenarios"`
	Keywords  []StandardKeyword   `json:"keywords"`
	SyncedAt  time.Time           `json:"synced_at"`
}

type PublicUploadBatch struct {
	UploadID        string    `json:"upload_id"`
	TaskID          string    `json:"task_id"`
	ClientRequestID string    `json:"client_request_id"`
	QueryCode       string    `json:"query_code"`
	Status          string    `json:"status"`
	OriginalName    string    `json:"original_name"`
	ErrorType       string    `json:"error_type"`
	ErrorMessage    string    `json:"error_message"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PublicUploadSession struct {
	UploadSessionID string              `json:"upload_session_id"`
	QueryCode       string              `json:"query_code"`
	Status          string              `json:"status"`
	Batches         []PublicUploadBatch `json:"batches"`
}

func New(cfg Config) *Client {
	authorization := strings.TrimSpace(cfg.Authorization)
	if authorization == "" {
		authorization = BuiltinUploadToken
	}
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"), healthPath: cfg.HealthPath,
		inspectPath: cfg.InspectPath, uploadPath: cfg.UploadPath,
		authorization: authorization,
		gzip:          cfg.Gzip,
		http:          &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) Health(ctx context.Context) error {
	baseURL, healthPath, _, _, _, client := c.snapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+healthPath, nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := client.Do(req)
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

func (c *Client) CreateUploadSession(ctx context.Context, input UploadSessionRequest) (UploadSessionAccepted, error) {
	baseURL, _, _, _, _, client := c.snapshot()
	body, err := json.Marshal(input)
	if err != nil {
		return UploadSessionAccepted{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/upload-sessions", bytes.NewReader(body))
	if err != nil {
		return UploadSessionAccepted{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return UploadSessionAccepted{}, err
	}
	defer resp.Body.Close()
	var envelope APIResponse[UploadSessionAccepted]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return UploadSessionAccepted{}, err
	}
	if resp.StatusCode != http.StatusCreated || envelope.Code != 0 {
		return UploadSessionAccepted{}, fmt.Errorf("create upload session rejected: HTTP %d code=%d: %s", resp.StatusCode, envelope.Code, envelope.Message)
	}
	accepted := envelope.Data
	if strings.TrimSpace(accepted.UploadSessionID) == "" || strings.TrimSpace(accepted.QueryCode) == "" || accepted.ClientRequestID != input.ClientRequestID {
		return UploadSessionAccepted{}, errors.New("create upload session acknowledgement is incomplete")
	}
	return accepted, nil
}

func (c *Client) CompleteUploadSession(ctx context.Context, uploadSessionID string) error {
	baseURL, _, _, _, _, client := c.snapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/upload-sessions/"+uploadSessionID+"/complete", nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope APIResponse[map[string]any]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		return fmt.Errorf("close upload session rejected: HTTP %d code=%d", resp.StatusCode, envelope.Code)
	}
	return nil
}

func (c *Client) SyncKeywords(ctx context.Context) ([]StandardKeyword, error) {
	baseURL, _, _, _, _, client := c.snapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/keywords/sync", nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var envelope APIResponse[[]StandardKeyword]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		return nil, fmt.Errorf("sync keywords rejected: HTTP %d code=%d: %s", resp.StatusCode, envelope.Code, envelope.Message)
	}
	if envelope.Data == nil {
		envelope.Data = []StandardKeyword{}
	}
	return envelope.Data, nil
}

func (c *Client) SyncCollectorConfig(ctx context.Context) (CollectorConfigSnapshot, error) {
	baseURL, _, _, _, _, client := c.snapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/collector/sync", nil)
	if err != nil {
		return CollectorConfigSnapshot{}, err
	}
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return CollectorConfigSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return CollectorConfigSnapshot{}, errors.New("当前后端不支持聚合配置同步")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return CollectorConfigSnapshot{}, fmt.Errorf("同步鉴权失败（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return CollectorConfigSnapshot{}, fmt.Errorf("聚合配置同步失败（HTTP %d）", resp.StatusCode)
	}
	// Some older Go services omit application/json even for JSON bodies. Reject
	// known HTML error pages, then let strict JSON decoding validate the body.
	if contentType := resp.Header.Get("Content-Type"); strings.Contains(strings.ToLower(contentType), "text/html") {
		return CollectorConfigSnapshot{}, fmt.Errorf("聚合配置同步返回了非 JSON 内容（%s）", contentType)
	}
	var envelope APIResponse[CollectorConfigSnapshot]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return CollectorConfigSnapshot{}, fmt.Errorf("聚合配置同步响应格式不正确: %w", err)
	}
	if envelope.Code != 0 {
		return CollectorConfigSnapshot{}, fmt.Errorf("聚合配置同步被后端拒绝（%s）", envelope.Message)
	}
	return envelope.Data, nil
}

func (c *Client) QueryUploadSession(ctx context.Context, queryCode string) (PublicUploadSession, error) {
	baseURL, _, _, _, _, client := c.snapshot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/query/"+strings.TrimSpace(queryCode), nil)
	if err != nil {
		return PublicUploadSession{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return PublicUploadSession{}, err
	}
	defer resp.Body.Close()
	var envelope APIResponse[PublicUploadSession]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		return PublicUploadSession{}, err
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		return PublicUploadSession{}, fmt.Errorf("query upload session rejected: HTTP %d code=%d: %s", resp.StatusCode, envelope.Code, envelope.Message)
	}
	return envelope.Data, nil
}

func (c *Client) Inspect(ctx context.Context, file spool.File) error {
	baseURL, _, inspectPath, _, gzipEnabled, client := c.snapshot()
	batch := spool.Batch{Files: []spool.File{file}}
	reader, contentType, sent, _, writeDone := multipartBody(batch, false, gzipEnabled)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+inspectPath, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if gzipEnabled {
		req.Header.Set("Content-Encoding", "gzip")
	}
	c.authorize(req)
	resp, err := client.Do(req)
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
	return c.UploadWithProgress(ctx, batch, nil)
}

type ProgressCallback func(sentBytes, totalBytes int64)

func (c *Client) UploadWithProgress(ctx context.Context, batch spool.Batch, progress ProgressCallback) (UploadAccepted, error) {
	baseURL, _, _, uploadPath, gzipEnabled, client := c.snapshot()
	if strings.TrimSpace(batch.ClientRequestID) == "" {
		batch.ClientRequestID = batch.ID
	}
	if err := validateUploadBatch(batch); err != nil {
		return UploadAccepted{}, &Failure{Kind: Rejected, Message: err.Error(), Err: err}
	}
	reader, contentType, sent, fileSent, writeDone := multipartBody(batch, true, gzipEnabled)
	var total int64
	for _, file := range batch.Files {
		total += file.SizeBytes
	}
	progressCtx, progressCancel := context.WithCancel(ctx)
	defer progressCancel()
	if progress != nil {
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-progressCtx.Done():
					return
				case <-ticker.C:
					progress(fileSent.Load(), total)
				}
			}
		}()
	}
	stopProgress := func() {
		progressCancel()
		if progress != nil {
			progress(fileSent.Load(), total)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+uploadPath, reader)
	if err != nil {
		return UploadAccepted{}, err
	}
	req.Header.Set("Content-Type", contentType)
	if batch.ClientRequestID != "" {
		req.Header.Set("Idempotency-Key", batch.ClientRequestID)
	}
	if gzipEnabled {
		req.Header.Set("Content-Encoding", "gzip")
	}
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		stopProgress()
		return UploadAccepted{}, transportFailure(err, sent.Load())
	}
	defer resp.Body.Close()
	if err := <-writeDone; err != nil {
		stopProgress()
		return UploadAccepted{}, &Failure{Kind: Uncertain, Message: "upload request body failed", Err: err}
	}
	var envelope APIResponse[UploadAccepted]
	if err := decodeJSON(resp.Body, &envelope); err != nil {
		stopProgress()
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: "upload acknowledgement invalid", Err: err}
	}
	if resp.StatusCode != http.StatusAccepted || envelope.Code != 0 {
		stopProgress()
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
		stopProgress()
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: fmt.Sprintf("incomplete upload acknowledgement: upload_id=%q task_id=%q file_count=%d expected=%d", accepted.UploadID, accepted.TaskID, accepted.FileCount, len(batch.Files))}
	}
	if accepted.ClientRequestID != "" && accepted.ClientRequestID != batch.ClientRequestID {
		stopProgress()
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: fmt.Sprintf("upload acknowledgement client_request_id mismatch: got=%q expected=%q", accepted.ClientRequestID, batch.ClientRequestID)}
	}
	if batch.QueryCode != "" && !strings.EqualFold(accepted.QueryCode, batch.QueryCode) {
		stopProgress()
		return UploadAccepted{}, &Failure{Kind: Uncertain, StatusCode: resp.StatusCode, Message: fmt.Sprintf("upload acknowledgement query_code mismatch: got=%q expected=%q", accepted.QueryCode, batch.QueryCode)}
	}
	stopProgress()
	return accepted, nil
}

func (c *Client) authorize(req *http.Request) {
	c.mu.RLock()
	authorization := c.authorization
	c.mu.RUnlock()
	if authorization != "" {
		req.Header.Set("Authorization", "Bearer "+authorization)
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

func multipartBody(batch spool.Batch, includeFields, gzipEnabled bool) (io.Reader, string, *atomic.Int64, *atomic.Int64, <-chan error) {
	pr, pw := io.Pipe()
	var target io.Writer = pw
	var zipper *gzip.Writer
	if gzipEnabled {
		zipper = gzip.NewWriter(pw)
		target = zipper
	}
	mw := multipart.NewWriter(target)
	var sent atomic.Int64
	var fileSent atomic.Int64
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
			fields := [][2]string{
				{"upload_session_id", batch.UploadSessionID}, {"query_code", batch.QueryCode},
				{"project_id", platformProjectID(batch.ProjectID)}, {"project_name", batch.ProjectName}, {"version", batch.Version},
				{"test_task_id", batch.TestTaskID}, {"test_task_name", batch.TestTaskName},
				{"uploader_name", batch.UploaderName}, {"uploader_email", batch.UploaderEmail}, {"remark", batch.Remark},
				{"client_request_id", batch.ClientRequestID}, {"collector_version", batch.CollectorVersion}, {"timezone", batch.Timezone},
				{"created_at", formatOptionalTime(batch.SourceCreatedAt)}, {"started_at", formatOptionalTime(batch.SourceStartedAt)}, {"ended_at", formatOptionalTime(batch.SourceEndedAt)},
				{"config_snapshot", firstNonEmpty(batch.ConfigSnapshot, "{}")},
			}
			encoded, err := json.Marshal(batch.ScenarioIDs)
			if err != nil {
				writeErr = err
				return
			}
			fields = append(fields, [2]string{"scenario_ids", string(encoded)})
			for _, field := range fields {
				if field[1] == "" {
					continue
				}
				if writeErr = mw.WriteField(field[0], field[1]); writeErr != nil {
					return
				}
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
			_, writeErr = io.Copy(part, countingReader{r: source, n: &fileSent})
			closeErr := source.Close()
			if writeErr == nil {
				writeErr = closeErr
			}
			if writeErr != nil {
				return
			}
		}
	}()
	return countingReader{r: pr, n: &sent}, mw.FormDataContentType(), &sent, &fileSent, done
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func platformProjectID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return ""
	}
	return value
}

func validateUploadBatch(batch spool.Batch) error {
	required := []struct{ name, value string }{{"project_name", batch.ProjectName}, {"version", batch.Version}, {"uploader_email", batch.UploaderEmail}}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if strings.TrimSpace(batch.UploadSessionID) != "" && strings.TrimSpace(batch.QueryCode) == "" {
		return errors.New("query_code is required with upload_session_id")
	}
	limits := []struct {
		name  string
		value string
		max   int
	}{
		{"project_name", batch.ProjectName, 128}, {"version", batch.Version, 64},
		{"test_task_id", batch.TestTaskID, 128}, {"test_task_name", batch.TestTaskName, 256},
		{"uploader_name", batch.UploaderName, 128}, {"remark", batch.Remark, 4000},
		{"client_request_id", batch.ClientRequestID, 128}, {"collector_version", batch.CollectorVersion, 64},
	}
	for _, field := range limits {
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s exceeds %d characters", field.name, field.max)
		}
	}
	if len(batch.ScenarioIDs) > 20 {
		return errors.New("scenario_ids exceeds 20 items")
	}
	return nil
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
