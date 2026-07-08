package rule

// Engine 将日志行与一组规则进行匹配。
type Engine struct {
	rules []Rule
}

// NewEngine 使用给定的规则创建一个新的 Engine。
func NewEngine(rules []Rule) *Engine {
	return &Engine{rules: rules}
}

// Match 按顺序应用规则，返回第一个匹配项，或返回默认结果。
func (e *Engine) Match(line string) MatchResult {
	for _, r := range e.rules {
		if result := r.Match(line); result.Matched {
			return result
		}
	}
	return MatchResult{
		Severity: "INFO",
		Category: "unknown",
	}
}