package logs

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestUploadMetadataFromForm(t *testing.T) {
	values := url.Values{
		"project_id":        {"42"},
		"test_task_id":      {"task-local"},
		"test_task_name":    {"高温测试"},
		"uploader_name":     {"wangzhanying"},
		"remark":            {"night run"},
		"client_request_id": {"form-value"},
		"collector_version": {"0.0.3"},
		"timezone":          {"Asia/Shanghai"},
		"created_at":        {"2026-08-03T10:00:00+08:00"},
	}
	request := httptest.NewRequest("POST", "/api/logs/upload", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Idempotency-Key", "header-value")

	metadata, err := uploadMetadataFromForm(request, "ou_123", "DR7800", "V1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ProjectID != "42" || metadata.ProjectName != "DR7800" || metadata.Version != "V1.2.0" {
		t.Fatalf("unexpected project metadata: %+v", metadata)
	}
	if metadata.UploaderID != "ou_123" || metadata.UploaderName != "wangzhanying" {
		t.Fatalf("unexpected uploader metadata: %+v", metadata)
	}
	if metadata.ClientRequestID != "header-value" {
		t.Fatalf("header idempotency key should take precedence, got %q", metadata.ClientRequestID)
	}
	wantCreatedAt, _ := time.Parse(time.RFC3339, "2026-08-03T10:00:00+08:00")
	if metadata.CreatedAt == nil || !metadata.CreatedAt.Equal(wantCreatedAt) {
		t.Fatalf("unexpected created_at: %v", metadata.CreatedAt)
	}
}

func TestUploadMetadataRejectsInvalidTimestamp(t *testing.T) {
	values := url.Values{"created_at": {"2026/08/03"}}
	request := httptest.NewRequest("POST", "/api/logs/upload", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := uploadMetadataFromForm(request, "ou_123", "DR7800", "V1"); err == nil {
		t.Fatal("expected invalid timestamp error")
	}
}

func TestSafeStorageSegment(t *testing.T) {
	tests := map[string]string{
		"DR7800":          "DR7800",
		" wang/zhanying ": "wang_zhanying",
		"..":              "fallback",
		"CON":             "CON_",
	}
	for input, want := range tests {
		if got := safeStorageSegment(input, "fallback"); got != want {
			t.Errorf("safeStorageSegment(%q) = %q, want %q", input, got, want)
		}
	}
}
