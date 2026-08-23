package logservice

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type downloadArtifact struct {
	Name string
	Data []byte
}

func (s *Service) buildTaskResultArtifacts(ctx context.Context, taskID, ownerOpenID string) ([]downloadArtifact, error) {
	upload, files, err := s.repo.GetUploadByTask(ctx, taskID, ownerOpenID)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.Results(ctx, taskID, ownerOpenID, 1000000, 0)
	if err != nil {
		return nil, err
	}
	agentResults, err := s.repo.AgentResults(ctx, taskID, ownerOpenID)
	if err != nil {
		return nil, err
	}
	if overview, overviewErr := s.repo.TaskOverview(ctx, taskID, ownerOpenID); overviewErr == nil {
		agentResults = append([]AgentAnalysisRecord{overview.AsAgentAnalysisRecord()}, agentResults...)
	} else if !errors.Is(overviewErr, sql.ErrNoRows) {
		return nil, overviewErr
	}

	var csvBuffer bytes.Buffer
	csvBuffer.Write([]byte{0xEF, 0xBB, 0xBF})
	csvWriter := csv.NewWriter(&csvBuffer)
	_ = csvWriter.Write([]string{"结果ID", "状态", "负责人", "文件", "级别", "规则", "分类", "行号", "命中文本", "日志内容"})
	for _, item := range results {
		_ = csvWriter.Write([]string{strconv.FormatInt(item.ID, 10), item.Status, item.AssignedTo, item.FilePath,
			item.Level, item.RuleName, item.Category, strconv.FormatInt(item.LineNumber, 10), item.MatchedText, item.Content})
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return nil, err
	}

	var jsonBuffer bytes.Buffer
	if err := json.NewEncoder(&jsonBuffer).Encode(map[string]any{
		"task": upload, "files": files, "results": results, "agent_results": agentResults, "exported_at": time.Now(),
	}); err != nil {
		return nil, err
	}

	var report bytes.Buffer
	fmt.Fprintf(&report, "# LogMaster 任务分析报告\n\n- 任务 ID：%s\n- 项目：%s\n- 版本：%s\n- 解析状态：%s\n- AI 状态：%s\n- 文件数：%d\n- 错误数：%d\n- 警告数：%d\n- 导出时间：%s\n\n",
		taskID, upload.ProjectName, upload.Version, upload.Status, upload.AIStatus, len(files), upload.ErrorCount,
		upload.WarningCount, time.Now().Format(time.RFC3339))
	if overview, overviewErr := s.repo.TaskOverview(ctx, taskID, ownerOpenID); overviewErr == nil {
		fmt.Fprintf(&report, "## AI 总体结论\n\n%s\n\n风险等级：%s\n\n", overview.Summary, overview.RiskLevel)
	}
	fmt.Fprint(&report, "## 异常明细\n\n")
	for _, item := range results {
		fmt.Fprintf(&report, "- `%s:%d` [%s] %s：%s\n", item.FilePath, item.LineNumber, item.Level, item.RuleName, item.MatchedText)
	}

	return []downloadArtifact{
		{Name: "results.csv", Data: csvBuffer.Bytes()},
		{Name: "data.json", Data: jsonBuffer.Bytes()},
		{Name: "report.md", Data: report.Bytes()},
	}, nil
}
