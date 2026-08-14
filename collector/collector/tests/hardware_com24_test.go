//go:build windows && hardware

package tests

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"logmaster-agent/agent/internal/app"
	"logmaster-agent/agent/internal/config"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

func TestHardwareCOM24CollectsSavesAndUploads(t *testing.T) {
	portName := os.Getenv("LOGMASTER_HARDWARE_PORT")
	if portName == "" {
		portName = "COM24"
	}
	detected, err := (serialagent.SystemDiscovery{}).List(context.Background())
	if err != nil {
		t.Fatalf("scan serial ports: %v", err)
	}
	detectedNames := make([]string, 0, len(detected))
	present := false
	for _, port := range detected {
		detectedNames = append(detectedNames, port.Name)
		present = present || strings.EqualFold(port.Name, portName)
	}
	if !present {
		t.Fatalf("%s is not currently present; detected ports=%v", portName, detectedNames)
	}
	var uploadRequests atomic.Int64
	var uploadedBytes atomic.Int64
	mockPlatform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]string{"status": "ok"}})
		case "/api/logs/upload":
			body := r.Body
			if r.Header.Get("Content-Encoding") == "gzip" {
				zipper, err := gzip.NewReader(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				defer zipper.Close()
				body = io.NopCloser(zipper)
			}
			r.Body = body
			if err := r.ParseMultipartForm(64 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.FormValue("project_name") != "hardware-com24" || r.FormValue("version") != "V1" {
				http.Error(w, "unexpected upload fields", http.StatusBadRequest)
				return
			}
			files := r.MultipartForm.File["file"]
			for _, header := range files {
				file, err := header.Open()
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				n, err := io.Copy(io.Discard, file)
				_ = file.Close()
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				uploadedBytes.Add(n)
			}
			uploadRequests.Add(1)
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_id": "hardware-upload", "task_id": "hardware-task", "status": "queued", "file_count": len(files)}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockPlatform.Close()

	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.ID = "hardware-com24"
	cfg.Agent.Listen = "127.0.0.1:0"
	cfg.Backend.BaseURL = mockPlatform.URL + "/api"
	cfg.Backend.ProjectName = "hardware-com24"
	cfg.Backend.Version = "V1"
	cfg.Backend.UploadInterval = 500 * time.Millisecond
	cfg.Backend.RequestTimeout = 10 * time.Second
	cfg.Backend.UploadConcurrency = 1
	cfg.Backend.UploadGzip = true
	cfg.Spool.Directory = filepath.Join(dir, "spool")
	cfg.Spool.SQLitePath = filepath.Join(dir, "agent.db")
	port := config.DefaultPortConfig()
	port.DeviceSN = "DUT-COM24"
	port.PortName = portName
	port.SegmentMaxAge = 3 * time.Second
	port.SegmentMaxBytes = 4096
	cfg.Serial.Ports = []config.PortConfig{port}

	store, err := spool.Open(cfg.Spool.SQLitePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var runtimeLogs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&runtimeLogs, nil))
	application, err := app.New(cfg, store, logger, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := store.Counts(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if uploadRequests.Load() > 0 && uploadedBytes.Load() > 0 && counts[spool.Uploaded] > 0 {
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	counts, _ := store.Counts(context.Background())
	t.Fatalf("%s did not complete real collect/save/upload within timeout: uploads=%d bytes=%d counts=%v runtime_logs=%s", portName, uploadRequests.Load(), uploadedBytes.Load(), counts, runtimeLogs.String())
}
