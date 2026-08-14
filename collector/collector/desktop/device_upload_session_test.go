package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/spool"
)

func TestUploadSessionFailureRetryAndRestart(t *testing.T) {
	var available atomic.Bool
	var firstRequestID atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request backend.UploadSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if stored := firstRequestID.Load(); stored == nil {
			firstRequestID.Store(request.ClientRequestID)
		} else if stored.(string) != request.ClientRequestID {
			t.Errorf("retry changed client_request_id: first=%s retry=%s", stored, request.ClientRequestID)
		}
		w.Header().Set("Content-Type", "application/json")
		if !available.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":1,"message":"offline","data":{}}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"upload_session_id": "0977a959-0ab5-4612-8b40-c278129833a1", "query_code": "ABC1234567", "client_request_id": request.ClientRequestID, "status": "active"}})
	}))
	defer server.Close()
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	service.backendClient = backend.New(backend.Config{BaseURL: server.URL, Timeout: 0})
	project := service.catalog.Projects[0]
	version := "E2E-V1"
	if len(project.Versions) > 0 {
		version = project.Versions[0]
	}
	dto := normalizeDeviceConfig(DeviceConfigDTO{DeviceID: "COM24", Name: "串口 COM24", PortName: "COM24", SaveEnabled: true, UploadEnabled: true, ProjectID: project.ID, ProjectName: project.Name, Version: version, UploaderName: "测试上传人"})
	failed, err := service.SaveDeviceConfigWithResult(dto.DeviceID, dto)
	if err != nil {
		t.Fatal(err)
	}
	if !failed.Saved || failed.UploadReady || failed.Message == "" || service.configs[dto.DeviceID].UploadSetupState != "pending" {
		t.Fatalf("unexpected failed save result=%+v config=%+v", failed, service.configs[dto.DeviceID])
	}
	content := []byte("pending while cloud is offline\n")
	path := filepath.Join(t.TempDir(), "pending.log")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if _, err := service.store.EnqueueFileWithMetadata(context.Background(), spool.UploadMetadata{ProjectName: project.Name, Version: version, UploaderName: dto.UploaderName}, spool.File{Path: path, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)), DeviceSN: dto.DeviceID, FirstSequence: 1, LastSequence: 1}); err != nil {
		t.Fatal(err)
	}
	if batch, err := service.store.ClaimReady(context.Background(), 1); err != nil || batch != nil {
		t.Fatalf("paused upload was claimable: batch=%+v err=%v", batch, err)
	}
	available.Store(true)
	succeeded, err := service.SaveDeviceConfigWithResult(dto.DeviceID, dto)
	if err != nil {
		t.Fatal(err)
	}
	if !succeeded.UploadReady || succeeded.QueryCode != "ABC1234567" {
		t.Fatalf("unexpected retry result: %+v", succeeded)
	}
	batch, err := service.store.ClaimReady(context.Background(), 1)
	if err != nil || batch == nil {
		t.Fatalf("bound upload not claimable: batch=%+v err=%v", batch, err)
	}
	if batch.UploadSessionID == "" || batch.QueryCode != "ABC1234567" {
		t.Fatalf("pending batch not bound to session: %+v", batch)
	}
	service.shutdown()
	restarted, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.shutdown()
	// 0.0.9 起通道不再保留历史配置：重启后上传会话/查询码等不应被恢复。
	if len(restarted.configs) != 0 {
		t.Fatalf("upload session should not persist across restart: %+v", restarted.configs)
	}
}
