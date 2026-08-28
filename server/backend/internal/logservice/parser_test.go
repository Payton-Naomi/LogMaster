package logservice

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLogDoesNotTreatGenericLevelsAsFindings(t *testing.T) {
	input := "INFO boot complete\nERROR disk full\nwarning temperature high\nFATAL recorder crashed\n"
	summary, err := parseLog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Lines != 4 || summary.Errors != 0 || summary.Warnings != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Results) != 0 {
		t.Fatalf("got %d results", len(summary.Results))
	}
}

func TestParseLogWithRulesBuildsFiftyLineContextAndCauses(t *testing.T) {
	input := strings.Join([]string{
		"[07/22 10:16:04:900 INFO recorder]:[File Start] video.mp4",
		"[07/22 10:16:12:235 INFO storage]:movie monitor",
		"______________sd removing___________",
		"3,495,65896862029,-;blk_update_request: I/O error, dev mmcblk0",
		"3,497,65896862426,-;FAT-fs (mmcblk0p1): FAT read failed",
		"[07/22 10:16:13:000 INFO XA_MP4_Write-749]:write no sdcard",
		"[07/22 10:16:13:000 ERROR xa_rec_runner_thread-662]:XA_MP4_Write failed",
		"[07/22 10:16:13:018 INFO StorageMng_SD_State_Cb-527]:sd_state : 2",
		"[07/22 10:16:24:001 INFO recorder]:included by line context",
	}, "\n")
	rules := []ParseRule{{ID: 1, Name: "MP4 写入失败", Category: "recording", Keyword: "XA_MP4_Write failed", Level: "critical", Enabled: true, Priority: 5}}
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.Local)
	summary, err := parseLogWithRules(strings.NewReader(input), rules, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Errors != 1 || len(summary.Results) != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	result := summary.Results[0]
	if result.LineNumber != 7 || result.EventTime == nil || result.EventTime.Hour() != 10 || result.EventTime.Second() != 13 {
		t.Fatalf("unexpected event: %+v", result)
	}
	if len(result.ContextLines) != 9 {
		t.Fatalf("context lines=%d, want 9", len(result.ContextLines))
	}
	kinds := map[string]bool{}
	for _, cause := range result.RelatedCauses {
		kinds[cause.Kind] = true
	}
	for _, kind := range []string{"sd_removed", "block_io", "fat_read", "no_sdcard"} {
		if !kinds[kind] {
			t.Fatalf("missing cause %s: %+v", kind, result.RelatedCauses)
		}
	}
}

func TestParseLogWithRulesRespectsEnabledAndPriority(t *testing.T) {
	rules := []ParseRule{
		{ID: 1, Name: "disabled", Keyword: "XA_MP4_Write failed", Level: "critical", Enabled: false, Priority: 1},
		{ID: 2, Name: "specific", Category: "recording", Keyword: "XA_MP4_Write failed", Level: "critical", Enabled: true, Priority: 5},
		{ID: 3, Name: "generic", Category: "system", Keyword: "ERROR", Level: "critical", Enabled: true, Priority: 900},
	}
	input := "[07/22 10:16:13:000 ERROR recorder]:XA_MP4_Write failed\n"
	summary, err := parseLogWithRules(strings.NewReader(input), rules, time.Date(2026, 7, 23, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 1 || summary.Results[0].RuleName != "specific" {
		t.Fatalf("unexpected result: %+v", summary.Results)
	}
}

func TestParseLogWithoutTimestampsKeepsContextBounded(t *testing.T) {
	const lineCount = 600
	lines := make([]string, lineCount)
	for i := range lines {
		lines[i] = "ERROR repeated failure"
	}
	rules := []ParseRule{{ID: 1, Name: "failure", Keyword: "ERROR", Level: "critical", Enabled: true}}
	summary, err := parseLogWithRules(strings.NewReader(strings.Join(lines, "\n")), rules, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != lineCount {
		t.Fatalf("results=%d, want %d", len(summary.Results), lineCount)
	}
	for _, result := range summary.Results {
		if len(result.ContextLines) > int(analysisContextLineWindow*2+1) {
			t.Fatalf("line %d context=%d exceeds fallback window", result.LineNumber, len(result.ContextLines))
		}
	}
}

func TestParseLogWithDenseTimestampsKeepsContextBounded(t *testing.T) {
	var input strings.Builder
	for index := 0; index < 500; index++ {
		input.WriteString("[07/24 16:15:30:123] INFO normal\n")
	}
	input.WriteString("[07/24 16:15:30:123] ERROR failed\n")
	rules := []ParseRule{{ID: 1, Name: "failure", Keyword: "ERROR", Level: "critical", Enabled: true}}

	summary, err := parseLogWithRules(strings.NewReader(input.String()), rules, time.Date(2026, 7, 24, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 1 {
		t.Fatalf("results = %d", len(summary.Results))
	}
	if got := len(summary.Results[0].ContextLines); got > int(analysisContextLineWindow)+1 {
		t.Fatalf("context lines = %d, want at most %d", got, analysisContextLineWindow+1)
	}
}

func TestFrameLossRuleMatchesEveryKeywordOccurrence(t *testing.T) {
	rule := ParseRule{ID: 10, Name: "30 秒窗口累计丢帧", Keyword: "SD write detected frame loss for", Level: "critical", Enabled: true, Priority: 5}
	input := strings.Join([]string{
		"[07/22 10:00:00:000 ERROR sd]:SD write detected frame loss for 6000ms",
		"[07/22 10:00:10:000 ERROR sd]:SD write detected frame loss for 5000ms",
		"[07/22 10:00:20:000 ERROR sd]:SD write detected frame loss for 4000ms",
	}, "\n")
	summary, err := parseLogWithRules(strings.NewReader(input), []ParseRule{rule}, time.Date(2026, 7, 23, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 3 {
		t.Fatalf("unexpected keyword results: %+v", summary.Results)
	}
}

func TestRuleMatchesEveryOccurrenceOnTheSameLine(t *testing.T) {
	rule := ParseRule{ID: 11, Name: "MMB专项检测", Keyword: "MMB", Level: "critical", Enabled: true, Priority: 5}
	summary, err := parseLogWithRules(strings.NewReader("MMB start MMB middle MMB end\n"), []ParseRule{rule}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 3 || summary.Errors != 3 {
		t.Fatalf("results = %d, errors = %d, want 3 and 3", len(summary.Results), summary.Errors)
	}
	for _, result := range summary.Results {
		if result.MatchedText != "MMB" || result.LineNumber != 1 || result.Level != "error" {
			t.Fatalf("unexpected result: %#v", result)
		}
	}
}

func TestKeywordContextContainsFiftyLinesOnEachSide(t *testing.T) {
	lines := make([]string, 121)
	for index := range lines {
		lines[index] = "INFO normal"
	}
	lines[60] = "ERROR keyword hit"
	rules := []ParseRule{{ID: 1, Name: "failure", Keyword: "keyword hit", Level: "critical", Enabled: true}}

	summary, err := parseLogWithRules(strings.NewReader(strings.Join(lines, "\n")), rules, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) != 1 {
		t.Fatalf("results = %d", len(summary.Results))
	}
	context := summary.Results[0].ContextLines
	if len(context) != 101 || context[0].LineNumber != 11 || context[len(context)-1].LineNumber != 111 {
		t.Fatalf("context range = %d-%d (%d lines)", context[0].LineNumber, context[len(context)-1].LineNumber, len(context))
	}
}

func TestSafeTargetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../secret.log", "folder/../../secret.log", "C:/secret.log"} {
		if _, _, err := safeTarget(root, name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestSafeTargetNormalizesRootedDevicePath(t *testing.T) {
	root := t.TempDir()
	target, relative, err := safeTarget(root, "/logfile_0")
	if err != nil {
		t.Fatal(err)
	}
	if relative != "logfile_0" || target != filepath.Join(root, "logfile_0") {
		t.Fatalf("target=%q relative=%q", target, relative)
	}
}
