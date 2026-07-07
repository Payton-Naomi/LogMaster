package rule

// Engine matches log lines against a set of rules.
type Engine struct {
	rules []Rule
}

// NewEngine creates a new Engine with the given rules.
func NewEngine(rules []Rule) *Engine {
	return &Engine{rules: rules}
}

// Match applies rules in order and returns the first match, or a default result.
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