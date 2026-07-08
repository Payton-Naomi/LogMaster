package rule

import (
	"regexp"
	"strings"
)

// Rule 定义一条日志解析规则。
type Rule struct {
	Name     string
	Keywords []string
	Pattern  string
	Severity string
	Category string
}

// MatchResult 保存日志行与规则匹配的结果。
type MatchResult struct {
	Matched  bool
	RuleName string
	Severity string
	Category string
	Tags     []string
}

// Match 检查日志行是否匹配此规则。
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