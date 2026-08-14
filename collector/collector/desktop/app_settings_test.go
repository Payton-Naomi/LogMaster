package main

import "testing"

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

func TestNormalizeAppSettingsMigratesLegacySegmentDefaults(t *testing.T) {
	settings := defaultAppSettings(t.TempDir())
	settings.SegmentMaxAgeSeconds = 300
	settings.SegmentMaxBytes = 32 * 1024 * 1024
	normalizeAppSettings(&settings, t.TempDir())
	if settings.SegmentMaxAgeSeconds != 1800 || settings.SegmentMaxBytes != 128*1024*1024 {
		t.Fatalf("legacy segment defaults were not migrated: %+v", settings)
	}
}
