package logs

import (
	"bufio"
	"io"
	"strings"
)

const maxStoredMatchesPerFile = 2000

type parseSummary struct {
	Lines    int64
	Errors   int64
	Warnings int64
	Results  []ParseResult
}

func parseLog(reader io.Reader) (parseSummary, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var summary parseSummary
	for scanner.Scan() {
		summary.Lines++
		line := scanner.Text()
		upper := strings.ToUpper(line)
		level, matched := "", ""
		switch {
		case strings.Contains(upper, "FATAL"):
			level, matched = "error", "FATAL"
		case strings.Contains(upper, "ERROR"):
			level, matched = "error", "ERROR"
		case strings.Contains(upper, "WARNING"):
			level, matched = "warning", "WARNING"
		case strings.Contains(upper, "WARN"):
			level, matched = "warning", "WARN"
		}
		if level == "" {
			continue
		}
		if level == "error" {
			summary.Errors++
		} else {
			summary.Warnings++
		}
		if len(summary.Results) < maxStoredMatchesPerFile {
			if len(line) > 4000 {
				line = line[:4000]
			}
			summary.Results = append(summary.Results, ParseResult{Level: level, MatchedText: matched, LineNumber: summary.Lines, Content: line})
		}
	}
	return summary, scanner.Err()
}
