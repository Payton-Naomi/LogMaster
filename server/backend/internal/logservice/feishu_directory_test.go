package logservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFeishuDirectoryIdentityIncludesJobTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			writeTestJSON(w, map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/contact/v3/users/batch_get_id":
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatal("batch lookup did not use tenant access token")
			}
			var input struct {
				Emails []string `json:"emails"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if len(input.Emails) != 1 || input.Emails[0] != "tester@70mai.com" {
				t.Fatalf("unexpected emails: %#v", input.Emails)
			}
			writeTestJSON(w, map[string]any{"code": 0, "data": map[string]any{"user_list": []any{map[string]any{
				"user_id": "ou_tester", "email": "tester@70mai.com",
				"status": map[string]any{"is_activated": true},
			}}}})
		case "/open-apis/contact/v3/users/ou_tester":
			writeTestJSON(w, map[string]any{"code": 0, "data": map[string]any{"user": map[string]any{
				"open_id": "ou_tester", "name": "测试用户", "email": "tester@70mai.com", "job_title": "助理软件测试工程师",
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	directory := newFeishuDirectory("app-id", "app-secret")
	directory.baseURL = server.URL
	directory.httpClient = &http.Client{Timeout: time.Second}
	identity, err := directory.identityByEmail(context.Background(), "tester@70mai.com")
	if err != nil {
		t.Fatal(err)
	}
	if identity.OpenID != "ou_tester" || identity.Name != "测试用户" || identity.Email != "tester@70mai.com" || identity.JobTitle != "助理软件测试工程师" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}
