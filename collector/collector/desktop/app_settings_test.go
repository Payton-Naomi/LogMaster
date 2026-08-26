package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAppSettingsLogFontSize(t *testing.T) {
	settings := defaultAppSettings(t.TempDir())
	if settings.LogFontSize != 12 {
		t.Fatalf("default log font size = %d", settings.LogFontSize)
	}
	settings.LogFontSize = 30
	normalizeAppSettings(&settings, t.TempDir())
	if settings.LogFontSize != 12 {
		t.Fatalf("normalized log font size = %d", settings.LogFontSize)
	}
}

func TestDefaultAppSettingsMatchMaintainedProductDefaults(t *testing.T) {
	settings := defaultAppSettings(t.TempDir())
	if settings.NoLogTimeoutSeconds != 1800 || settings.MaxLogLines != 100000 {
		t.Fatalf("unexpected log defaults: %+v", settings)
	}
	if settings.MaxDiskBytes != 50*1024*1024*1024 || settings.UploadConcurrency != 5 {
		t.Fatalf("unexpected storage or upload defaults: %+v", settings)
	}
	if settings.BackendURL != "http://localhost:8080/api" || !settings.UploadGzip {
		t.Fatalf("unexpected backend defaults: %+v", settings)
	}
	if !settings.DefaultSaveEnabled || settings.DefaultUploadEnabled {
		t.Fatalf("unexpected default policies: %+v", settings)
	}
	if settings.SegmentMaxAgeSeconds != 1800 || settings.SegmentMaxBytes != 128*1024*1024 {
		t.Fatalf("unexpected segment defaults: %+v", settings)
	}
	if settings.ProgramName != "LogMaster采集端" || settings.ProgramVersion != "0.0.10" || settings.CompanyName != "上海七十迈数字科技有限公司" {
		t.Fatalf("unexpected product metadata: %+v", settings)
	}
}

func TestDesktopStartupMaterializesCompleteAppSettings(t *testing.T) {
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	service.shutdown()
	data, err := os.ReadFile(filepath.Join(root, "config", "settings-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings AppSettingsDTO
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.DefaultLogDirectory == "" || settings.SegmentMaxAgeSeconds == 0 || settings.SegmentMaxBytes == 0 || settings.MaxLogLines == 0 || settings.MaxDiskBytes == 0 || settings.BackendURL == "" || settings.UploadConcurrency == 0 {
		t.Fatalf("materialized settings are incomplete: %+v", settings)
	}
}

func TestDesktopMigratesLegacySettingsWhenPortableConfigIsUntouched(t *testing.T) {
	root := t.TempDir()
	configDirectory := filepath.Join(root, "config")
	if err := os.MkdirAll(configDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	defaults := defaultAppSettings(root)
	defaults.SchemaVersion = 0 // Simulate the settings file shipped with this upgrade.
	if err := writeAppSettings(filepath.Join(configDirectory, "settings-config.json"), defaults); err != nil {
		t.Fatal(err)
	}
	legacy := defaultAppSettings(root)
	legacy.BackendURL = "http://10.0.0.8:8080/api"
	legacy.UploadConcurrency = 7
	if err := writeAppSettings(filepath.Join(root, "app-settings.json"), legacy); err != nil {
		t.Fatal(err)
	}
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	loaded := service.GetAppSettings()
	if loaded.BackendURL != legacy.BackendURL || loaded.UploadConcurrency != legacy.UploadConcurrency || loaded.SchemaVersion != 1 {
		t.Fatalf("legacy settings were not migrated: %+v", loaded)
	}
}

func TestNormalizeAppSettingsMigratesLegacySegmentDefaults(t *testing.T) {
	settings := defaultAppSettings(t.TempDir())
	settings.SegmentMaxAgeSeconds = 300
	settings.SegmentMaxBytes = 32 * 1024 * 1024
	normalizeAppSettings(&settings, t.TempDir())
	if settings.SegmentMaxAgeSeconds != 1800 || settings.SegmentMaxBytes != 128*1024*1024 {
		t.Fatalf("legacy segment defaults were not migrated: %+v", settings)
	}
}

func TestNormalizeAppSettingsMigratesBrokenEarlyPreset(t *testing.T) {
	settings := defaultAppSettings(t.TempDir())
	settings.DefaultSaveEnabled = false
	settings.SegmentMaxAgeSeconds = 120
	settings.SegmentMaxBytes = 10 * 1024 * 1024
	settings.MaxDiskBytes = 10 * 1024 * 1024
	settings.NoLogTimeoutSeconds = 3000
	settings.UploadConcurrency = 2
	settings.UploadGzip = false
	normalizeAppSettings(&settings, t.TempDir())
	if !settings.DefaultSaveEnabled || settings.SegmentMaxAgeSeconds != 1800 || settings.SegmentMaxBytes != 128*1024*1024 || settings.MaxDiskBytes != 50*1024*1024*1024 || settings.UploadConcurrency != 5 || !settings.UploadGzip {
		t.Fatalf("broken early preset was not migrated: %+v", settings)
	}
}

func TestNormalizeAppSettingsFillsEmptyMetadata(t *testing.T) {
	settings := AppSettingsDTO{}
	normalizeAppSettings(&settings, t.TempDir())
	if settings.BackendURL == "" || settings.ProgramName == "" || settings.CompanyName == "" || settings.CommunityTitle == "" || settings.CommunityText == "" {
		t.Fatalf("empty metadata was not filled: %+v", settings)
	}
}

func TestSaveAppSettingsAppliesBackendAndLogDirectoryImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"status": "ok"}})
	}))
	defer server.Close()
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	settings := service.GetAppSettings()
	settings.BackendURL = server.URL + "/api"
	settings.DefaultLogDirectory = filepath.Join(root, "hot-logs")
	settings.UploadConcurrency = 3
	if err := service.SaveAppSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := service.backendClient.Health(service.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings.DefaultLogDirectory); err != nil {
		t.Fatal(err)
	}
	if service.worker.Concurrency() != 3 {
		t.Fatalf("worker concurrency = %d", service.worker.Concurrency())
	}
}
