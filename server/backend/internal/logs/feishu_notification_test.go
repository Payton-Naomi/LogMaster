package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFeishuNotifierSendsAnalysisAndReusesToken(t *testing.T) {
	var tokenRequests atomic.Int32
	var messageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			tokenRequests.Add(1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["app_id"] != "app-id" || body["app_secret"] != "app-secret" {
				t.Fatalf("unexpected credentials: %#v", body)
			}
			writeTestJSON(w, map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/im/v1/messages":
			messageRequests.Add(1)
			if r.URL.Query().Get("receive_id_type") != "open_id" {
				t.Fatalf("unexpected receive_id_type: %s", r.URL.RawQuery)
			}
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("unexpected authorization header")
			}
			var body struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			var content map[string]string
			if err := json.Unmarshal([]byte(body.Content), &content); err != nil {
				t.Fatal(err)
			}
			if body.ReceiveID != "ou_user" || body.MsgType != "text" || !strings.Contains(content["text"], "错误：3") {
				t.Fatalf("unexpected message: %#v, content=%#v", body, content)
			}
			writeTestJSON(w, map[string]any{"code": 0, "msg": "success"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	notifier := newFeishuNotifier("app-id", "app-secret", server.URL, &http.Client{Timeout: time.Second})
	notification := AnalysisNotification{
		RecipientOpenID: "ou_user", ProjectName: "demo", TotalLines: 100,
		ErrorCount: 3, WarningCount: 7, TopMatches: []AnalysisNotificationMatch{{Keyword: "timeout", Count: 2}},
	}
	if err := notifier.Notify(context.Background(), notification); err != nil {
		t.Fatal(err)
	}
	if err := notifier.Notify(context.Background(), notification); err != nil {
		t.Fatal(err)
	}
	if tokenRequests.Load() != 1 || messageRequests.Load() != 2 {
		t.Fatalf("token requests=%d, message requests=%d", tokenRequests.Load(), messageRequests.Load())
	}
}

func writeTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
