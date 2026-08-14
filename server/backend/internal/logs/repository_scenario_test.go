package logs

import (
	"strings"
	"testing"
	"time"
)

func TestRulesFromScenariosUseOnlyEnabledScenarioChecks(t *testing.T) {
	ruleOneID := int64(1)
	ruleTwoID := int64(2)
	rules := []ParseRule{
		{ID: ruleOneID, Name: "场景引用规则", Keyword: "old-keyword", Level: "warning", Enabled: false},
		{ID: ruleTwoID, Name: "全局开启规则", Keyword: "global-keyword", Level: "critical", Enabled: true},
	}
	scenarios := []scenarioSnapshot{{Name: "开关机测试", Checks: []ScenarioCheck{
		{Enabled: true, Source: "rule", RuleID: &ruleOneID, Keywords: []string{"scene-keyword"}},
		{Enabled: false, Source: "rule", RuleID: &ruleTwoID},
		{Enabled: true, Source: "custom", Name: "自定义检测", Severity: "critical", Keywords: []string{"custom-a", "custom-b"}},
	}}}

	result, err := rulesFromScenarios(rules, scenarios, true)
	if err != nil {
		t.Fatalf("build scenario rules: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("rules count = %d, want 2", len(result))
	}
	if result[0].ID != ruleOneID || !result[0].Enabled || result[0].Keyword != "scene-keyword" {
		t.Fatalf("referenced scenario rule = %#v", result[0])
	}
	if result[1].ID != 0 || result[1].Name != "自定义检测" || result[1].Keyword != "custom-a|custom-b" || result[1].Level != "critical" || !result[1].Enabled {
		t.Fatalf("custom scenario rule = %#v", result[1])
	}
	for _, resultRule := range result {
		if resultRule.ID == ruleTwoID {
			t.Fatal("globally enabled but unselected rule must not run when scenarios are selected")
		}
	}

	summary, err := parseLogWithRules(strings.NewReader("global-keyword\ncustom-a\n"), result, time.Now())
	if err != nil {
		t.Fatalf("parse scenario keywords: %v", err)
	}
	if len(summary.Results) != 1 || summary.Results[0].MatchedText != "custom-a" {
		t.Fatalf("scenario parse results = %#v", summary.Results)
	}
}

func TestRulesFromScenariosAcceptLegacyCustomChecks(t *testing.T) {
	result, err := rulesFromScenarios(nil, []scenarioSnapshot{{Name: "旧场景", Checks: []ScenarioCheck{
		{Enabled: true, Name: "旧关键词", Severity: "warning", Keywords: []string{"legacy"}},
	}}}, true)
	if err != nil || len(result) != 1 || result[0].Keyword != "legacy" {
		t.Fatalf("legacy custom rules = %#v, error = %v", result, err)
	}
}

func TestRulesFromScenariosRejectUnavailableRule(t *testing.T) {
	missingRuleID := int64(99)
	_, err := rulesFromScenarios([]ParseRule{{ID: 1}}, []scenarioSnapshot{{Checks: []ScenarioCheck{
		{Enabled: true, Source: "rule", RuleID: &missingRuleID},
	}}}, true)
	if err != ErrScenarioRuleUnavailable {
		t.Fatalf("error = %v, want ErrScenarioRuleUnavailable", err)
	}
}

func TestRulesFromScenariosCanIncludeEnabledParsingRules(t *testing.T) {
	scenarioRuleID := int64(1)
	globalRuleID := int64(2)
	disabledRuleID := int64(3)
	rules := []ParseRule{
		{ID: scenarioRuleID, Name: "场景引用规则", Keyword: "old", Level: "warning", Enabled: false},
		{ID: globalRuleID, Name: "已启用解析规则", Keyword: "global", Level: "critical", Enabled: true},
		{ID: disabledRuleID, Name: "已禁用解析规则", Keyword: "disabled", Level: "warning", Enabled: false},
	}
	scenarios := []scenarioSnapshot{{Name: "开关机测试", Checks: []ScenarioCheck{
		{Enabled: true, Source: "rule", RuleID: &scenarioRuleID, Keywords: []string{"scene"}},
	}}}

	result, err := rulesFromScenarios(rules, scenarios, false)
	if err != nil {
		t.Fatalf("build combined rules: %v", err)
	}
	if len(result) != 2 || result[0].ID != scenarioRuleID || result[1].ID != globalRuleID {
		t.Fatalf("combined rules = %#v", result)
	}
}
