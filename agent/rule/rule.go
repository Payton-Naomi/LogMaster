package rule

import (
	"regexp"
	"strings"
)

// Rule defines a log parsing rule.
type Rule struct {
	Name     string
	Keywords []string
	Pattern  string
	Severity string
	Category string
}

// MatchResult holds the result of matching a log line against a rule.
type MatchResult struct {
	Matched  bool
	RuleName string
	Severity string
	Category string
	Tags     []string
}

// Match checks if the log line matches this rule.
func (r *Rule) Match(line string) MatchResult {
	for _, kw := range r.Keywords {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(kw)) {
			return MatchResult{}
		}
	}
	if r.Pattern != "" {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return MatchResult{}
		}
		if !re.MatchString(line) {
			return MatchResult{}
		}
	}
	return MatchResult{
		Matched:  true,
		RuleName: r.Name,
		Severity: r.Severity,
		Category: r.Category,
		Tags:     []string{r.Category, r.Severity},
	}
}