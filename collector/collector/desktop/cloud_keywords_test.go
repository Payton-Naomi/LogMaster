package main

import (
	"logmaster-agent/agent/internal/backend"
	"testing"
)

func TestMergeCloudKeywordsCreatesReadOnlyStableRules(t *testing.T) {
	catalog := CatalogConfig{SchemaVersion: 1, Projects: []CatalogProject{{ID: "p1", Name: "项目", Tasks: []CatalogTask{{ID: "t1", Name: "任务", KeywordProfiles: []CatalogKeywordProfile{{ID: "local", Name: "本地"}}}}}}}
	merged := mergeCloudKeywords(catalog, []backend.StandardKeyword{{ID: 7, Name: "设备断连", Category: "connection", Scope: "line", Keyword: "disconnect", Level: "warning", Description: "链路断开"}})
	profiles := merged.Projects[0].Tasks[0].KeywordProfiles
	if len(profiles) != 2 || profiles[0].ID != "local" || profiles[1].ID != cloudKeywordProfileID {
		t.Fatalf("profiles = %#v", profiles)
	}
	rule := profiles[1].Groups[0].Rules[0]
	if rule.ID != "cloud-rule-7" || rule.Mode != "contains" || !rule.ReadOnly || rule.Level != "warning" || rule.Description != "链路断开" {
		t.Fatalf("cloud rule = %#v", rule)
	}
}

func TestMergeCloudKeywordsReplacesPreviousCloudProfile(t *testing.T) {
	catalog := CatalogConfig{Projects: []CatalogProject{{Tasks: []CatalogTask{{KeywordProfiles: []CatalogKeywordProfile{{ID: cloudKeywordProfileID}, {ID: "local"}}}}}}}
	merged := mergeCloudKeywords(catalog, []backend.StandardKeyword{{ID: 8, Name: "重启", Category: "system", Scope: "全局", Keyword: "reboot"}})
	if got := len(merged.Projects[0].Tasks[0].KeywordProfiles); got != 2 {
		t.Fatalf("cloud profile duplicated: %d", got)
	}
}
