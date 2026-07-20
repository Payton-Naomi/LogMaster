package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPAgentAnalyzer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("authorization = %q", got)
		}
		var request AgentAnalysisRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.TaskID != "task-1" || len(request.Matches) != 1 {
			t.Errorf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AgentAnalysisResponse{
			Summary:  "one storage failure",
			Findings: []AgentFinding{{Category: "storage", Severity: "error", RootCause: "disk full", Suggestion: "free disk space", Confidence: 0.9}},
		})
	}))
	defer server.Close()

	analyzer := NewHTTPAgentAnalyzer(server.URL, "test-token", time.Second)
	result, err := analyzer.Analyze(context.Background(), AgentAnalysisRequest{
		TaskID: "task-1", UploadID: "upload-1", Matches: []ParseResult{{Level: "error", Content: "ERROR disk full"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary != "one storage failure" || len(result.Findings) != 1 {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestHTTPAgentAnalyzerFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	analyzer := NewHTTPAgentAnalyzer(server.URL, "", time.Second)
	if _, err := analyzer.Analyze(context.Background(), AgentAnalysisRequest{}); err == nil {
		t.Fatal("expected agent error")
	}
}
