package uploader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content type")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	u := New(server.URL, "", 10, 100)
	batch := []LogEntry{
		{Device: "dev1", Content: "test log 1", Severity: "ERROR"},
		{Device: "dev1", Content: "test log 2", Severity: "INFO"},
	}
	err := u.Upload(batch)
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
}

func TestUploadServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := New(server.URL, "", 10, 100)
	err := u.Upload([]LogEntry{{Content: "test"}})
	if err == nil {
		t.Fatal("Upload() should return error on 500")
	}
}

func TestUploadNetworkError(t *testing.T) {
	u := New("http://127.0.0.1:19999", "", 10, 100)
	err := u.Upload([]LogEntry{{Content: "test"}})
	if err == nil {
		t.Fatal("Upload() should return error on connection refused")
	}
}

func TestMarshalBatch(t *testing.T) {
	batch := []LogEntry{
		{Device: "dev1", Content: "line1", Severity: "ERROR"},
	}
	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("empty marshal")
	}
}