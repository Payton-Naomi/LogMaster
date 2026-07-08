package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaClient 定义 Ollama API 通信接口。
type OllamaClient interface {
	Generate(prompt string) (string, error)
}

// ollamaClient 是 OllamaClient 的默认 HTTP 实现。
type ollamaClient struct {
	endpoint string
	model    string
	client   *http.Client
}

// ollamaRequest 是 POST /api/generate 的请求体。
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaResponse 是 POST /api/generate 的响应体。
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewOllamaClient 创建一个新的 Ollama HTTP 客户端。
func NewOllamaClient(endpoint, model string) *ollamaClient {
	return &ollamaClient{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

// Generate 向 Ollama 发送提示词并返回响应。
func (c *ollamaClient) Generate(prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.client.Post(c.endpoint+"/api/generate", "application/json", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Response, nil
}