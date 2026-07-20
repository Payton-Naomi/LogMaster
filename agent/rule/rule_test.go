package rule

import (
    "testing"
)

func TestRuleMatchKeywordsAllPresent(t *testing.T) {
    r := Rule{
        Name:     "crash",
        Keywords: []string{"panic", "fatal"},
        Severity: "ERROR",
        Category: "crash",
    }
    result := r.Match("fatal panic: runtime error")
    if !result.Matched {
        t.Fatal("should match when all keywords present")
    }
    if result.Severity != "ERROR" {
        t.Fatalf("severity = %s, want ERROR", result.Severity)
    }
    if result.Category != "crash" {
        t.Fatalf("category = %s, want crash", result.Category)
    }
}

func TestRuleMatchKeywordsMissing(t *testing.T) {
    r := Rule{
        Name:     "crash",
        Keywords: []string{"panic", "fatal"},
        Severity: "ERROR",
        Category: "crash",
    }
    result := r.Match("just a warning message")
    if result.Matched {
        t.Fatal("should not match when keyword missing")
    }
}

func TestRuleMatchWithPattern(t *testing.T) {
    r := Rule{
        Name:     "error_code",
        Keywords: []string{"error"},
        Pattern:  `ERR_\d{3}`,
        Severity: "ERROR",
        Category: "system",
    }
    result := r.Match("system error: ERR_502 occurred")
    if !result.Matched {
        t.Fatal("should match when keyword and pattern match")
    }
}

func TestRuleMatchPatternNoMatch(t *testing.T) {
    r := Rule{
        Name:     "error_code",
        Keywords: []string{"error"},
        Pattern:  `ERR_\d{3}`,
        Severity: "ERROR",
        Category: "system",
    }
    result := r.Match("system error: something went wrong")
    if result.Matched {
        t.Fatal("should not match when pattern doesn't match")
    }
}

func TestRuleMatchCaseInsensitive(t *testing.T) {
    r := Rule{
        Name:     "timeout",
        Keywords: []string{"timeout"},
        Severity: "WARN",
        Category: "network",
    }
    result := r.Match("Connection TIMEOUT occurred")
    if !result.Matched {
        t.Fatal("should match case-insensitively")
    }
}

func TestEngineMatch(t *testing.T) {
    rules := []Rule{
        {Name: "crash", Keywords: []string{"panic"}, Severity: "ERROR", Category: "crash"},
        {Name: "timeout", Keywords: []string{"timeout"}, Severity: "WARN", Category: "network"},
    }
    engine := NewEngine(rules)
    
    result := engine.Match("panic: nil pointer")
    if !result.Matched {
        t.Fatal("should match crash rule")
    }
    if result.RuleName != "crash" {
        t.Fatalf("rule = %s, want crash", result.RuleName)
    }
}

func TestEngineNoMatch(t *testing.T) {
    engine := NewEngine([]Rule{})
    result := engine.Match("normal log message")
    if result.Matched {
        t.Fatal("should not match with no rules")
    }
    if result.Severity != "INFO" {
        t.Fatalf("default severity = %s, want INFO", result.Severity)
    }
}

func TestEngineFirstMatchWins(t *testing.T) {
    rules := []Rule{
        {Name: "crash", Keywords: []string{"panic"}, Severity: "ERROR", Category: "crash"},
        {Name: "memory", Keywords: []string{"panic"}, Severity: "WARN", Category: "memory"},
    }
    engine := NewEngine(rules)
    result := engine.Match("panic: something")
    if result.RuleName != "crash" {
        t.Fatalf("first match should win, got %s", result.RuleName)
    }
}
