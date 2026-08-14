package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRulesHandlerRejectsUserRuleCreation(t *testing.T) {
	service := &Service{}
	service.SetCurrentUserResolver(func(*http.Request) (string, bool) { return "ou_test", true })
	request := httptest.NewRequest(http.MethodPost, "/api/rules", strings.NewReader(`{"name":"custom","keyword":"FATAL"}`))
	recorder := httptest.NewRecorder()

	service.rulesHandler(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
