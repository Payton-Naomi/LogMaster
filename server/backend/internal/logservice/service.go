package logservice

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"logmaster-agent/internal/config"
	"logmaster-agent/internal/securevalue"
)

type Service struct {
	config              config.Config
	repo                *Repository
	agent               AgentAnalyzer
	agentJobs           chan agentJob
	agentWg             sync.WaitGroup
	notifier            AnalysisNotifier
	currentUserResolver func(*http.Request) (string, bool)
	uploadToken         string
	uploadOwnerOpenID   string
	directory           *feishuDirectory
	dynamicLLM          bool
}

const (
	builtinUploadToken       = "logmaster-internal-collector-v1"
	builtinUploadOwnerOpenID = "logmaster-internal-collector"
)

func NewService(cfg config.Config, repo *Repository) *Service {
	service := &Service{config: cfg, repo: repo}
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		service.directory = newFeishuDirectory(cfg.FeishuAppID, cfg.FeishuAppSecret)
	}
	switch {
	case cfg.LLMAPIBaseURL != "" || cfg.AgentAnalysisURL == "":
		service.agent = NewLLMAnalyzer(cfg.LLMAPIBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMTimeout, cfg.LLMMaxMatches, cfg.LLMMaxInputBytes)
		service.dynamicLLM = true
	case cfg.AgentAnalysisURL != "":
		service.agent = NewHTTPAgentAnalyzer(cfg.AgentAnalysisURL, cfg.AgentAnalysisToken, cfg.AgentAnalysisTimeout)
	}
	if service.agent != nil {
		service.startAgentWorkers()
	}
	service.startStaleTaskMonitor()
	return service
}

func NewServiceWithAgent(cfg config.Config, repo *Repository, analyzer AgentAnalyzer) *Service {
	service := &Service{config: cfg, repo: repo, agent: analyzer}
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		service.directory = newFeishuDirectory(cfg.FeishuAppID, cfg.FeishuAppSecret)
	}
	if service.agent != nil {
		service.startAgentWorkers()
	}
	return service
}

type agentJob struct {
	taskID      string
	uploadID    string
	ownerOpenID string
	file        LogFile
	totalLines  int64
	matches     []ParseResult
}

func (s *Service) startAgentWorkers() {
	s.agentJobs = make(chan agentJob, 64)
	s.agentWg.Add(1)
	go s.runAgentWorker()
}

func (s *Service) runAgentWorker() {
	defer s.agentWg.Done()
	for job := range s.agentJobs {
		s.processAgentJob(job)
	}
}

func (s *Service) processAgentJob(job agentJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	settings, settingsErr := s.repo.AIAnalysisSettings(ctx, s.fallbackAISettings())
	if settingsErr != nil {
		log.Printf("load AI settings for %s: %v", job.file.RelativePath, settingsErr)
		return
	}
	analyzer := s.agent
	if s.dynamicLLM {
		apiKey := s.config.LLMAPIKey
		if settings.LLMAPIKeyEncrypted != "" {
			apiKey, settingsErr = securevalue.Decrypt(settings.LLMAPIKeyEncrypted, s.config.ConfigEncryptionKey)
			if settingsErr != nil {
				log.Printf("decrypt LLM API key for %s: %v", job.file.RelativePath, settingsErr)
				return
			}
		}
		analyzer = NewLLMAnalyzer(settings.LLMAPIBaseURL, apiKey, settings.LLMModel,
			time.Duration(settings.LLMTimeoutSeconds)*time.Second, settings.LLMMaxMatches, settings.LLMMaxInputBytes)
	}
	if job.ownerOpenID != "" {
		if settings.DailyTokenQuota > 0 {
			used, usageErr := s.repo.UserDailyTokenUsage(ctx, job.ownerOpenID)
			if usageErr == nil && used >= settings.DailyTokenQuota {
				quotaErr := fmt.Errorf("daily AI token quota exceeded (%d/%d)", used, settings.DailyTokenQuota)
				if err := s.repo.SaveAgentAnalysis(ctx, job.taskID, job.file.ID, analyzer.Provider(), AgentAnalysisResponse{}, quotaErr); err != nil {
					log.Printf("save agent analysis for %s: %v", job.file.RelativePath, err)
				}
				return
			}
		}
	}

	request := AgentAnalysisRequest{
		TaskID: job.taskID, UploadID: job.uploadID, File: job.file,
		TotalLines: job.totalLines, Matches: job.matches,
	}
	var result AgentAnalysisResponse
	var agentErr error
	if limited, ok := analyzer.(TokenLimitedAnalyzer); ok {
		result, agentErr = limited.AnalyzeWithTokenLimit(ctx, request, settings.MaxTokensPerFile)
	} else {
		result, agentErr = analyzer.Analyze(ctx, request)
	}

	if reporter, ok := analyzer.(TokenUsageReporter); ok && agentErr == nil {
		prompt, completion := reporter.LastUsage()
		if prompt > 0 || completion > 0 {
			if err := s.repo.RecordAIUsage(ctx, job.ownerOpenID, job.taskID, job.file.ID, prompt, completion); err != nil {
				log.Printf("record AI usage for %s: %v", job.file.RelativePath, err)
			}
		}
	}

	if err := s.repo.SaveAgentAnalysis(ctx, job.taskID, job.file.ID, analyzer.Provider(), result, agentErr); err != nil {
		log.Printf("save agent analysis for %s: %v", job.file.RelativePath, err)
	}
}

func (s *Service) fallbackAISettings() AIAnalysisSettings {
	return AIAnalysisSettings{
		LLMAPIBaseURL: s.config.LLMAPIBaseURL, LLMModel: s.config.LLMModel,
		LLMTimeoutSeconds: int(s.config.LLMTimeout / time.Second), LLMMaxMatches: s.config.LLMMaxMatches,
		LLMMaxInputBytes: s.config.LLMMaxInputBytes, MaxTokensPerFile: s.config.AIMaxTokensPerFile,
		DailyTokenQuota: s.config.AIDailyTokenQuota,
	}
}

func (s *Service) analysisEnabled(ctx context.Context) bool {
	if s.agent == nil {
		return false
	}
	if !s.dynamicLLM {
		return true
	}
	settings, err := s.repo.AIAnalysisSettings(ctx, s.fallbackAISettings())
	return err == nil && strings.TrimSpace(settings.LLMAPIBaseURL) != ""
}

func (s *Service) enqueueAgentAnalysis(job agentJob) {
	select {
	case s.agentJobs <- job:
	default:
		log.Printf("agent analysis queue full, dropping job for %s", job.file.RelativePath)
	}
}

func (s *Service) SetAnalysisNotifier(notifier AnalysisNotifier) {
	s.notifier = notifier
}

func (s *Service) SetCurrentUserResolver(resolver func(*http.Request) (string, bool)) {
	s.currentUserResolver = resolver
}

func (s *Service) SetUploadAuthenticator(token, ownerOpenID string) {
	s.uploadToken = strings.TrimSpace(token)
	s.uploadOwnerOpenID = strings.TrimSpace(ownerOpenID)
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/upload-requests", s.uploadRequestHandler)
	mux.HandleFunc("/api/upload-sessions", s.uploadSessionHandler)
	mux.HandleFunc("/api/upload-sessions/", s.completeUploadSessionHandler)
	mux.HandleFunc("/api/upload-config", s.uploadConfigHandler)
	mux.HandleFunc("/api/keywords/sync", s.keywordSyncHandler)
	mux.HandleFunc("/api/logs/upload", s.uploadHandler)
	mux.HandleFunc("/api/query/", s.queryHandler)
	mux.HandleFunc("/api/logs/inspect", s.inspectHandler)
	mux.HandleFunc("/api/logs", s.listUploadsHandler)
	mux.HandleFunc("/api/logs/", s.logDetailHandler)
	mux.HandleFunc("/api/tasks", s.listTasksHandler)
	mux.HandleFunc("/api/tasks/", s.taskHandler)
	mux.HandleFunc("/api/dashboard/stats", s.dashboardHandler)
	mux.HandleFunc("/api/projects", s.projectsHandler)
	mux.HandleFunc("/api/rules", s.rulesHandler)
	mux.HandleFunc("/api/rules/batch", s.ruleBatchHandler)
	mux.HandleFunc("/api/rules/", s.ruleHandler)
	mux.HandleFunc("/api/scenarios", s.scenariosHandler)
	mux.HandleFunc("/api/scenarios/", s.scenarioHandler)
}

func (s *Service) startStaleTaskMonitor() {
	if s.repo == nil {
		return
	}
	check := func() {
		const message = "解析任务长时间无进度，后台进程可能已中断，请重新上传解析"
		if err := s.repo.FailStaleTasks(context.Background(), message, time.Now().Add(-5*time.Minute)); err != nil {
			log.Printf("mark stale parsing tasks failed: %v", err)
		}
	}
	check()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			check()
		}
	}()
}

func (s *Service) keepTaskAlive(uploadID string, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.repo.TouchTask(context.Background(), uploadID); err != nil {
				log.Printf("update task heartbeat %s: %v", uploadID, err)
			}
		case <-done:
			return
		}
	}
}

type parsingProgressReader struct {
	reader     io.Reader
	base       int64
	read       int64
	lastReport time.Time
	report     func(int64)
}

const parsingProgressInterval = 2 * time.Second

func (r *parsingProgressReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.read += int64(n)
	if err == io.EOF || time.Since(r.lastReport) >= parsingProgressInterval {
		r.report(r.base + r.read)
		r.lastReport = time.Now()
	}
	return n, err
}

func (s *Service) processUpload(uploadID string) {
	ctx := context.Background()
	fail := func(event, message string) {
		s.repo.MarkFailed(ctx, uploadID, message)
		s.repo.RecordUploadRuntimeLog(ctx, uploadID, "parsing", event, "failed", message)
	}
	storagePath, err := s.repo.UploadStoragePath(ctx, uploadID)
	if err != nil {
		fail("resolve upload storage", "resolve upload storage failed")
		return
	}
	ownerOpenID, err := s.repo.UploadOwner(ctx, uploadID)
	if err != nil || ownerOpenID == "" {
		fail("resolve upload owner", "resolve upload owner failed")
		return
	}
	taskID, files, err := s.repo.StartParsing(ctx, uploadID)
	if err != nil {
		fail("start parsing", err.Error())
		return
	}
	rules, err := s.repo.RulesForUpload(ctx, uploadID, ownerOpenID)
	if err != nil {
		fail("load parsing rules", fmt.Sprintf("load parsing rules: %v", err))
		return
	}
	var processedBytes int64
	for _, file := range files {
		path := filepath.Join(storagePath, filepath.FromSlash(file.RelativePath))
		input, err := os.Open(path)
		if err != nil {
			fail("open log file", fmt.Sprintf("open %s: %v", file.RelativePath, err))
			return
		}
		progressInput := &parsingProgressReader{
			reader: input,
			base:   processedBytes,
			report: func(current int64) {
				if err := s.repo.UpdateParsingProgress(ctx, taskID, current); err != nil {
					log.Printf("update parsing progress %s: %v", taskID, err)
				}
			},
		}
		summary, parseErr := parseLogWithRules(progressInput, rules, time.Now())
		input.Close()
		if parseErr != nil {
			fail("parse log file", fmt.Sprintf("parse %s: %v", file.RelativePath, parseErr))
			return
		}
		if err := s.repo.SaveFileResults(ctx, taskID, file.ID, summary.Lines, summary.Errors, summary.Warnings, summary.Results); err != nil {
			fail("save parsing result", err.Error())
			return
		}
		processedBytes += file.SizeBytes
		if err := s.repo.UpdateParsingProgress(ctx, taskID, processedBytes); err != nil {
			log.Printf("finalize parsing progress %s: %v", taskID, err)
		}
		if s.analysisEnabled(ctx) {
			file.LineCount = summary.Lines
			s.enqueueAgentAnalysis(agentJob{
				taskID: taskID, uploadID: uploadID, ownerOpenID: ownerOpenID, file: file,
				totalLines: summary.Lines, matches: summary.Results,
			})
		}
	}
	if err := s.repo.CompleteParsing(ctx, uploadID); err != nil {
		log.Printf("complete log parsing %s: %v", uploadID, err)
		s.repo.RecordUploadRuntimeLog(ctx, uploadID, "parsing", "complete parsing", "failed", err.Error())
		return
	}
	s.repo.RecordUploadRuntimeLog(ctx, uploadID, "parsing", "complete parsing", "success", fmt.Sprintf("parsed %d log files", len(files)))
	s.sendAnalysisNotification(uploadID)
}

func (s *Service) sendAnalysisNotification(uploadID string) {
	if s.notifier == nil {
		return
	}
	notification, err := s.repo.AnalysisNotification(context.Background(), uploadID)
	if err != nil {
		log.Printf("load analysis notification %s: %v", uploadID, err)
		return
	}
	if notification.RecipientOpenID == "" {
		log.Printf("skip analysis notification %s: uploader has no Feishu OpenID", uploadID)
		return
	}
	if s.config.PublicBaseURL != "" {
		notification.ResultURL = s.config.PublicBaseURL + "/analysis/" + notification.TaskID
	}
	go func() {
		delays := []time.Duration{0, 2 * time.Second, 10 * time.Second}
		var notifyErr error
		for _, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			notifyErr = s.notifier.Notify(ctx, notification)
			cancel()
			if notifyErr == nil {
				return
			}
		}
		log.Printf("send analysis notification for task %s failed: %v", notification.TaskID, notifyErr)
	}()
}
