package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"logmaster-agent/agent/internal/spool"
)

func TestDesktopDefaultsDoNotInventCOMPorts(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	if len(service.configs) != 0 {
		t.Fatalf("desktop created fixed channels: %+v", service.configs)
	}
}

func TestDesktopBuildsChannelFromDetectedPort(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	channel := service.configForPortLocked("COM24", 0)
	if channel.DeviceID != "COM24" || channel.PortName != "COM24" || channel.Name != "串口 COM24" {
		t.Fatalf("unexpected dynamic channel: %+v", channel)
	}
}

func TestNormalizeDeviceConfigMigratesLegacyChannelName(t *testing.T) {
	channel := normalizeDeviceConfig(DeviceConfigDTO{Name: "设备通道", PortName: "COM24"})
	if channel.Name != "串口 COM24" {
		t.Fatalf("legacy default channel name was not migrated: %+v", channel)
	}
}

func TestDesktopMigratesLegacyChannelToSerialOnly(t *testing.T) {
	root := t.TempDir()
	settings := desktopSettings{Devices: []DeviceConfigDTO{
		{DeviceID: "COM24", Name: "旧设备", PortName: "COM24", UploadEnabled: true, ProjectName: "旧项目", UploaderName: "旧上传人"},
	}}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "desktop-config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	if len(service.configs) != 1 {
		t.Fatalf("legacy serial settings were not retained: %+v", service.configs)
	}
	channel := service.configs["COM24"]
	if channel.UploadEnabled || channel.ProjectName != "" || channel.UploaderName != "" {
		t.Fatalf("legacy upload identity must be stripped without USB serial: %+v", channel)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("migrated desktop-config.json should remain available: %v", err)
	}
}

func TestDesktopSerialSettingsPersistWithoutCloudIdentity(t *testing.T) {
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	dto := normalizeDeviceConfig(DeviceConfigDTO{Name: "主设备", PortName: "COM88", BaudRate: 921600, UploaderName: "张三", UploaderEmail: "zhangsan@company.com", Remark: "高温回归", UploadEnabled: true})
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
	if len(restarted.configs) != 1 || restarted.configs["COM88"].BaudRate != 921600 {
		t.Fatalf("serial configuration should persist across restart: %+v", restarted.configs)
	}
	if restarted.configs["COM88"].UploadEnabled || restarted.configs["COM88"].UploaderEmail != "" || restarted.configs["COM88"].Remark != "" {
		t.Fatalf("cloud identity must not persist without USB serial: %+v", restarted.configs["COM88"])
	}
}

func TestFreshChannelDefaultsUploadCloudOff(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	channel := service.configForPortLocked("COM24", 0)
	if channel.UploadEnabled {
		t.Fatalf("fresh channel must default upload cloud off: %+v", channel)
	}
	if channel.ProjectID != "" || channel.Version != "" || channel.UploaderName != "" || channel.Remark != "" {
		t.Fatalf("fresh channel must not inherit upload fields: %+v", channel)
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
