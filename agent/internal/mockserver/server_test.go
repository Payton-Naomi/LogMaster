package mockserver

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"logmaster-agent/agent/internal/model"
	"logmaster-agent/agent/internal/upload"
)

func TestServerPersistsAndDeduplicatesBatch(t *testing.T) {
	dir := t.TempDir()
	receiver := &Server{Dir: dir}
	server := httptest.NewServer(receiver.Handler())
	defer server.Close()
	batch := model.UploadBatch{BatchID: "0123456789abcdef0123456789abcdef", AgentID: "agent", ProjectID: "project", DeviceSN: "DUT1", SentAt: time.Now().UTC(), Logs: []model.LogEntry{{Sequence: 1, CapturedAt: time.Now().UTC(), Content: "line"}}}
	client := upload.NewHTTP(server.URL+"/api/v1/logs/upload", "", time.Second)
	if err := client.Upload(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("files=%d, want 1", len(entries))
	}
}
