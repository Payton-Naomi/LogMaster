package logs

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const analysisContextLineWindow int64 = 50

var (
	fullTimestampPattern  = regexp.MustCompile(`^\[(\d{2})/(\d{2})\s+(\d{2}):(\d{2}):(\d{2}):(\d{3})`)
	shortTimestampPattern = regexp.MustCompile(`^\[(\d{2}):(\d{2}):(\d{2}):(\d{3})`)
)

type parseSummary struct {
	Lines    int64
	Errors   int64
	Warnings int64
	Results  []ParseResult
}

type parsedLine struct {
	Number    int64
	Timestamp *time.Time
	Level     string
	Content   string
	Causes    []RelatedCause
}

type compiledRule struct {
	Rule         ParseRule
	Alternatives []string
}

type configuredMatch struct {
	Text  string
	Start int
	End   int
}

type activeResult struct {
	Result  ParseResult
	EndLine int64
}

type causeDefinition struct {
	Kind       string
	Label      string
	Reason     string
	Confidence float64
	Pattern    *regexp.Regexp
}

var causeDefinitions = []causeDefinition{
	{Kind: "sd_removed", Label: "SD 卡被移除", Reason: "存储介质移除会中断正在进行的视频写入", Confidence: 0.99, Pattern: regexp.MustCompile(`(?i)sd removing|sd remove\b|card .* removed|IsInserted\s+0`)},
	{Kind: "block_io", Label: "块设备 I/O 异常", Reason: "底层块设备写入失败，可能导致文件系统和录像写入连续报错", Confidence: 0.98, Pattern: regexp.MustCompile(`(?i)blk_update_request: I/O error|Buffer I/O error|Input/output error`)},
	{Kind: "fat_read", Label: "FAT 文件系统读取失败", Reason: "文件系统元数据读取失败，可能由介质异常或非正常移除引起", Confidence: 0.96, Pattern: regexp.MustCompile(`(?i)FAT read failed|unable to read inode|Dirty bit is set|may be corrupt`)},
	{Kind: "allocation", Label: "录像空间分配失败", Reason: "录像文件扩容或预分配失败，后续写入可能无法继续", Confidence: 0.94, Pattern: regexp.MustCompile(`(?i)FALLOCATE_CON_CLUSTER failed|POOL_ALLOC failed|ensureCapacity:.*failed`)},
	{Kind: "no_sdcard", Label: "写入时未检测到 SD 卡", Reason: "录像线程执行写入时存储卡已经不可用", Confidence: 0.99, Pattern: regexp.MustCompile(`(?i)write no sdcard|storage sd 0`)},
	{Kind: "watchdog", Label: "看门狗或软件复位", Reason: "系统可能因看门狗超时或软件复位而异常重启", Confidence: 0.92, Pattern: regexp.MustCompile(`(?i)POWER_ID_SWRT|watchdog|2f0050080`)},
	{Kind: "crash", Label: "系统或应用崩溃", Reason: "崩溃堆栈或信号信息可能是当前错误的上游原因", Confidence: 0.9, Pattern: regexp.MustCompile(`(?i)backtrace|Log_Signal_Data|panic|assert`)},
}

// parseLog keeps the original parser entry point for tests and callers that do not load database rules.
func parseLog(reader io.Reader) (parseSummary, error) {
	rules := []ParseRule{
		{Name: "通用错误", Category: "system", Keyword: "FATAL|ERROR", Level: "critical", Enabled: true, Priority: 900},
		{Name: "通用警告", Category: "system", Keyword: "WARNING|WARN", Level: "warning", Enabled: true, Priority: 950},
	}
	return parseLogWithRules(reader, rules, time.Now())
}

func parseLogWithRules(reader io.Reader, rules []ParseRule, now time.Time) (parseSummary, error) {
	compiled := compileRules(rules)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var summary parseSummary
	var recent []parsedLine
	var active []*activeResult
	var lastTimestamp *time.Time

	for scanner.Scan() {
		summary.Lines++
		content := scanner.Text()
		timestamp := parseTimestamp(content, lastTimestamp, now)
		if timestamp != nil {
			value := *timestamp
			lastTimestamp = &value
		}
		line := parsedLine{
			Number:    summary.Lines,
			Timestamp: timestamp,
			Level:     detectLineLevel(content),
			Content:   trimLogContent(content),
		}
		line.Causes = detectRelatedCauses(line)

		active = appendLineToActive(active, line, &summary)
		recent = trimRecentLines(recent)

		if rule, matches, ok := matchConfiguredRule(content, compiled); ok {
			for _, matched := range matches {
				level := resultLevel(rule.Rule.Level)
				result := ParseResult{
					Level:       level,
					MatchedText: matched,
					LineNumber:  line.Number,
					Content:     line.Content,
					RuleID:      rule.Rule.ID,
					RuleName:    rule.Rule.Name,
					Category:    rule.Rule.Category,
					EventTime:   cloneTime(timestamp),
				}
				context := append([]parsedLine(nil), recent...)
				context = append(context, line)
				for _, contextLine := range context {
					appendContext(&result, contextLine, contextLine.Number == line.Number)
				}
				active = append(active, &activeResult{Result: result, EndLine: line.Number + analysisContextLineWindow})
			}
		}
		recent = append(recent, line)
	}
	if err := scanner.Err(); err != nil {
		return parseSummary{}, err
	}
	for _, window := range active {
		commitResult(&summary, window.Result)
	}
	return summary, nil
}

func compileRules(rules []ParseRule) []compiledRule {
	sorted := append([]ParseRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i].Priority, sorted[j].Priority
		if left <= 0 {
			left = 100
		}
		if right <= 0 {
			right = 100
		}
		if left == right {
			return sorted[i].ID < sorted[j].ID
		}
		return left < right
	})
	compiled := make([]compiledRule, 0, len(sorted))
	for _, rule := range sorted {
		if !rule.Enabled {
			continue
		}
		alternatives := make([]string, 0)
		for _, item := range strings.Split(rule.Keyword, "|") {
			if value := strings.TrimSpace(item); value != "" {
				alternatives = append(alternatives, strings.ToLower(value))
			}
		}
		if len(alternatives) > 0 {
			compiled = append(compiled, compiledRule{Rule: rule, Alternatives: alternatives})
		}
	}
	return compiled
}

func matchConfiguredRule(content string, rules []compiledRule) (compiledRule, []string, bool) {
	lower := strings.ToLower(content)
	for _, rule := range rules {
		candidates := make([]configuredMatch, 0)
		labels := strings.Split(rule.Rule.Keyword, "|")
		for index, keyword := range rule.Alternatives {
			label := strings.TrimSpace(labels[index])
			for offset := 0; offset < len(lower); {
				position := strings.Index(lower[offset:], keyword)
				if position < 0 {
					break
				}
				start := offset + position
				end := start + len(keyword)
				candidates = append(candidates, configuredMatch{Text: label, Start: start, End: end})
				offset = end
			}
		}
		if len(candidates) == 0 {
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Start == candidates[j].Start {
				return candidates[i].End > candidates[j].End
			}
			return candidates[i].Start < candidates[j].Start
		})
		matches := make([]string, 0, len(candidates))
		lastEnd := -1
		for _, candidate := range candidates {
			if candidate.Start < lastEnd {
				continue
			}
			matches = append(matches, candidate.Text)
			lastEnd = candidate.End
		}
		return rule, matches, true
	}
	return compiledRule{}, nil, false
}

func appendLineToActive(active []*activeResult, line parsedLine, summary *parseSummary) []*activeResult {
	remaining := active[:0]
	for _, window := range active {
		if line.Number > window.EndLine {
			commitResult(summary, window.Result)
			continue
		}
		appendContext(&window.Result, line, line.Number == window.Result.LineNumber)
		remaining = append(remaining, window)
	}
	return remaining
}

func appendContext(result *ParseResult, line parsedLine, isHit bool) {
	contextLine := ContextLine{
		LineNumber: line.Number,
		Timestamp:  cloneTime(line.Timestamp),
		Level:      line.Level,
		Content:    line.Content,
		IsHit:      isHit,
	}
	result.ContextLines = append(result.ContextLines, contextLine)
	if isHit {
		return
	}
	for _, cause := range line.Causes {
		if hasCause(result.RelatedCauses, cause.Kind, cause.LineNumber) {
			continue
		}
		result.RelatedCauses = append(result.RelatedCauses, cause)
	}
}

func detectRelatedCauses(line parsedLine) []RelatedCause {
	lower := strings.ToLower(line.Content)
	if !couldContainRelatedCause(lower) {
		return nil
	}
	causes := make([]RelatedCause, 0, 1)
	for _, definition := range causeDefinitions {
		if !definition.Pattern.MatchString(line.Content) {
			continue
		}
		causes = append(causes, RelatedCause{
			Kind:       definition.Kind,
			Label:      definition.Label,
			Reason:     definition.Reason,
			Confidence: definition.Confidence,
			LineNumber: line.Number,
			Timestamp:  cloneTime(line.Timestamp),
			Content:    line.Content,
		})
	}
	return causes
}

func couldContainRelatedCause(content string) bool {
	keywords := [...]string{
		"sd remov", "card ", "isinserted", "i/o error", "input/output error", "fat read failed",
		"unable to read inode", "dirty bit", "fallocate", "pool_alloc", "no sdcard",
		"storage sd 0", "power_id_swrt", "watchdog", "2f0050080", "backtrace",
		"log_signal_data", "panic", "assert",
	}
	for _, keyword := range keywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

func hasCause(causes []RelatedCause, kind string, lineNumber int64) bool {
	for _, cause := range causes {
		if cause.Kind == kind && cause.LineNumber == lineNumber {
			return true
		}
	}
	return false
}

func commitResult(summary *parseSummary, result ParseResult) {
	for _, line := range result.ContextLines {
		if line.Timestamp != nil {
			result.ContextStartTime = cloneTime(line.Timestamp)
			break
		}
	}
	for index := len(result.ContextLines) - 1; index >= 0; index-- {
		if result.ContextLines[index].Timestamp != nil {
			result.ContextEndTime = cloneTime(result.ContextLines[index].Timestamp)
			break
		}
	}
	summary.Results = append(summary.Results, result)
	switch result.Level {
	case "error":
		summary.Errors++
	case "warning":
		summary.Warnings++
	}
}

func trimRecentLines(lines []parsedLine) []parsedLine {
	if int64(len(lines)) > analysisContextLineWindow {
		return lines[len(lines)-int(analysisContextLineWindow):]
	}
	return lines
}

func parseTimestamp(content string, previous *time.Time, now time.Time) *time.Time {
	if values := fullTimestampPattern.FindStringSubmatch(content); len(values) == 7 {
		month, _ := strconv.Atoi(values[1])
		day, _ := strconv.Atoi(values[2])
		hour, _ := strconv.Atoi(values[3])
		minute, _ := strconv.Atoi(values[4])
		second, _ := strconv.Atoi(values[5])
		millisecond, _ := strconv.Atoi(values[6])
		value := time.Date(now.Year(), time.Month(month), day, hour, minute, second, millisecond*int(time.Millisecond), now.Location())
		return &value
	}
	if values := shortTimestampPattern.FindStringSubmatch(content); len(values) == 5 {
		date := now
		if previous != nil {
			date = *previous
		}
		hour, _ := strconv.Atoi(values[1])
		minute, _ := strconv.Atoi(values[2])
		second, _ := strconv.Atoi(values[3])
		millisecond, _ := strconv.Atoi(values[4])
		value := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, millisecond*int(time.Millisecond), date.Location())
		return &value
	}
	return cloneTime(previous)
}

func detectLineLevel(content string) string {
	upper := strings.ToUpper(content)
	switch {
	case strings.Contains(upper, "FATAL") || strings.Contains(upper, "ERROR"):
		return "error"
	case strings.Contains(upper, "WARNING") || strings.Contains(upper, "WARN"):
		return "warning"
	case strings.Contains(upper, "INFO"):
		return "info"
	default:
		return ""
	}
}

func resultLevel(level string) string {
	switch level {
	case "critical":
		return "error"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func trimLogContent(content string) string {
	if len(content) > 4000 {
		return content[:4000]
	}
	return content
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func formatEventTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func debugResult(result ParseResult) string {
	return fmt.Sprintf("%s:%d %s %s", result.RuleName, result.LineNumber, formatEventTime(result.EventTime), result.Content)
}
