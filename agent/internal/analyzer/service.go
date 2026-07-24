package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Analyzer struct {
	config    Config
	cache     Cache
	models    []Generator
	semaphore chan struct{}
	flights   flightGroup
	now       func() time.Time
}

func New(config Config, cache Cache) (*Analyzer, error) {
	config.applyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var ollama Generator
	var qwen Generator
	if config.Mode == ModeRulesThenOllama || config.Mode == ModeRulesOllamaThenQwen {
		ollama = NewOllamaClient(config.Ollama, config.Timeout)
	}
	if config.Mode == ModeRulesThenQwen || config.Mode == ModeRulesOllamaThenQwen {
		qwen = NewQwenClient(config.Qwen, config.Timeout)
	}
	return newWithGenerators(config, cache, ollama, qwen), nil
}

func NewWithGenerators(config Config, cache Cache, ollama, qwen Generator) (*Analyzer, error) {
	config.applyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return newWithGenerators(config, cache, ollama, qwen), nil
}

func newWithGenerators(config Config, cache Cache, ollama, qwen Generator) *Analyzer {
	if cache == nil {
		cache = NewMemoryCache()
	}
	models := make([]Generator, 0, 2)
	switch config.Mode {
	case ModeRulesThenOllama:
		models = appendGenerator(models, ollama)
	case ModeRulesThenQwen:
		models = appendGenerator(models, qwen)
	case ModeRulesOllamaThenQwen:
		models = appendGenerator(models, ollama)
		models = appendGenerator(models, qwen)
	}
	return &Analyzer{
		config: config, cache: cache, models: models,
		semaphore: make(chan struct{}, config.MaxConcurrent), now: time.Now,
	}
}

func appendGenerator(models []Generator, generator Generator) []Generator {
	if generator != nil {
		return append(models, generator)
	}
	return models
}

func (a *Analyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if err := ValidateRequest(req); err != nil {
		return AnalysisResponse{}, err
	}
	key := AnalysisKey(req)
	if cached, ok, err := a.cache.Get(ctx, key); err == nil && ok {
		return cached, nil
	}
	return a.flights.Do(ctx, key, func() (AnalysisResponse, error) {
		if cached, ok, err := a.cache.Get(ctx, key); err == nil && ok {
			return cached, nil
		}
		select {
		case a.semaphore <- struct{}{}:
			defer func() { <-a.semaphore }()
		case <-ctx.Done():
			return AnalysisResponse{}, ctx.Err()
		}

		result := analyzeRules(req)
		if len(req.Matches) > 0 {
			prompt, err := buildPrompt(req)
			if err != nil {
				return AnalysisResponse{}, fmt.Errorf("build model prompt: %w", err)
			}
			for _, model := range a.models {
				modelResult, err := analyzeWithRepair(ctx, model, prompt)
				if err != nil {
					continue
				}
				result = mergeResponses(modelResult, result)
				if !hasLowConfidence(modelResult) {
					break
				}
			}
		}
		result, err := fitResponse(result, a.config.MaxFindings)
		if err != nil {
			return AnalysisResponse{}, err
		}
		_ = a.cache.Set(ctx, key, result, a.now().Add(a.config.CacheTTL))
		return result, nil
	})
}

func analyzeWithRepair(ctx context.Context, model Generator, prompt string) (AnalysisResponse, error) {
	raw, err := model.Generate(ctx, prompt)
	if err != nil {
		return AnalysisResponse{}, err
	}
	response, err := parseModelResponse(raw)
	if err == nil {
		return response, nil
	}
	repaired, repairErr := model.Generate(ctx, buildRepairPrompt(raw))
	if repairErr != nil {
		return AnalysisResponse{}, err
	}
	response, repairErr = parseModelResponse(repaired)
	if repairErr != nil {
		return AnalysisResponse{}, repairErr
	}
	return response, nil
}

func parseModelResponse(raw string) (AnalysisResponse, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if index := strings.IndexByte(raw, '\n'); index >= 0 {
			raw = raw[index+1:]
		}
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "```"))
	}
	var response AnalysisResponse
	if err := decodeStrictJSON([]byte(raw), &response); err != nil {
		return AnalysisResponse{}, err
	}
	if err := ValidateResponse(response); err != nil {
		return AnalysisResponse{}, err
	}
	return response, nil
}

func mergeResponses(primary, fallback AnalysisResponse) AnalysisResponse {
	result := AnalysisResponse{Summary: primary.Summary, Findings: make([]Finding, 0, len(primary.Findings)+len(fallback.Findings))}
	seen := make(map[string]struct{})
	for _, response := range []AnalysisResponse{primary, fallback} {
		for _, finding := range response.Findings {
			key := finding.Category + "\x00" + finding.Severity
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result.Findings = append(result.Findings, finding)
		}
	}
	if strings.TrimSpace(result.Summary) == "" {
		result.Summary = fallback.Summary
	}
	return result
}

func hasLowConfidence(response AnalysisResponse) bool {
	if len(response.Findings) == 0 {
		return true
	}
	for _, finding := range response.Findings {
		if finding.Confidence < 0.85 {
			return true
		}
	}
	return false
}

func fitResponse(response AnalysisResponse, maxFindings int) (AnalysisResponse, error) {
	response.Summary = truncateUTF8(strings.TrimSpace(response.Summary), 32<<10)
	if response.Findings == nil {
		response.Findings = []Finding{}
	}
	if len(response.Findings) > maxFindings {
		response.Findings = response.Findings[:maxFindings]
	}
	for i := range response.Findings {
		finding := &response.Findings[i]
		finding.RootCause = truncateUTF8(strings.TrimSpace(finding.RootCause), 16<<10)
		finding.Suggestion = truncateUTF8(strings.TrimSpace(finding.Suggestion), 16<<10)
		finding.Evidence = truncateUTF8(strings.TrimSpace(finding.Evidence), 64<<10)
	}
	if err := ValidateResponse(response); err != nil {
		return AnalysisResponse{}, err
	}
	for {
		data, err := json.Marshal(response)
		if err != nil {
			return AnalysisResponse{}, err
		}
		if len(data)+1 <= MaxResponseBytes {
			return response, nil
		}
		if len(response.Findings) == 0 {
			return AnalysisResponse{}, errors.New("analysis response exceeds 1 MiB")
		}
		response.Findings = response.Findings[:len(response.Findings)-1]
	}
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
