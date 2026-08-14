//go:build windows && hardware_legacy

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"logmaster-agent/agent/internal/app"
	"logmaster-agent/agent/internal/config"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

func TestHardwareCOM24CollectsSavesAndUploadsToPlatform(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LOGMASTER_HARDWARE_BACKEND_URL"), "/")
	if baseURL == "" {
		t.Skip("LOGMASTER_HARDWARE_BACKEND_URL is required for live platform validation")
	}
	if os.Getenv("LOGMASTER_UPLOAD_TOKEN") == "" {
		t.Fatal("LOGMASTER_UPLOAD_TOKEN is required for live platform validation")
	}
	portName := firstHardwareValue("LOGMASTER_HARDWARE_PORT", "COM24")
	projectName := firstHardwareValue("LOGMASTER_HARDWARE_PROJECT", "DR2860")
	version := firstHardwareValue("LOGMASTER_HARDWARE_VERSION", "COM24-E2E")

	detected, err := (serialagent.SystemDiscovery{}).List(context.Background())
	if err != nil {
		t.Fatalf("scan serial ports: %v", err)
	}
	present := false
	for _, port := range detected {
		present = present || strings.EqualFold(port.Name, portName)
	}
	if !present {
		t.Fatalf("%s is not currently present", portName)
	}

	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Agent.ID = "hardware-com24-platform"
	cfg.Agent.Listen = "127.0.0.1:0"
	cfg.Backend.BaseURL = baseURL
	cfg.Backend.AuthorizationTokenEnv = "LOGMASTER_UPLOAD_TOKEN"
	cfg.Backend.ProjectName = projectName
	cfg.Backend.Version = version
	cfg.Backend.UploadInterval = 500 * time.Millisecond
	cfg.Backend.RequestTimeout = 30 * time.Second
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
	application, err := app.New(cfg, store, slog.New(slog.NewTextHandler(&runtimeLogs, nil)), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("stop application: %v", err)
		}
	}()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		batches, err := store.ListByState(context.Background(), spool.Uploaded)
		if err != nil {
			t.Fatal(err)
		}
		for _, batch := range batches {
			if batch.QueryCode == "" || len(batch.Files) == 0 {
				continue
			}
			file := batch.Files[0]
			content, err := os.ReadFile(file.Path)
			if err != nil || len(content) == 0 {
				t.Fatalf("saved COM24 log is missing or empty: path=%s err=%v", file.Path, err)
			}
			result, err := queryHardwareUpload(baseURL, batch.QueryCode)
			if err != nil {
				t.Fatal(err)
			}
			if result.UploadID != batch.UploadID || result.TaskID != batch.TaskID || result.QueryCode != batch.QueryCode {
				t.Fatalf("platform acknowledgement mismatch: batch=%+v query=%+v", batch, result)
			}
			t.Logf("validated COM24 bytes=%d upload_id=%s task_id=%s query_code=%s status=%s", len(content), batch.UploadID, batch.TaskID, batch.QueryCode, result.Status)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("COM24 did not complete live platform upload within timeout: %s", runtimeLogs.String())
}

type hardwareQueryResult struct {
	UploadID  string `json:"upload_id"`
	TaskID    string `json:"task_id"`
	QueryCode string `json:"query_code"`
	Status    string `json:"status"`
}

func queryHardwareUpload(baseURL, queryCode string) (hardwareQueryResult, error) {
	endpoint := strings.TrimSuffix(baseURL, "/api") + "/api/query/" + queryCode
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return hardwareQueryResult{}, err
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return hardwareQueryResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return hardwareQueryResult{}, fmt.Errorf("query code rejected: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		Code int                 `json:"code"`
		Data hardwareQueryResult `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return hardwareQueryResult{}, err
	}
	if envelope.Code != 0 {
		return hardwareQueryResult{}, fmt.Errorf("query code response code=%d", envelope.Code)
	}
	return envelope.Data, nil
}

func firstHardwareValue(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
