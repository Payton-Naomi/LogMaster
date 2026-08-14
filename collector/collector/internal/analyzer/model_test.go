package analyzer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaClientCompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["model"] != "local-model" || request["stream"] != false {
			t.Errorf("unexpected request: %+v", request)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"response": validModelJSON()})
	}))
	defer server.Close()

	client := NewOllamaClient(ModelConfig{BaseURL: server.URL, Model: "local-model"}, time.Second)
	result, err := client.Generate(context.Background(), "prompt")
	if err != nil || result != validModelJSON() {
		t.Fatalf("result=%q err=%v", result, err)
	}
}

func TestQwenOpenAICompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer injected-key" {
			t.Errorf("authorization header is incorrect")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": validModelJSON()}}},
		})
	}))
	defer server.Close()

	client := NewQwenClient(ModelConfig{BaseURL: server.URL + "/v1", Model: "cloud-model", APIKey: "injected-key"}, time.Second)
	result, err := client.Generate(context.Background(), "prompt")
	if err != nil || result != validModelJSON() {
		t.Fatalf("result=%q err=%v", result, err)
	}
}

func TestDefaultConfigContainsNoModelCredentials(t *testing.T) {
	config := DefaultConfig()
	if config.Mode != ModeRules || config.Ollama.BaseURL != "" || config.Ollama.Model != "" ||
		config.Qwen.BaseURL != "" || config.Qwen.Model != "" || config.Qwen.APIKey != "" || config.Token != "" {
		t.Fatalf("default config contains model or token values: %+v", config)
	}
	if _, err := New(config, nil); err != nil {
		t.Fatal(err)
	}
	config.Mode = ModeRulesThenOllama
	if _, err := New(config, nil); err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("expected missing Ollama config error, got %v", err)
	}
}
