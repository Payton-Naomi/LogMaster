package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxAgentResponseBytes = 4 << 20

// AgentAnalyzer is the extension point for AI or rule-agent log diagnosis.
type AgentAnalyzer interface {
	Provider() string
	Analyze(context.Context, AgentAnalysisRequest) (AgentAnalysisResponse, error)
}

type AgentAnalysisRequest struct {
	TaskID     string        `json:"task_id"`
	UploadID   string        `json:"upload_id"`
	File       LogFile       `json:"file"`
	TotalLines int64         `json:"total_lines"`
	Matches    []ParseResult `json:"matches"`
}

type AgentFinding struct {
	Category   string  `json:"category"`
	Severity   string  `json:"severity"`
	RootCause  string  `json:"root_cause"`
	Suggestion string  `json:"suggestion"`
	Evidence   string  `json:"evidence,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type AgentAnalysisResponse struct {
	Summary  string         `json:"summary"`
	Findings []AgentFinding `json:"findings"`
}

type HTTPAgentAnalyzer struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewHTTPAgentAnalyzer(endpoint, token string, timeout time.Duration) *HTTPAgentAnalyzer {
	return &HTTPAgentAnalyzer{
		endpoint: strings.TrimRight(endpoint, "/"),
		token:    token,
		client:   &http.Client{Timeout: timeout},
	}
}

func (a *HTTPAgentAnalyzer) Provider() string { return "http-agent" }

func (a *HTTPAgentAnalyzer) Analyze(ctx context.Context, request AgentAnalysisRequest) (AgentAnalysisResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return AgentAnalysisResponse{}, fmt.Errorf("marshal agent request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(payload))
	if err != nil {
		return AgentAnalysisResponse{}, fmt.Errorf("create agent request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+a.token)
	}

	httpResponse, err := a.client.Do(httpRequest)
	if err != nil {
		return AgentAnalysisResponse{}, fmt.Errorf("call agent: %w", err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 1024))
		return AgentAnalysisResponse{}, fmt.Errorf("agent returned %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(body)))
	}
	var result AgentAnalysisResponse
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, maxAgentResponseBytes))
	if err := decoder.Decode(&result); err != nil {
		return AgentAnalysisResponse{}, fmt.Errorf("decode agent response: %w", err)
	}
	if result.Findings == nil {
		result.Findings = []AgentFinding{}
	}
	return result, nil
}

type AgentAnalysisRecord struct {
	ID           int64          `json:"id"`
	TaskID       string         `json:"task_id"`
	LogFileID    int64          `json:"log_file_id"`
	FilePath     string         `json:"file_path"`
	Provider     string         `json:"provider"`
	Status       string         `json:"status"`
	Summary      string         `json:"summary"`
	Findings     []AgentFinding `json:"findings"`
	ErrorMessage string         `json:"error_message,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
