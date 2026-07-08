package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Diagnosis 保存 AI 对日志行的分析结果。
type Diagnosis struct {
	AnomalyType string `json:"anomaly_type"`
	Severity    string `json:"severity"`
	RootCause   string `json:"root_cause"`
	Suggestion  string `json:"suggestion"`
}

// Analyzer 使用 Ollama 分析异常日志行。
type Analyzer struct {
	client OllamaClient
	model  string
}

// NewAnalyzer 创建一个新的 Analyzer。
func NewAnalyzer(client OllamaClient, model string) *Analyzer {
	return &Analyzer{client: client, model: model}
}

// Analyze 将日志行发送给 Ollama 进行分析，并返回诊断结果。
func (a *Analyzer) Analyze(logLine string) (*Diagnosis, error) {
	prompt := buildPrompt(logLine)
	response, err := a.client.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	return parseDiagnosis(response)
}

// buildPrompt 构造 Ollama 的分析提示词。
func buildPrompt(logLine string) string {
	return fmt.Sprintf(`You are a log analysis expert for embedded dashcam firmware. Analyze the following error log and return a JSON diagnosis.

Log line: %s

Return ONLY valid JSON in this format:
{"anomaly_type":"<type>","severity":"<ERROR|WARN>","root_cause":"<brief root cause>","suggestion":"<fix suggestion>"}

Do not include any other text.`, logLine)
}

// parseDiagnosis 从 Ollama 响应中提取 JSON 并解析为诊断结果。
func parseDiagnosis(response string) (*Diagnosis, error) {
	// 在响应中查找 JSON（Ollama 可能会附带额外文本）
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