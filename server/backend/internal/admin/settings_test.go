package admin

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseKeywordTXTUsesDefaultsAndSkipsDuplicates(t *testing.T) {
	rules, skipped, err := parseKeywordFile(
		multipartFile{Reader: strings.NewReader("# comment\nFATAL_DEVICE\n\nFatal_Device\nSD write failed\n")},
		&multipart.FileHeader{Filename: "keywords.txt"},
		keywordDefaults{Category: "system", Level: "critical", Scope: "DR2861", Description: "研发异常"},
	)
	if err != nil {
		t.Fatalf("parse TXT: %v", err)
	}
	if len(rules) != 2 || skipped != 3 {
		t.Fatalf("rules = %d, skipped = %d, want 2 and 3", len(rules), skipped)
	}
	if rules[0].Keyword != "FATAL_DEVICE" || rules[0].Level != "critical" || rules[0].Scope != "DR2861" {
		t.Fatalf("unexpected first rule: %#v", rules[0])
	}
}

func TestParseKeywordCSVSupportsChineseHeadersAndOverridesDefaults(t *testing.T) {
	content := "规则名称,关键词,分类,级别,适用范围,说明\n录像失败,XA_WRITE_FAIL,recording,warning,DR7800,写盘异常\n"
	rules, skipped, err := parseKeywordFile(
		multipartFile{Reader: strings.NewReader(content)},
		&multipart.FileHeader{Filename: "keywords.csv"},
		keywordDefaults{Category: "system", Level: "critical", Scope: "全局"},
	)
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rules) != 1 || skipped != 0 {
		t.Fatalf("rules = %d, skipped = %d, want 1 and 0", len(rules), skipped)
	}
	if rules[0].Name != "录像失败" || rules[0].Category != "recording" || rules[0].Level != "warning" || rules[0].Scope != "DR7800" {
		t.Fatalf("unexpected CSV rule: %#v", rules[0])
	}
}

func TestNormalizeKeywordRuleAcceptsCompletePublicRule(t *testing.T) {
	rule := keywordRule{
		Name:        "  Write failure  ",
		Category:    " RECORDING ",
		Keyword:     "  XA_WRITE_FAIL ",
		Scope:       " global ",
		Level:       " CRITICAL ",
		Description: " write failed ",
	}
	if err := normalizeKeywordRule(&rule); err != nil {
		t.Fatalf("normalizeKeywordRule() error = %v", err)
	}
	if rule.Name != "Write failure" || rule.Category != "recording" || rule.Keyword != "XA_WRITE_FAIL" || rule.Level != "critical" {
		t.Fatalf("unexpected normalized rule: %#v", rule)
	}
}

func TestNormalizeKeywordRuleRejectsInvalidPublicRule(t *testing.T) {
	for _, rule := range []keywordRule{
		{Name: "", Category: "system", Keyword: "fatal", Level: "critical"},
		{Name: "rule", Category: "unknown", Keyword: "fatal", Level: "critical"},
		{Name: "rule", Category: "system", Keyword: "fatal", Level: "error"},
	} {
		if err := normalizeKeywordRule(&rule); err != errInvalidKeywordRule {
			t.Fatalf("normalizeKeywordRule(%#v) error = %v, want errInvalidKeywordRule", rule, err)
		}
	}
}

func TestRequireKeywordRuleAdminAllowsKeywordRoles(t *testing.T) {
	for _, role := range []string{roleDeveloper, roleAdmin, roleSuperAdmin} {
		service := &Service{
			currentUserResolver: func(*http.Request) (string, bool) { return "ou_test", true },
			roleResolver:        func(context.Context, string) (string, error) { return role, nil },
		}
		if _, ok := service.requireKeywordRuleAdmin(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/admin/keyword-rules", nil)); !ok {
			t.Fatalf("role %q should manage keyword rules", role)
		}
	}
}

func TestRequireKeywordRuleAdminRejectsUser(t *testing.T) {
	service := &Service{
		currentUserResolver: func(*http.Request) (string, bool) { return "ou_test", true },
		roleResolver:        func(context.Context, string) (string, error) { return roleUser, nil },
	}
	recorder := httptest.NewRecorder()
	if _, ok := service.requireKeywordRuleAdmin(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keyword-rules", nil)); ok {
		t.Fatal("ordinary user should not manage keyword rules")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

type multipartFile struct {
	*strings.Reader
}

func (multipartFile) Close() error { return nil }
