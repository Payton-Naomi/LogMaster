package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type generatorFunc func(context.Context, string) (string, error)

func (f generatorFunc) Generate(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

func modelConfig(mode Mode) Config {
	config := DefaultConfig()
	config.Mode = mode
	config.Ollama = ModelConfig{BaseURL: "http://127.0.0.1:11434", Model: "test"}
	config.Qwen = ModelConfig{BaseURL: "https://example.invalid/v1", Model: "test", APIKey: "test-key"}
	return config
}

func validModelJSON() string {
	return `{"summary":"录像模块异常","findings":[{"category":"recording","severity":"error","root_cause":"录像服务失败","suggestion":"检查编码链路","evidence":"ERROR recorder failed","confidence":0.95}]}`
}

func TestSingleflightAndCacheAvoidDuplicateModelCalls(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	generator := generatorFunc(func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return validModelJSON(), nil
	})
	analyzer, err := NewWithGenerators(modelConfig(ModeRulesThenOllama), NewMemoryCache(), generator, nil)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	var wg sync.WaitGroup
	errorsFound := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, analyzeErr := analyzer.Analyze(context.Background(), validRequest())
			errorsFound <- analyzeErr
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(errorsFound)
	for analyzeErr := range errorsFound {
		if analyzeErr != nil {
			t.Fatal(analyzeErr)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("model calls = %d", calls.Load())
	}
	if _, err := analyzer.Analyze(context.Background(), validRequest()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("cached call invoked model; calls = %d", calls.Load())
	}
}

func TestModelFailureFallsBackToRules(t *testing.T) {
	generator := generatorFunc(func(context.Context, string) (string, error) {
		return "", errors.New("model unavailable")
	})
	analyzer, err := NewWithGenerators(modelConfig(ModeRulesThenOllama), nil, generator, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := analyzer.Analyze(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Findings) == 0 || response.Findings[0].Category != "recording" {
		t.Fatalf("rules fallback missing: %+v", response)
	}
}

func TestQwenIsUsedOnlyWhenOllamaCannotProduceConfidentResult(t *testing.T) {
	var qwenCalls atomic.Int32
	qwen := generatorFunc(func(context.Context, string) (string, error) {
		qwenCalls.Add(1)
		return validModelJSON(), nil
	})

	ollamaFailure := generatorFunc(func(context.Context, string) (string, error) {
		return "", errors.New("offline")
	})
	analyzer, err := NewWithGenerators(modelConfig(ModeRulesOllamaThenQwen), nil, ollamaFailure, qwen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzer.Analyze(context.Background(), validRequest()); err != nil {
		t.Fatal(err)
	}
	if qwenCalls.Load() != 1 {
		t.Fatalf("Qwen calls after Ollama failure = %d", qwenCalls.Load())
	}

	qwenCalls.Store(0)
	ollamaSuccess := generatorFunc(func(context.Context, string) (string, error) {
		return validModelJSON(), nil
	})
	analyzer, err = NewWithGenerators(modelConfig(ModeRulesOllamaThenQwen), nil, ollamaSuccess, qwen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzer.Analyze(context.Background(), validRequest()); err != nil {
		t.Fatal(err)
	}
	if qwenCalls.Load() != 0 {
		t.Fatalf("Qwen was called after a confident Ollama result")
	}
}

func TestInvalidModelResponseIsRepairedOnce(t *testing.T) {
	var calls atomic.Int32
	generator := generatorFunc(func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 {
			return `{"summary":"bad","findings":[{"category":"invalid"}]}`, nil
		}
		return validModelJSON(), nil
	})
	analyzer, err := NewWithGenerators(modelConfig(ModeRulesThenOllama), nil, generator, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := analyzer.Analyze(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || response.Summary != "录像模块异常" {
		t.Fatalf("repair was not used: calls=%d response=%+v", calls.Load(), response)
	}
}

func TestPromptOmitsAccessiblePathsAndMarksLogsUntrusted(t *testing.T) {
	request := validRequest()
	request.File.RelativePath = "secret/root/logfile_0"
	request.Matches[0].FilePath = "C:\\private\\backend.log"
	prompt, err := buildPrompt(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret/root", "C:\\private", "file_path", "relative_path"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt leaked %q: %s", forbidden, prompt)
		}
	}
	if !strings.Contains(prompt, "logfile_0") || !strings.Contains(prompt, "不可信数据") {
		t.Fatalf("prompt lacks required safeguards: %s", prompt)
	}
}

func TestRuleOutputUsesOnlyContractEnums(t *testing.T) {
	request := validRequest()
	request.Matches = []Match{
		{Level: "INFO", LineNumber: 1, Content: "unclassified condition"},
		{Level: "FATAL", LineNumber: 2, Content: "camera crash"},
	}
	response := analyzeRules(request)
	if err := ValidateResponse(response); err != nil {
		t.Fatal(err)
	}
	for _, finding := range response.Findings {
		if _, ok := validCategories[finding.Category]; !ok {
			t.Fatalf("invalid category: %q", finding.Category)
		}
		if _, ok := validSeverities[finding.Severity]; !ok {
			t.Fatalf("invalid severity: %q", finding.Severity)
		}
	}
}

func TestResponseIsCappedByCountAndSize(t *testing.T) {
	findings := make([]Finding, 30)
	for i := range findings {
		findings[i] = Finding{
			Category: "system", Severity: "error", RootCause: "cause",
			Suggestion: "suggestion", Evidence: strings.Repeat("界", 100000), Confidence: 0.9,
		}
	}
	response, err := fitResponse(AnalysisResponse{Summary: "summary", Findings: findings}, 20)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Findings) > 20 || len(data)+1 > MaxResponseBytes {
		t.Fatalf("response not capped: findings=%d bytes=%d", len(response.Findings), len(data)+1)
	}
}

func TestMemoryCacheExpiresEntries(t *testing.T) {
	cache := NewMemoryCache()
	now := time.Unix(100, 0)
	cache.now = func() time.Time { return now }
	response := AnalysisResponse{Summary: "ok", Findings: []Finding{}}
	if err := cache.Set(context.Background(), "key", response, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := cache.Get(context.Background(), "key"); !ok {
		t.Fatal("cache entry missing")
	}
	now = now.Add(2 * time.Hour)
	if _, ok, _ := cache.Get(context.Background(), "key"); ok {
		t.Fatal("expired cache entry returned")
	}
}
