package tests

import (
	"path/filepath"
	"testing"

	config "logmaster-agent/agent/internal/config"
)

func TestDeliveryConfigTemplateDefinesFourChannelsAndEightSlots(t *testing.T) {
	path := filepath.Join("..", "config_template.yaml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config template: %v", err)
	}
	if len(cfg.Serial.Ports) != 8 {
		t.Fatalf("port slots = %d, want 8", len(cfg.Serial.Ports))
	}
	for index, expected := range []string{"DUT-01", "DUT-02", "DUT-03", "DUT-04"} {
		if cfg.Serial.Ports[index].DeviceSN != expected {
			t.Fatalf("port %d device = %q, want %q", index+1, cfg.Serial.Ports[index].DeviceSN, expected)
		}
	}
	if cfg.Serial.Ports[7].DeviceSN != "DUT-08-RESERVED" {
		t.Fatalf("eighth slot = %q, want DUT-08-RESERVED", cfg.Serial.Ports[7].DeviceSN)
	}
}
