package logservice

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotificationSettingsRejectsIncompleteDocument(t *testing.T) {
	service := &Service{}
	service.SetCurrentUserResolver(func(*http.Request) (string, bool) { return "ou_test", true })
	request := httptest.NewRequest(http.MethodPut, "/api/notification-settings",
		strings.NewReader(`{"task_completed":true}`))
	response := httptest.NewRecorder()

	service.notificationSettingsHandler(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}
