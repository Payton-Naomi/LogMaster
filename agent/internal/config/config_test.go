//go:build legacy_agent_config

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaultsAndTokenOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`agent_id: test-agent
project_id: test-project
upload:
  url: http://localhost/upload
storage:
  sqlite_path: data/test.db
  log_dir: logs
devices:
  - device_sn: SIM001
    source: mock
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOGMASTER_TOKEN", "secret")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Upload.Interval != 5*time.Minute || cfg.Upload.BatchSize != 500 {
		t.Fatalf("unexpected upload defaults: %+v", cfg.Upload)
	}
	if cfg.Upload.Protocol != "batch_json" {
		t.Fatalf("unexpected default protocol: %q", cfg.Upload.Protocol)
	}
	if cfg.Upload.Token != "secret" || cfg.Devices[0].MockInterval != 500*time.Millisecond {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsDuplicateDevices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`agent_id: a
project_id: p
upload: {url: http://localhost/upload}
storage: {sqlite_path: data/test.db, log_dir: logs}
devices:
  - {device_sn: same, source: mock}
  - {device_sn: same, source: mock}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate device error")
	}
}
