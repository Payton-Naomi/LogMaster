package logservice

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const taskOverviewMaxTokens = 4000

// processTaskOverviewJob runs behind all file-level jobs for the same upload.
// It only forwards the completed diagnoses to the LLM, never the source files.
func (s *Service) processTaskOverviewJob(job agentJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	current, err := s.repo.IsAgentExecutionAllowed(ctx, job.taskID, job.attemptNo)
	if err != nil {
		log.Printf("validate parse attempt for task overview %s: %v", job.taskID, err)
		return
	}
	if !current {
		return
	}

	upload, files, err := s.repo.GetUploadByTask(ctx, job.taskID, job.ownerOpenID)
	if err != nil {
		log.Printf("load task overview input %s: %v", job.taskID, err)
		return
	}
	records, err := s.repo.AgentResults(ctx, job.taskID, job.ownerOpenID)
	if err != nil {
		log.Printf("load file AI results for task overview %s: %v", job.taskID, err)
		return
	}
	request := TaskOverviewRequest{
		TaskID: job.taskID, ProjectName: upload.ProjectName, Version: upload.Version,
		TotalFiles: upload.TotalFiles, TotalLines: upload.TotalLines,
		ErrorCount: upload.ErrorCount, WarningCount: upload.WarningCount,
		Files: make([]TaskOverviewFile, 0, len(records)),
	}
	if request.TotalFiles <= 0 {
		request.TotalFiles = len(files)
	}
	for _, record := range records {
		if record.Status != "completed" {
			continue
		}
		request.Files = append(request.Files, TaskOverviewFile{
			FilePath: record.FilePath, Status: record.Status,
			Summary: record.Summary, Findings: record.Findings,
		})
	}
	if len(request.Files) == 0 {
		err := fmt.Errorf("no completed file-level AI analyses")
		if saveErr := s.repo.SaveTaskOverview(ctx, job.taskID, job.attemptNo, "llm", TaskOverview{}, err); saveErr != nil {
			log.Printf("save empty task overview %s: %v", job.taskID, saveErr)
		}
		return
	}

	settings, err := s.repo.AIAnalysisSettings(ctx, s.fallbackAISettings())
	if err != nil {
		log.Printf("load AI settings for task overview %s: %v", job.taskID, err)
		return
	}
	if settings.DailyTokenQuota > 0 && job.ownerOpenID != "" {
		used, usageErr := s.repo.UserDailyTokenUsage(ctx, job.ownerOpenID)
		if usageErr != nil || used >= settings.DailyTokenQuota {
			if usageErr == nil {
				usageErr = fmt.Errorf("daily AI token quota exceeded (%d/%d)", used, settings.DailyTokenQuota)
			}
			if saveErr := s.repo.SaveTaskOverview(ctx, job.taskID, job.attemptNo, "llm", TaskOverview{}, usageErr); saveErr != nil {
				log.Printf("save task overview quota error %s: %v", job.taskID, saveErr)
			}
			return
		}
	}

	analyzer, provider, err := s.taskOverviewAnalyzer(settings)
	if err != nil {
		if saveErr := s.repo.SaveTaskOverview(ctx, job.taskID, job.attemptNo, provider, TaskOverview{}, err); saveErr != nil {
			log.Printf("save task overview configuration error %s: %v", job.taskID, saveErr)
		}
		return
	}
	maxTokens := settings.MaxTokensPerFile
	if maxTokens <= 0 || maxTokens > taskOverviewMaxTokens {
		maxTokens = taskOverviewMaxTokens
	}
	stopCancellationWatch := s.watchAgentCancellation(ctx, cancel, job.taskID, job.attemptNo)
	overview, analysisErr := analyzer.SummarizeTask(ctx, request, maxTokens)
	stopCancellationWatch()
	if analysisErr != nil {
		log.Printf("generate task AI overview %s: %v", job.taskID, analysisErr)
	}
	if err := s.repo.SaveTaskOverview(ctx, job.taskID, job.attemptNo, provider, overview, analysisErr); err != nil {
		log.Printf("save task AI overview %s: %v", job.taskID, err)
	}
	if analysisErr == nil && job.ownerOpenID != "" {
		promptTokens, completionTokens := analyzer.LastUsage()
		if promptTokens > 0 || completionTokens > 0 {
			if err := s.repo.RecordAITaskUsage(ctx, job.ownerOpenID, job.taskID, promptTokens, completionTokens); err != nil {
				log.Printf("record task AI usage for %s: %v", job.taskID, err)
			}
		}
	}
}

func (s *Service) taskOverviewAnalyzer(settings AIAnalysisSettings) (*LLMAnalyzer, string, error) {
	if !s.dynamicLLM {
		return nil, "http-agent", fmt.Errorf("task overview requires the built-in LLM analyzer")
	}
	if strings.TrimSpace(settings.LLMAPIBaseURL) == "" {
		return nil, "llm", fmt.Errorf("AI analysis is not configured")
	}
	timeout := time.Duration(settings.LLMTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = s.config.LLMTimeout
	}
	return NewLLMAnalyzer(settings.LLMAPIBaseURL, s.config.LLMAPIKey, settings.LLMModel, timeout,
		settings.LLMMaxMatches, settings.LLMMaxInputBytes), "llm", nil
}
