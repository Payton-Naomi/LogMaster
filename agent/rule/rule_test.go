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
        t.Fatal("所有关键字都存在时应匹配")
    }
    if result.Severity != "ERROR" {
        t.Fatalf("severity = %s, 期望 ERROR", result.Severity)
    }
    if result.Category != "crash" {
        t.Fatalf("category = %s, 期望 crash", result.Category)
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
        t.Fatal("缺少关键字时不应匹配")
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
        t.Fatal("关键字和正则都匹配时应匹配")
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
        t.Fatal("正则不匹配时不应匹配")
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
        t.Fatal("应不区分大小写匹配")
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
        t.Fatal("应匹配 crash 规则")
    }
    if result.RuleName != "crash" {
        t.Fatalf("rule = %s, 期望 crash", result.RuleName)
    }
}

func TestEngineNoMatch(t *testing.T) {
    engine := NewEngine([]Rule{})
    result := engine.Match("normal log message")
    if result.Matched {
        t.Fatal("无规则时不应匹配")
    }
    if result.Severity != "INFO" {
        t.Fatalf("default severity = %s, 期望 INFO", result.Severity)
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
        t.Fatalf("第一个匹配应胜出，实际为 %s", result.RuleName)
    }
}
