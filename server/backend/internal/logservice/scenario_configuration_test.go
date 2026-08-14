package logservice

import (
	"context"
	"encoding/json"
	"testing"
)

func TestValidateSimplifiedScenarioBuildsAnalysisCheck(t *testing.T) {
	repository := &Repository{}
	prepared, err := repository.ValidateAndSnapshotScenario(context.Background(), "ou_test", TestScenario{
		ID:          "power-cycle",
		Name:        "开关机专项测试",
		Remark:      "检查异常重启",
		Enabled:     true,
		AllProjects: true,
		Projects:    []string{},
		Keywords:    []string{"backtrace", " backtrace ", "POWER_ID_SWRT"},
	})
	if err != nil {
		t.Fatalf("validate scenario: %v", err)
	}
	if !prepared.Enabled || !prepared.AllProjects {
		t.Fatalf("prepared state = enabled %v, all projects %v", prepared.Enabled, prepared.AllProjects)
	}
	if prepared.Remark != "检查异常重启" || len(prepared.Keywords) != 2 {
		t.Fatalf("prepared scenario = %#v", prepared)
	}

	var checks []ScenarioCheck
	if err := json.Unmarshal(prepared.Checks, &checks); err != nil {
		t.Fatalf("decode checks: %v", err)
	}
	if len(checks) != 1 || !checks[0].Enabled || checks[0].Severity != "critical" || checks[0].MatchType != "forbidden" || len(checks[0].Keywords) != 2 {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestValidateSimplifiedScenarioCanBeDisabled(t *testing.T) {
	repository := &Repository{}
	prepared, err := repository.ValidateAndSnapshotScenario(context.Background(), "ou_test", TestScenario{
		ID:          "sd-card",
		Name:        "SD卡专项测试",
		Enabled:     false,
		AllProjects: true,
		Keywords:    []string{"I/O error"},
	})
	if err != nil {
		t.Fatalf("validate scenario: %v", err)
	}
	var metadata ScenarioMetadata
	if err := json.Unmarshal(prepared.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata.Status != "disabled" || prepared.Enabled {
		t.Fatalf("disabled scenario = metadata %#v, enabled %v", metadata, prepared.Enabled)
	}
}

func TestValidateScenarioPreservesStructuredKeywordChecks(t *testing.T) {
	repository := &Repository{}
	checks, err := json.Marshal([]ScenarioCheck{
		{ID: "keyword-1", Name: "disk full", Description: "存储空间不足", Severity: "warning", Enabled: true, Source: "custom", MatchType: "forbidden", MinCount: 1, Keywords: []string{"disk full"}},
		{ID: "keyword-2", Name: "retry", Description: "发生重试", Severity: "info", Enabled: true, Source: "custom", MatchType: "forbidden", MinCount: 1, Keywords: []string{"retry"}},
	})
	if err != nil {
		t.Fatalf("encode checks: %v", err)
	}
	prepared, err := repository.ValidateAndSnapshotScenario(context.Background(), "ou_test", TestScenario{
		ID:          "structured",
		Name:        "结构化关键词",
		Enabled:     true,
		AllProjects: true,
		Checks:      checks,
	})
	if err != nil {
		t.Fatalf("validate scenario: %v", err)
	}
	var actual []ScenarioCheck
	if err := json.Unmarshal(prepared.Checks, &actual); err != nil {
		t.Fatalf("decode checks: %v", err)
	}
	if len(actual) != 2 || actual[0].Severity != "warning" || actual[0].Description != "存储空间不足" || actual[1].Severity != "info" || actual[1].Description != "发生重试" || actual[1].MatchType != "forbidden" {
		t.Fatalf("structured checks = %#v", actual)
	}
}
