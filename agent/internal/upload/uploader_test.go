package upload

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
)

func sampleBatch() model.UploadBatch {
	return model.UploadBatch{BatchID: "0123456789abcdef0123456789abcdef", AgentID: "agent", ProjectID: "project", DeviceSN: "DUT1", SentAt: time.Now().UTC(), Logs: []model.LogEntry{{Sequence: 1, CapturedAt: time.Now().UTC(), Content: "hello"}}}
}

func TestHTTPUploaderSendsGzipAndValidatesAck(t *testing.T) {
	batch := sampleBatch()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" || r.Header.Get("Idempotency-Key") != batch.BatchID {
			t.Error("missing upload headers")
		}
		zipper, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var received model.UploadBatch
		if err := json.NewDecoder(zipper).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = zipper.Close()
		_ = json.NewEncoder(w).Encode(model.UploadResponse{Accepted: true, BatchID: received.BatchID, Received: len(received.Logs)})
	}))
	defer server.Close()
	if err := NewHTTP(server.URL, "", time.Second).Upload(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPUploaderRejectsBadAcknowledgement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(model.UploadResponse{Accepted: true, BatchID: "wrong", Received: 1})
	}))
	defer server.Close()
	if err := NewHTTP(server.URL, "", time.Second).Upload(context.Background(), sampleBatch()); err == nil {
		t.Fatal("expected bad acknowledgement error")
	}
}

func TestHTTPUploaderReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "retry", http.StatusInternalServerError) }))
	defer server.Close()
	err := NewHTTP(server.URL, "", time.Second).Upload(context.Background(), sampleBatch())
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected error: %v", err)
	}
}
