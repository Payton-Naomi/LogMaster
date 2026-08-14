//go:build windows && hardware

package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"logmaster-agent/agent/internal/backend"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

func TestHardwareCOM24SessionUpload(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("LOGMASTER_HARDWARE_BACKEND_URL"), "/")
	token := os.Getenv("LOGMASTER_UPLOAD_TOKEN")
	if baseURL == "" {
		t.Skip("LOGMASTER_HARDWARE_BACKEND_URL is required")
	}
	if token == "" {
		t.Fatal("LOGMASTER_UPLOAD_TOKEN is required")
	}
	portName := hardwareValue("LOGMASTER_HARDWARE_PORT", "COM24")
	projectName := hardwareValue("LOGMASTER_HARDWARE_PROJECT", "DR5800")
	version := hardwareValue("LOGMASTER_HARDWARE_VERSION", "COM24-E2E")
	uploader := hardwareValue("LOGMASTER_HARDWARE_UPLOADER", "COM24实机测试")
	detected, err := (serialagent.SystemDiscovery{}).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var descriptor serialagent.PortDescriptor
	for _, item := range detected {
		if strings.EqualFold(item.Name, portName) {
			descriptor = item
			break
		}
	}
	if descriptor.Name == "" {
		t.Fatalf("%s is not currently present", portName)
	}
	serialConfig := serialagent.SerialConfig{PortName: portName, BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: serialagent.ParityNone, Handshake: serialagent.HandshakeNone, ReadTimeout: 500 * time.Millisecond, WriteTimeout: time.Second, IdleGap: 10 * time.Millisecond, MaxFrameBytes: 64 * 1024, Encoding: serialagent.EncodingUTF8}
	port, err := (serialagent.GoBugFactory{}).Open(context.Background(), serialConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer port.Close()
	client := backend.New(backend.Config{BaseURL: baseURL, UploadPath: "/logs/upload", Authorization: token, Gzip: true, Timeout: 30 * time.Second})
	request := backend.UploadSessionRequest{ClientRequestID: fmt.Sprintf("com24-%d", time.Now().UnixNano()), DeviceID: portName, Name: "串口 " + portName, PortName: portName, BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: "none", Handshake: "none", ProjectName: projectName, Version: version, TestTaskName: "COM24连续上传", UploaderName: uploader, ScenarioIDs: []string{}, KeywordRuleIDs: []string{}, SaveEnabled: true, UploadEnabled: true, NoLogTimeoutSeconds: 300, VID: descriptor.VID, PID: descriptor.PID, USBSerial: descriptor.USBSerial, Location: descriptor.Location, CollectorVersion: "0.0.7", Timezone: "Asia/Shanghai"}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session, err := client.CreateUploadSession(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := json.Marshal(request)
	dir, buffer := t.TempDir(), make([]byte, 64*1024)
	for index := 1; index <= 2; index++ {
		var content []byte
		deadline := time.Now().Add(30 * time.Second)
		for len(content) == 0 && time.Now().Before(deadline) {
			n, readErr := port.Read(buffer)
			if n > 0 {
				content = append(content, buffer[:n]...)
			}
			if readErr != nil {
				t.Logf("read retry: %v", readErr)
			}
		}
		if len(content) == 0 {
			t.Fatalf("COM24 produced no data for batch %d", index)
		}
		path := filepath.Join(dir, fmt.Sprintf("com24-%d.log", index))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		batch := spool.Batch{ID: fmt.Sprintf("com24-%d-%d", time.Now().UnixNano(), index), ProjectName: projectName, Version: version, TestTaskName: request.TestTaskName, UploaderName: uploader, CollectorVersion: request.CollectorVersion, Timezone: request.Timezone, UploadSessionID: session.UploadSessionID, QueryCode: session.QueryCode, ConfigSnapshot: string(snapshot), Files: []spool.File{{Path: path, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)), DeviceSN: portName, FirstSequence: int64(index), LastSequence: int64(index)}}}
		accepted, err := client.Upload(ctx, batch)
		if err != nil {
			t.Fatal(err)
		}
		if accepted.QueryCode != session.QueryCode {
			t.Fatalf("batch %d query code mismatch", index)
		}
		t.Logf("COM24 batch=%d bytes=%d saved=%s", index, len(content), path)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		result, err := querySession(ctx, baseURL, session.QueryCode)
		if err == nil && result.BatchCount == 2 && result.TotalFiles == 2 && result.Status == "completed" {
			t.Logf("COM24 query_code=%s batches=2 files=2", session.QueryCode)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("COM24 aggregation failed: result=%+v err=%v", result, err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func hardwareValue(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
