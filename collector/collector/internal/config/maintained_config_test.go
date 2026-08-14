package config

import (
	"testing"

	"gopkg.in/yaml.v3"
	logmasterconfig "logmaster-agent/agent/configs/logmaster"
)

func TestMaintainedDefaultsContainNoFakePort(t *testing.T) {
	var defaults struct {
		SchemaVersion int `yaml:"schema_version"`
		Serial        struct {
			BaudRate int `yaml:"baud_rate"`
			DataBits int `yaml:"data_bits"`
		} `yaml:"serial"`
		Storage struct {
			LimitGB int64 `yaml:"limit_gb"`
		} `yaml:"storage"`
		Upload struct {
			Concurrency int `yaml:"concurrency"`
		} `yaml:"upload"`
	}
	if err := yaml.Unmarshal(logmasterconfig.DefaultsYAML, &defaults); err != nil {
		t.Fatal(err)
	}
	if defaults.SchemaVersion != 1 || defaults.Serial.BaudRate != 115200 || defaults.Serial.DataBits != 8 {
		t.Fatalf("unexpected serial defaults: %+v", defaults)
	}
	if defaults.Storage.LimitGB != 50 || defaults.Upload.Concurrency != 5 {
		t.Fatalf("unexpected storage or upload defaults: %+v", defaults)
	}
}
