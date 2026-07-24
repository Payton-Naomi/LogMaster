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
	batch := spool.Batch{ID: "local", ProjectName: "DR2860", Version: "V1.0.0"}
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
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		if r.FormValue("project_name") != batch.ProjectName || r.FormValue("version") != batch.Version {
			t.Errorf("unexpected fields: %v", r.Form)
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
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "up", "task_id": "task", "status": "queued", "file_count": 2}})
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL + "/api", UploadPath: "/logs/upload", Timeout: time.Second})
	accepted, err := client.Upload(context.Background(), batch)
	if err != nil || accepted.UploadID != "up" || accepted.TaskID != "task" {
		t.Fatalf("upload failed: %+v %v", accepted, err)
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
