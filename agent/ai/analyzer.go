package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Diagnosis holds the AI analysis result for a log line.
type Diagnosis struct {
	AnomalyType string `json:"anomaly_type"`
	Severity    string `json:"severity"`
	RootCause   string `json:"root_cause"`
	Suggestion  string `json:"suggestion"`
}

// Analyzer uses Ollama to analyze anomalous log lines.
type Analyzer struct {
	client OllamaClient
	model  string
}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer(client OllamaClient, model string) *Analyzer {
	return &Analyzer{client: client, model: model}
}

// Analyze sends a log line to Ollama for analysis and returns a diagnosis.
func (a *Analyzer) Analyze(logLine string) (*Diagnosis, error) {
	prompt := buildPrompt(logLine)
	response, err := a.client.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	return parseDiagnosis(response)
}

// buildPrompt creates the analysis prompt for Ollama.
func buildPrompt(logLine string) string {
	return fmt.Sprintf(`You are a log analysis expert for embedded dashcam firmware. Analyze the following error log and return a JSON diagnosis.

Log line: %s

Return ONLY valid JSON in this format:
{"anomaly_type":"<type>","severity":"<ERROR|WARN>","root_cause":"<brief root cause>","suggestion":"<fix suggestion>"}

Do not include any other text.`, logLine)
}

// parseDiagnosis extracts JSON from Ollama response and parses it.
func parseDiagnosis(response string) (*Diagnosis, error) {
	// Find JSON in response (Ollama may add extra text)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in response: %s", response)
	}
	jsonStr := response[start : end+1]

	var d Diagnosis
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		return nil, fmt.Errorf("parse diagnosis: %w", err)
	}
	return &d, nil
}