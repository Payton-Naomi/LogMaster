package analyzer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validRequest() AnalysisRequest {
	return AnalysisRequest{
		TaskID:     "d3a67a1e-34c9-40f7-9224-42b19f53d143",
		UploadID:   "eb1527fc-58bb-42a1-bd56-dff40d374afa",
		TotalLines: 12000,
		File: AnalysisFile{
			ID: 1, RelativePath: "items/1/extracted/logfile_0", SizeBytes: 3145728,
			SHA256: strings.Repeat("ab", 32), LineCount: 12000,
		},
		Matches: []Match{{
			Level: "error", MatchedText: "ERROR", LineNumber: 42,
			Content: "ERROR recorder failed", FilePath: "ignored/backend/path",
		}},
	}
}

func newTestAnalyzer(t *testing.T, mutate func(*Config)) *Analyzer {
	t.Helper()
	config := DefaultConfig()
	if mutate != nil {
		mutate(&config)
	}
	analyzer, err := New(config, NewMemoryCache())
	if err != nil {
		t.Fatal(err)
	}
	return analyzer
}

func requestJSON(t *testing.T, analyzer *Analyzer, request AnalysisRequest, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))
	httpRequest.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	analyzer.Handler().ServeHTTP(response, httpRequest)
	return response
}

func TestHandlerImplementsContract(t *testing.T) {
	analyzer := newTestAnalyzer(t, func(config *Config) { config.Token = "secret" })
	response := requestJSON(t, analyzer, validRequest(), "secret")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var result AnalysisResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Category != "recording" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(response.Body.Bytes()) > MaxResponseBytes {
		t.Fatalf("response exceeded limit: %d", response.Body.Len())
	}
}

func TestHandlerAuthenticationAndMethod(t *testing.T) {
	analyzer := newTestAnalyzer(t, func(config *Config) { config.Token = "secret" })
	response := requestJSON(t, analyzer, validRequest(), "wrong")
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("unexpected unauthorized response: %d %s", response.Code, response.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/analyze", nil)
	recorder := httptest.NewRecorder()
	analyzer.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("unexpected method response: %d", recorder.Code)
	}
}

func TestHandlerRejectsUnknownFieldAndSemanticErrors(t *testing.T) {
	analyzer := newTestAnalyzer(t, nil)
	unknown := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(`{"unknown":true}`))
	unknown.Header.Set("Content-Type", "application/json")
	unknownResponse := httptest.NewRecorder()
	analyzer.Handler().ServeHTTP(unknownResponse, unknown)
	if unknownResponse.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", unknownResponse.Code)
	}

	request := validRequest()
	request.TaskID = "not-a-uuid"
	response := requestJSON(t, analyzer, request, "")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("semantic error status = %d", response.Code)
	}
}

func TestHandlerRequestLimit(t *testing.T) {
	analyzer := newTestAnalyzer(t, func(config *Config) { config.MaxRequestBytes = 64 })
	request := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(strings.Repeat("x", 65)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	analyzer.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestValidateRequestBoundaries(t *testing.T) {
	request := validRequest()
	request.Matches = make([]Match, MaxMatches)
	for i := range request.Matches {
		request.Matches[i] = Match{LineNumber: i + 1, Content: strings.Repeat("x", MaxMatchContentBytes)}
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("valid boundary rejected: %v", err)
	}
	request.Matches = append(request.Matches, Match{LineNumber: MaxMatches + 1})
	if err := ValidateRequest(request); err == nil {
		t.Fatal("expected 2001 matches to fail")
	}
	request = validRequest()
	request.Matches[0].Content = strings.Repeat("x", MaxMatchContentBytes+1)
	if err := ValidateRequest(request); err == nil {
		t.Fatal("expected oversized line to fail")
	}
}

func TestEmptyMatchesReturnStableResponse(t *testing.T) {
	analyzer := newTestAnalyzer(t, nil)
	request := validRequest()
	request.Matches = nil
	response := requestJSON(t, analyzer, request, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var result AnalysisResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Findings == nil || len(result.Findings) != 0 || !strings.Contains(result.Summary, "未发现") {
		t.Fatalf("unexpected empty result: %+v", result)
	}
}
