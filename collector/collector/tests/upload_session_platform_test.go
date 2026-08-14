package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/spool"
)

func TestUploadSessionPlatformE2E(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LOGMASTER_E2E_BACKEND_URL"), "/")
	token := os.Getenv("LOGMASTER_E2E_UPLOAD_TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("set LOGMASTER_E2E_BACKEND_URL and LOGMASTER_E2E_UPLOAD_TOKEN")
	}
	client := backend.New(backend.Config{BaseURL: baseURL, UploadPath: "/logs/upload", Authorization: token, Timeout: 30 * time.Second})
	request := backend.UploadSessionRequest{ClientRequestID: "e2e-session-" + time.Now().UTC().Format("20060102150405.000000000"), DeviceID: "COM24", Name: "串口 COM24", PortName: "COM24", BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: "none", Handshake: "none", ProjectName: "DR5800", Version: "E2E-V1", TestTaskID: "", TestTaskName: "连续上传测试", UploaderName: "全链路测试", Remark: "two batches", ScenarioIDs: []string{}, KeywordProfileID: "", KeywordRuleIDs: []string{}, KeywordMatching: false, SaveEnabled: true, UploadEnabled: true, NoLogTimeoutSeconds: 300, VID: "1A86", PID: "7523", USBSerial: "", Location: "", CollectorVersion: "0.0.7", Timezone: "Asia/Shanghai"}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session, err := client.CreateUploadSession(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := client.CreateUploadSession(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.UploadSessionID != session.UploadSessionID || replayed.QueryCode != session.QueryCode {
		t.Fatalf("session idempotency mismatch: first=%+v replay=%+v", session, replayed)
	}
	snapshot, _ := json.Marshal(request)
	for index := 1; index <= 2; index++ {
		content := []byte(fmt.Sprintf("2026-08-09 15:00:0%d INFO COM24 batch=%d\n2026-08-09 15:00:0%d WARN continuous upload\n", index, index, index))
		path := filepath.Join(t.TempDir(), fmt.Sprintf("com24-%d.log", index))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		batch := spool.Batch{ID: fmt.Sprintf("e2e-batch-%d-%d", time.Now().UnixNano(), index), ProjectName: request.ProjectName, Version: request.Version, TestTaskID: request.TestTaskID, TestTaskName: request.TestTaskName, UploaderName: request.UploaderName, Remark: request.Remark, CollectorVersion: request.CollectorVersion, Timezone: request.Timezone, ScenarioIDs: []string{}, UploadSessionID: session.UploadSessionID, QueryCode: session.QueryCode, ConfigSnapshot: string(snapshot), Files: []spool.File{{Path: path, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)), DeviceSN: "COM24", FirstSequence: int64(index * 10), LastSequence: int64(index*10 + 1)}}}
		accepted, err := client.Upload(ctx, batch)
		if err != nil {
			t.Fatal(err)
		}
		if accepted.QueryCode != session.QueryCode {
			t.Fatalf("batch %d returned query code %q, want %q", index, accepted.QueryCode, session.QueryCode)
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		result, err := querySession(ctx, baseURL, session.QueryCode)
		if err == nil && result.BatchCount == 2 && result.Status == "completed" && result.TotalFiles == 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("query did not aggregate two completed batches: result=%+v err=%v", result, err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

type querySessionResult struct {
	Status     string `json:"status"`
	BatchCount int    `json:"batch_count"`
	TotalFiles int    `json:"total_files"`
}

func querySession(ctx context.Context, baseURL, queryCode string) (querySessionResult, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/query/"+queryCode, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return querySessionResult{}, err
	}
	defer resp.Body.Close()
	var envelope struct {
		Code int                `json:"code"`
		Data querySessionResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return querySessionResult{}, err
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		return querySessionResult{}, fmt.Errorf("query HTTP %d code=%d", resp.StatusCode, envelope.Code)
	}
	return envelope.Data, nil
}
