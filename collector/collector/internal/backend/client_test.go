package backend

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"logmaster-agent/agent/internal/spool"
)

func uploadBatch(t *testing.T, count int) spool.Batch {
	t.Helper()
	dir := t.TempDir()
	createdAt := time.Date(2026, 8, 3, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	batch := spool.Batch{ID: "local", ClientRequestID: "request-001", ProjectID: "42", ProjectName: "DR2860", Version: "V1.0.0", TestTaskID: "task-template", TestTaskName: "高温测试", UploaderName: "张三", UploaderEmail: "zhangsan@company.com", Remark: "重启回归", CollectorVersion: "0.0.3", Timezone: "Asia/Shanghai", SourceCreatedAt: &createdAt, SourceStartedAt: &createdAt, ScenarioIDs: []string{"scene-a", "scene-b"}}
	for i := 0; i < count; i++ {
		content := []byte("ERROR test\n")
		path := filepath.Join(dir, string(rune('a'+i))+".log")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		batch.Files = append(batch.Files, spool.File{Path: path, SizeBytes: int64(len(content)), SHA256: hex.EncodeToString(sum[:])})
	}
	return batch
}

func TestUploadUsesExactMultipartContract(t *testing.T) {
	batch := uploadBatch(t, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != batch.ClientRequestID {
			t.Errorf("idempotency key = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		if r.FormValue("project_id") != batch.ProjectID || r.FormValue("project_name") != batch.ProjectName || r.FormValue("version") != batch.Version {
			t.Errorf("unexpected fields: %v", r.Form)
		}
		for name, want := range map[string]string{"test_task_id": batch.TestTaskID, "test_task_name": batch.TestTaskName, "uploader_name": batch.UploaderName, "uploader_email": batch.UploaderEmail, "remark": batch.Remark, "client_request_id": batch.ClientRequestID, "collector_version": batch.CollectorVersion, "timezone": batch.Timezone, "created_at": batch.SourceCreatedAt.Format(time.RFC3339Nano), "started_at": batch.SourceStartedAt.Format(time.RFC3339Nano), "scenario_ids": `["scene-a","scene-b"]`} {
			if got := r.FormValue(name); got != want {
				t.Errorf("%s = %q want %q", name, got, want)
			}
		}
		if len(r.MultipartForm.File["file"]) != 2 {
			t.Errorf("expected repeated file parts")
		}
		for _, header := range r.MultipartForm.File["file"] {
			file, _ := header.Open()
			_, _ = io.Copy(io.Discard, file)
			file.Close()
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "status": "queued", "file_count": 2, "client_request_id": batch.ClientRequestID}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL + "/api", UploadPath: "/logs/upload", Timeout: time.Second})
	accepted, err := client.Upload(context.Background(), batch)
	if err != nil || accepted.UploadID != "up" || accepted.TaskID != "task" {
		t.Fatalf("upload failed: %+v %v", accepted, err)
	}
}

func TestUploadRejectsMismatchedClientRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "file_count": 1, "client_request_id": "different"}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, UploadPath: "/upload", Timeout: time.Second})
	_, err := client.Upload(context.Background(), uploadBatch(t, 1))
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != Uncertain {
		t.Fatalf("expected uncertain mismatch failure, got %v", err)
	}
}

func TestUploadSendsConfiguredBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer field-token" {
			t.Errorf("authorization = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "file_count": 1}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, UploadPath: "/upload", Timeout: time.Second, Authorization: "field-token"})
	if _, err := client.Upload(context.Background(), uploadBatch(t, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestUploadSendsBuiltinBearerTokenByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+BuiltinUploadToken {
			t.Errorf("authorization = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "file_count": 1}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, UploadPath: "/upload", Timeout: time.Second})
	if _, err := client.Upload(context.Background(), uploadBatch(t, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestUploadRequiresHTTP202(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "file_count": 1}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, UploadPath: "/upload", Timeout: time.Second})
	_, err := client.Upload(context.Background(), uploadBatch(t, 1))
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != Uncertain {
		t.Fatalf("expected uncertain contract failure, got %v", err)
	}
}

func TestUploadCanGzipMultipartBody(t *testing.T) {
	batch := uploadBatch(t, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("content encoding = %q", r.Header.Get("Content-Encoding"))
		}
		zipper, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		r.Body = io.NopCloser(zipper)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "status": "queued", "file_count": 1}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, UploadPath: "/upload", Timeout: time.Second, Gzip: true})
	if _, err := client.Upload(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
}

func TestResponseClassification(t *testing.T) {
	for status, kind := range map[int]FailureKind{400: Rejected, 401: Pause, 403: Pause, 413: Split, 429: Retryable, 500: Retryable, 503: Retryable} {
		var failure *Failure
		if !errors.As(classifyResponse(status, "failed"), &failure) || failure.Kind != kind {
			t.Errorf("HTTP %d: got %+v want %s", status, failure, kind)
		}
	}
}

func TestPlatformProjectIDOnlyAcceptsCurrentNumericIDs(t *testing.T) {
	if got := platformProjectID(" 42 "); got != "42" {
		t.Fatalf("numeric project id = %q", got)
	}
	if got := platformProjectID("project-a"); got != "" {
		t.Fatalf("local catalog id must not be uploaded: %q", got)
	}
}

func TestSyncCollectorConfigUsesUploadTokenAndParsesSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/collector/sync" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+BuiltinUploadToken {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "success", "data": map[string]any{
			"projects":  []map[string]any{{"id": "42", "name": "DR2860"}},
			"scenarios": []map[string]any{{"id": "aging", "name": "普通挂测", "enabled": true}},
			"keywords":  []map[string]any{{"id": 7, "name": "断连", "category": "连接", "keyword": "disconnect", "scope": "line", "level": "warning"}},
			"synced_at": "2026-08-26T00:00:00Z",
		}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL + "/api", Timeout: time.Second})
	result, err := client.SyncCollectorConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Projects) != 1 || result.Projects[0].ID != "42" || len(result.Scenarios) != 1 || len(result.Keywords) != 1 {
		t.Fatalf("unexpected snapshot: %+v", result)
	}
}
