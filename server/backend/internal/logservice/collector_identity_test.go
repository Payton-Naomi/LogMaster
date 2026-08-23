package logservice

import (
	"testing"

	"logmaster-agent/internal/config"
)

func TestCollectorUploadOwnerDetection(t *testing.T) {
	service := &Service{config: config.Config{}}
	if !service.isCollectorUploadOwner(builtinUploadOwnerOpenID) {
		t.Fatal("built-in collector owner must be recognized")
	}
	service.uploadToken = "configured-token"
	service.uploadOwnerOpenID = "configured-owner"
	if !service.isCollectorUploadOwner("configured-owner") {
		t.Fatal("configured collector owner must be recognized")
	}
	if service.isCollectorUploadOwner("regular-user") {
		t.Fatal("regular user must not receive collector auto-linking")
	}
}
