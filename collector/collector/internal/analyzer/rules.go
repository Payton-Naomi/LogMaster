package analyzer

import (
	"fmt"
	"sort"
	"strings"
)

func analyzeRules(req AnalysisRequest) AnalysisResponse {
	matches := normalizeMatches(req.Matches)
	if len(matches) == 0 {
		return AnalysisResponse{
			Summary:  "本地解析未发现 ERROR、FATAL、WARN 或 WARNING 命中项",
			Findings: []Finding{},
		}
	}

	findings := make([]Finding, 0, len(matches))
	seen := make(map[string]struct{})
	errorsCount := 0
	warningsCount := 0
	for _, match := range matches {
		severity := ruleSeverity(match)
		if severity == "warning" {
			warningsCount++
		} else {
			errorsCount++
		}
		category, cause, suggestion, confidence := classifyRule(match)
		key := category + "\x00" + severity
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		findings = append(findings, Finding{
			Category: category, Severity: severity, RootCause: cause,
			Suggestion: suggestion, Evidence: truncateUTF8(match.Content, MaxMatchContentBytes),
			Confidence: confidence,
		})
	}
	return AnalysisResponse{
		Summary:  fmt.Sprintf("检测到 %d 条异常命中：%d 条错误，%d 条警告", len(matches), errorsCount, warningsCount),
		Findings: findings,
	}
}

func normalizeMatches(matches []Match) []Match {
	result := make([]Match, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		match.Level = strings.ToLower(strings.TrimSpace(match.Level))
		match.MatchedText = strings.TrimSpace(match.MatchedText)
		match.Content = strings.TrimSpace(match.Content)
		key := fmt.Sprintf("%d\x00%s\x00%s", match.LineNumber, match.Level, match.Content)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, match)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].LineNumber < result[j].LineNumber
	})
	return result
}

func ruleSeverity(match Match) string {
	text := strings.ToLower(match.Level + " " + match.MatchedText + " " + match.Content)
	switch {
	case strings.Contains(text, "fatal"), strings.Contains(text, "panic"), strings.Contains(text, "crash"):
		return "critical"
	case strings.Contains(text, "error"), strings.Contains(text, "fail"):
		return "error"
	default:
		return "warning"
	}
}

func classifyRule(match Match) (string, string, string, float64) {
	text := strings.ToLower(match.MatchedText + " " + match.Content)
	switch {
	case containsAny(text, "camera", "摄像头", "lens", "isp"):
		return "camera", "摄像头或图像采集模块报告异常", "检查摄像头连接、驱动状态和初始化顺序", 0.82
	case containsAny(text, "record", "录像", "video", "encode", "编码", "mux"):
		return "recording", "录像或编码链路报告异常", "检查录像服务、编码器状态和输出链路", 0.82
	case containsAny(text, "gps", "gnss", "定位", "satellite"):
		return "gps", "定位模块或卫星信号异常", "检查定位模块连接、天线和授时状态", 0.82
	case containsAny(text, "disk", "storage", "sd card", "sdcard", "filesystem", "存储", "磁盘", "i/o"):
		return "storage", "存储介质或文件系统操作异常", "检查存储介质健康状态、文件系统和剩余空间", 0.82
	case containsAny(text, "sensor", "imu", "gyro", "accelerometer", "传感器", "陀螺仪"):
		return "sensor", "传感器读取或初始化异常", "检查传感器连接、供电和驱动配置", 0.82
	case containsAny(text, "network", "socket", "dns", "http", "tcp", "udp", "网络", "timeout"):
		return "network", "通信链路超时或网络异常", "检查链路状态、地址配置、超时和重试策略", 0.78
	case containsAny(text, "system", "kernel", "memory", "oom", "panic", "crash", "系统", "内存"):
		return "system", "系统资源或进程运行异常", "结合相邻日志检查资源使用、进程状态和崩溃信息", 0.78
	default:
		return "unknown", "日志中出现尚未归类的异常标记", "结合该行前后日志和设备状态进一步排查", 0.55
	}
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
