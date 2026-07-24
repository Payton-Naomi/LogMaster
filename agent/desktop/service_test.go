package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"logmaster-agent/agent/internal/spool"
)

func TestDesktopSettingsPersistAcrossRestart(t *testing.T) {
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	dto := service.configs["DUT-01"]
	dto.Name, dto.PortName = "主设备", "COM88"
	service.mu.Lock()
	service.configs[dto.DeviceID] = dto
	service.mu.Unlock()
	if err := service.saveSettings(); err != nil {
		t.Fatal(err)
	}
	service.shutdown()

	restarted, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.shutdown()
	states := restarted.GetDeviceStates()
	if len(states) != 4 || states[0].Name != "主设备" || states[0].PortName != "COM88" {
		t.Fatalf("settings were not restored: %+v", states)
	}
}

func TestDesktopCanResolveUncertainUpload(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	path := filepath.Join(t.TempDir(), "DUT-01.log")
	content := []byte("line\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	id, err := service.store.EnqueueFile(context.Background(), "p", "v", spool.File{Path: path, SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)), DeviceSN: "DUT-01", FirstSequence: 1, LastSequence: 1})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := service.store.ClaimReady(context.Background(), 1)
	if err != nil || batch == nil || batch.ID != id {
		t.Fatalf("claim=%+v err=%v", batch, err)
	}
	if err := service.store.MarkUncertain(context.Background(), id, "response lost"); err != nil {
		t.Fatal(err)
	}
	listed, err := service.GetUploadQueueBatches()
	if err != nil || len(listed) != 1 || listed[0].ID != id {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	if err := service.ConfirmUncertain(id, "upload-1", "task-1"); err != nil {
		t.Fatal(err)
	}
	counts, err := service.store.Counts(context.Background())
	if err != nil || counts[spool.Uploaded] != 1 || counts[spool.Uncertain] != 0 {
		t.Fatalf("counts=%v err=%v", counts, err)
	}
}
