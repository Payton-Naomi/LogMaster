package logservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"logmaster-agent/internal/config"
)

type Service struct {
	config              config.Config
	repo                *Repository
	agent               AgentAnalyzer
	agentQueueWake      chan struct{}
	agentWg             sync.WaitGroup
	agentWorkerID       string
	parseQueueWake      chan struct{}
	parseWg             sync.WaitGroup
	parseWorkerID       string
	notifier            AnalysisNotifier
	currentUserResolver func(*http.Request) (string, bool)
	uploadToken         string
	uploadOwnerOpenID   string
	directory           *feishuDirectory
	dynamicLLM          bool
	searchCache         *logSearchCache
}

const (
	builtinUploadToken       = "logmaster-internal-collector-v1"
	builtinUploadOwnerOpenID = "logmaster-internal-collector"
)

func NewService(cfg config.Config, repo *Repository) *Service {
	service := &Service{config: cfg, repo: repo, searchCache: newLogSearchCache()}
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
	if cfg.MaxParseWorkers > 0 {
		service.startParseWorkers()
		service.startStaleTaskMonitor()
	}
	return service
}

func NewServiceWithAgent(cfg config.Config, repo *Repository, analyzer AgentAnalyzer) *Service {
	service := &Service{config: cfg, repo: repo, agent: analyzer, searchCache: newLogSearchCache()}
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		service.directory = newFeishuDirectory(cfg.FeishuAppID, cfg.FeishuAppSecret)
	}
	if service.agent != nil {
		service.startAgentWorkers()
	}
	if cfg.MaxParseWorkers > 0 {
		service.startParseWorkers()
		service.startStaleTaskMonitor()
	}
	return service
}

type agentJob struct {
	taskID          string
	uploadID        string
	ownerOpenID     string
	attemptNo       int
	file            LogFile
	totalLines      int64
	matches         []ParseResult
	overview        bool
	refreshOverview bool
	queueID         int64
	runToken        string
}

func (s *Service) startAgentWorkers() {
	if s.repo == nil {
		return
	}
	s.agentQueueWake = make(chan struct{}, 1)
	s.agentWorkerID = fmt.Sprintf("ai-%d-%s", os.Getpid(), newID())
	if err := s.repo.ReconcileAIQueue(context.Background()); err != nil {
		log.Printf("reconcile AI queue failed: %v", err)
	}
	workerCount := s.config.MaxAIWorkers
	if workerCount < 1 {
		workerCount = 1
	}
	for index := 0; index < workerCount; index++ {
		s.agentWg.Add(1)
		go s.runAgentWorker(fmt.Sprintf("%s-%d", s.agentWorkerID, index+1))
	}
}

func (s *Service) startParseWorkers() {
	if s.repo == nil {
		return
	}
	workerCount := s.config.MaxParseWorkers
	if workerCount < 1 {
		workerCount = 1
	}
	maxAttempts := s.config.MaxParseAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	s.parseQueueWake = make(chan struct{}, 1)
	s.parseWorkerID = fmt.Sprintf("backend-%d-%s", os.Getpid(), newID())
	if err := s.repo.ReconcileParseQueue(context.Background(), maxAttempts); err != nil {
		log.Printf("reconcile parse queue failed: %v", err)
	}
	for index := 0; index < workerCount; index++ {
		s.parseWg.Add(1)
		go s.runParseWorker(fmt.Sprintf("%s-%d", s.parseWorkerID, index+1), maxAttempts)
	}
}

func (s *Service) signalParseQueue() {
	if s.parseQueueWake == nil {
		return
	}
	select {
	case s.parseQueueWake <- struct{}{}:
	default:
	}
}

func (s *Service) runParseWorker(workerID string, maxAttempts int) {
	defer s.parseWg.Done()
	for {
		job, err := s.repo.ClaimParseTask(context.Background(), workerID, maxAttempts, s.config.MaxParsePerUser, s.config.MaxParsePerProject)
		if err == nil {
			s.executeParseTask(job)
			continue
		}
		if err != sql.ErrNoRows {
			log.Printf("claim parse task failed: %v", err)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		if s.parseQueueWake != nil {
			select {
			case <-s.parseQueueWake:
				if !timer.Stop() {
					<-timer.C
				}
				continue
			case <-timer.C:
			}
		} else {
			<-timer.C
		}
	}
}

func (s *Service) executeParseTask(task ClaimedParseTask) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("parse worker panic for task %s: %v", task.TaskID, recovered)
		}
	}()
	s.processParseTask(task)
}

func (s *Service) processParseTask(task ClaimedParseTask) {
	done := make(chan struct{})
	go s.keepParseTaskAlive(task.TaskID, task.RunToken, done)
	defer close(done)
	if task.Phase == "prepare" {
		s.prepareParseTask(task)
		return
	}
	s.processUpload(task)
}

func (s *Service) prepareParseTask(task ClaimedParseTask) {
	ctx := context.Background()
	items, err := storedUploadItems(task.StoragePath)
	if err != nil {
		s.failClaimedTask(task, "locate uploaded files", err.Error())
		return
	}
	var logFiles []LogFile
	var extractedSize int64
	var totalParseBytes int64
	archivePasswords, passwordErr := s.repo.ArchivePasswords(ctx)
	if passwordErr != nil {
		// Keep existing deployments usable before migration 033 is applied.
		log.Printf("load archive passwords failed: %v", passwordErr)
	}
	for _, item := range items {
		cancelled, cancelErr := s.repo.IsParseTaskStopped(ctx, task.TaskID)
		if cancelErr != nil {
			s.failClaimedTask(task, "check task cancellation", cancelErr.Error())
			return
		}
		if cancelled {
			return
		}
		if err := os.RemoveAll(filepath.Join(item.itemRoot, "extracted")); err != nil {
			s.failClaimedTask(task, "clean incomplete archive extraction", err.Error())
			return
		}
		files, extractErr := collectLogFilesWithPasswords(item.storedPath, item.itemRoot, s.config.MaxExtractBytes-extractedSize, archivePasswords)
		if extractErr != nil {
			s.failClaimedTask(task, "extract uploaded archive", extractErr.Error())
			return
		}
		for i := range files {
			totalParseBytes += files[i].SizeBytes
			path := filepath.ToSlash(files[i].RelativePath)
			if strings.Contains(path, "/extracted/") || strings.HasPrefix(path, "extracted/") {
				extractedSize += files[i].SizeBytes
			}
			files[i].RelativePath = filepath.ToSlash(filepath.Join("items", strconv.Itoa(item.index), filepath.FromSlash(files[i].RelativePath)))
		}
		logFiles = append(logFiles, files...)
		if len(logFiles) > s.config.MaxFilesPerParseTask {
			s.failClaimedTask(task, "validate parse task capacity", fmt.Sprintf("解析任务文件数量超过限制，最多允许 %d 个文件", s.config.MaxFilesPerParseTask))
			return
		}
		if totalParseBytes > s.config.MaxBytesPerParseTask {
			s.failClaimedTask(task, "validate parse task capacity", fmt.Sprintf("解析任务总大小超过限制，最多允许 %d 字节", s.config.MaxBytesPerParseTask))
			return
		}
	}
	if cancelled, cancelErr := s.repo.IsParseTaskStopped(ctx, task.TaskID); cancelErr != nil {
		s.failClaimedTask(task, "check task cancellation", cancelErr.Error())
		return
	} else if cancelled {
		return
	}
	if err := s.repo.QueuePreparedUpload(ctx, task, logFiles); err != nil {
		s.failClaimedTask(task, "queue prepared upload", err.Error())
		return
	}
	s.repo.RecordUploadRuntimeLog(ctx, task.UploadID, "archive", "prepare uploaded files", "success", fmt.Sprintf("prepared %d log files", len(logFiles)))
	task.Phase = "parse"
	s.processUpload(task)
}

func storedUploadItems(storagePath string) ([]storedUploadItem, error) {
	entries, err := os.ReadDir(filepath.Join(storagePath, "items"))
	if err != nil {
		return nil, fmt.Errorf("read uploaded items: %w", err)
	}
	items := make([]storedUploadItem, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		index, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || index < 1 {
			continue
		}
		itemRoot := filepath.Join(storagePath, "items", entry.Name())
		originalDir := filepath.Join(itemRoot, "original")
		originals, readErr := os.ReadDir(originalDir)
		if readErr != nil {
			return nil, fmt.Errorf("read uploaded item %d: %w", index, readErr)
		}
		found := false
		for _, original := range originals {
			if original.IsDir() {
				continue
			}
			items = append(items, storedUploadItem{index: index, itemRoot: itemRoot, storedPath: filepath.Join(originalDir, original.Name())})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("uploaded item %d contains no source file", index)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("uploaded task contains no source files")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].index < items[j].index })
	return items, nil
}

func (s *Service) runAgentWorker(workerID string) {
	defer s.agentWg.Done()
	lastReconcile := time.Now()
	for {
		job, err := s.repo.ClaimAgentJob(context.Background(), workerID)
		if err == nil {
			done := make(chan struct{})
			go s.keepAgentJobAlive(job.queueID, job.runToken, done)
			s.processAgentJob(job)
			close(done)
			if err := s.repo.FinalizeAgentJob(context.Background(), job.queueID, job.runToken); err != nil && !errors.Is(err, sql.ErrNoRows) {
				log.Printf("finalize AI job %d: %v", job.queueID, err)
			}
			if err := s.repo.CreatePendingAINotifications(context.Background(), job.taskID); err != nil {
				log.Printf("create AI completion notification: %v", err)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("claim AI job failed: %v", err)
		}
		if time.Since(lastReconcile) >= 5*time.Second {
			if reconcileErr := s.repo.ReconcileAIQueue(context.Background()); reconcileErr != nil {
				log.Printf("reconcile AI queue failed: %v", reconcileErr)
			}
			if notifyErr := s.repo.CreatePendingAINotifications(context.Background(), ""); notifyErr != nil {
				log.Printf("create recovered AI notification: %v", notifyErr)
			}
			lastReconcile = time.Now()
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-s.agentQueueWake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

func (s *Service) keepAgentJobAlive(jobID int64, runToken string, done <-chan struct{}) {
	ticker := time.NewTicker(agentJobLeaseDuration / 3)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.repo.RenewAgentJobLease(context.Background(), jobID, runToken); err != nil {
				if !errors.Is(err, ErrParseTaskLeaseLost) {
					log.Printf("renew AI job lease %d: %v", jobID, err)
				}
				return
			}
		case <-done:
			return
		}
	}
}

func (s *Service) processAgentJob(job agentJob) {
	if job.overview {
		s.processTaskOverviewJob(job)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	current, err := s.repo.IsAgentExecutionAllowed(ctx, job.taskID, job.attemptNo)
	if err != nil {
		log.Printf("validate parse attempt for %s: %v", job.file.RelativePath, err)
		return
	}
	if !current {
		return
	}
	refreshQueued := false
	if job.refreshOverview {
		defer func() {
			if !refreshQueued {
				_ = s.repo.ClearAgentRetryRequested(context.Background(), job.taskID, job.attemptNo)
			}
		}()
	}

	settings, settingsErr := s.repo.AIAnalysisSettings(ctx, s.fallbackAISettings())
	if settingsErr != nil {
		log.Printf("load AI settings for %s: %v", job.file.RelativePath, settingsErr)
		return
	}
	analyzer := s.agent
	if s.dynamicLLM {
		analyzer = NewLLMAnalyzer(settings.LLMAPIBaseURL, s.config.LLMAPIKey, settings.LLMModel,
			time.Duration(settings.LLMTimeoutSeconds)*time.Second, settings.LLMMaxMatches, settings.LLMMaxInputBytes)
	}
	if job.ownerOpenID != "" {
		if settings.DailyTokenQuota > 0 {
			used, usageErr := s.repo.UserDailyTokenUsage(ctx, job.ownerOpenID)
			if usageErr == nil && used >= settings.DailyTokenQuota {
				quotaErr := fmt.Errorf("daily AI token quota exceeded (%d/%d)", used, settings.DailyTokenQuota)
				saveErr := s.repo.SaveAgentAnalysis(ctx, job.taskID, job.attemptNo, job.file.ID, analyzer.Provider(), AgentAnalysisResponse{}, quotaErr)
				if saveErr != nil {
					log.Printf("save agent analysis for %s: %v", job.file.RelativePath, saveErr)
				}
				if job.refreshOverview && saveErr == nil {
					refreshQueued = s.finishAgentFileRetry(job)
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
	stopCancellationWatch := s.watchAgentCancellation(ctx, cancel, job.taskID, job.attemptNo)
	if limited, ok := analyzer.(TokenLimitedAnalyzer); ok {
		result, agentErr = limited.AnalyzeWithTokenLimit(ctx, request, settings.MaxTokensPerFile)
	} else {
		result, agentErr = analyzer.Analyze(ctx, request)
	}
	stopCancellationWatch()

	if reporter, ok := analyzer.(TokenUsageReporter); ok && agentErr == nil {
		prompt, completion := reporter.LastUsage()
		if prompt > 0 || completion > 0 {
			if err := s.repo.RecordAIUsage(ctx, job.ownerOpenID, job.taskID, job.file.ID, prompt, completion); err != nil {
				log.Printf("record AI usage for %s: %v", job.file.RelativePath, err)
			}
		}
	}

	saveErr := s.repo.SaveAgentAnalysis(ctx, job.taskID, job.attemptNo, job.file.ID, analyzer.Provider(), result, agentErr)
	if saveErr != nil {
		log.Printf("save agent analysis for %s: %v", job.file.RelativePath, saveErr)
	}
	if job.refreshOverview && saveErr == nil {
		refreshQueued = s.finishAgentFileRetry(job)
	}
}

func (s *Service) finishAgentFileRetry(job agentJob) bool {
	if !s.dynamicLLM {
		_ = s.repo.ClearAgentRetryRequested(context.Background(), job.taskID, job.attemptNo)
		return true
	}
	queued := s.enqueueAgentAnalysis(agentJob{
		taskID: job.taskID, uploadID: job.uploadID, ownerOpenID: job.ownerOpenID,
		attemptNo: job.attemptNo, overview: true,
	})
	if !queued {
		_ = s.repo.ClearAgentRetryRequested(context.Background(), job.taskID, job.attemptNo)
	}
	return queued
}

func (s *Service) watchAgentCancellation(ctx context.Context, cancel context.CancelFunc, taskID string, attemptNo int) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, stopCheck := context.WithTimeout(context.Background(), 2*time.Second)
				allowed, err := s.repo.IsAgentExecutionAllowed(checkCtx, taskID, attemptNo)
				stopCheck()
				if err == nil && !allowed {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done) }
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

func (s *Service) enqueueAgentAnalysis(job agentJob) bool {
	if err := s.repo.EnqueueAgentJob(context.Background(), job); err != nil {
		if job.overview {
			log.Printf("enqueue task AI overview %s: %v", job.taskID, err)
		} else {
			log.Printf("enqueue AI analysis for %s: %v", job.file.RelativePath, err)
		}
		return false
	}
	select {
	case s.agentQueueWake <- struct{}{}:
	default:
	}
	return true
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
	mux.HandleFunc("/api/projects/sync", s.collectorProjectsSyncHandler)
	mux.HandleFunc("/api/scenarios/sync", s.collectorScenariosSyncHandler)
	mux.HandleFunc("/api/collector/sync", s.collectorSyncHandler)
	mux.HandleFunc("/api/logs/upload", s.uploadHandler)
	mux.HandleFunc("/api/query/", s.queryHandler)
	mux.HandleFunc("/api/logs/inspect", s.inspectHandler)
	mux.HandleFunc("/api/logs", s.listUploadsHandler)
	mux.HandleFunc("/api/logs/", s.logDetailHandler)
	mux.HandleFunc("/api/results/", s.resultHandler)
	mux.HandleFunc("/api/analysis/compare", s.compareHandler)
	mux.HandleFunc("/api/notifications", s.notificationsHandler)
	mux.HandleFunc("/api/notifications/", s.notificationsHandler)
	mux.HandleFunc("/api/notification-settings", s.notificationSettingsHandler)
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
		if err := s.repo.ReconcileParseQueue(context.Background(), max(1, s.config.MaxParseAttempts)); err != nil {
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

func (s *Service) keepParseTaskAlive(taskID, runToken string, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.repo.RenewParseTaskLease(context.Background(), taskID, runToken); err != nil {
				if err != ErrParseTaskLeaseLost {
					log.Printf("update task heartbeat %s: %v", taskID, err)
				}
				return
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

func (s *Service) processUpload(task ClaimedParseTask) {
	ctx := context.Background()
	fail := func(event, message string) {
		s.failClaimedTask(task, event, message)
	}
	storagePath := task.StoragePath
	ownerOpenID, err := s.repo.UploadOwner(ctx, task.UploadID)
	if err != nil || ownerOpenID == "" {
		fail("resolve upload owner", "resolve upload owner failed")
		return
	}
	taskID, files, err := s.repo.BeginClaimedParsing(ctx, task)
	if err != nil {
		fail("start parsing", err.Error())
		return
	}
	rules, err := s.repo.RulesForUpload(ctx, task.UploadID, ownerOpenID)
	if err != nil {
		fail("load parsing rules", fmt.Sprintf("load parsing rules: %v", err))
		return
	}
	aiEnabled, err := s.repo.UploadAIAnalysisEnabled(ctx, task.UploadID)
	if err != nil {
		fail("load AI analysis setting", err.Error())
		return
	}
	aiEnabled = aiEnabled && s.analysisEnabled(ctx)
	var processedBytes int64
	allAgentJobsQueued := true
	for _, file := range files {
		cancelled, cancelErr := s.repo.IsParseTaskStopped(ctx, task.TaskID)
		if cancelErr != nil {
			fail("check task cancellation", cancelErr.Error())
			return
		}
		if cancelled {
			return
		}
		path := filepath.Join(storagePath, filepath.FromSlash(file.RelativePath))
		input, err := os.Open(path)
		if err != nil {
			fail("open log file", fmt.Sprintf("open %s: %v", file.RelativePath, err))
			return
		}
		progressCancelled := false
		progressInput := &parsingProgressReader{
			reader: input,
			base:   processedBytes,
			report: func(current int64) {
				if err := s.repo.UpdateClaimedParsingProgress(ctx, taskID, task.RunToken, current); err != nil {
					if err == ErrParseTaskLeaseLost {
						progressCancelled = true
						return
					}
					log.Printf("update parsing progress %s: %v", taskID, err)
				}
			},
		}
		summary, parseErr := parseLogWithRules(progressInput, rules, time.Now())
		input.Close()
		if progressCancelled {
			return
		}
		if parseErr != nil {
			fail("parse log file", fmt.Sprintf("parse %s: %v", file.RelativePath, parseErr))
			return
		}
		if err := s.repo.SaveClaimedFileResults(ctx, task, file.ID, summary.Lines, summary.Errors, summary.Warnings, summary.Results); err != nil {
			fail("save parsing result", err.Error())
			return
		}
		processedBytes += file.SizeBytes
		if err := s.repo.UpdateClaimedParsingProgress(ctx, taskID, task.RunToken, processedBytes); err != nil {
			log.Printf("finalize parsing progress %s: %v", taskID, err)
		}
		if aiEnabled {
			file.LineCount = summary.Lines
			if !s.enqueueAgentAnalysis(agentJob{
				taskID: taskID, uploadID: task.UploadID, ownerOpenID: ownerOpenID, attemptNo: task.AttemptNo, file: file,
				totalLines: summary.Lines, matches: summary.Results,
			}) {
				allAgentJobsQueued = false
			}
		}
	}
	if err := s.repo.CompleteClaimedParsing(ctx, task); err != nil {
		log.Printf("complete log parsing %s: %v", task.UploadID, err)
		s.repo.RecordUploadRuntimeLog(ctx, task.UploadID, "parsing", "complete parsing", "failed", err.Error())
		return
	}
	if err := s.repo.CreateTaskNotifications(ctx, task.TaskID, "task_completed", "日志解析完成", "日志解析任务已完成，可查看分析结果"); err != nil {
		log.Printf("create completion notification %s: %v", task.TaskID, err)
	}
	if s.dynamicLLM && aiEnabled && allAgentJobsQueued {
		s.enqueueAgentAnalysis(agentJob{taskID: taskID, uploadID: task.UploadID, ownerOpenID: ownerOpenID, attemptNo: task.AttemptNo, overview: true})
	}
	s.repo.RecordUploadRuntimeLog(ctx, task.UploadID, "parsing", "complete parsing", "success", fmt.Sprintf("parsed %d log files", len(files)))
	s.sendAnalysisNotification(task.UploadID)
}

func (s *Service) failClaimedTask(task ClaimedParseTask, event, message string) {
	ctx := context.Background()
	message = chineseErrorMessage(message)
	if err := s.repo.FailClaimedParseTask(ctx, task, message); err != nil {
		if err != ErrParseTaskLeaseLost {
			log.Printf("mark parse task failed %s: %v", task.TaskID, err)
		}
		return
	}
	s.repo.RecordUploadRuntimeLog(ctx, task.UploadID, task.Phase, event, "failed", message)
	if err := s.repo.CreateTaskNotifications(ctx, task.TaskID, "task_failed", "日志处理失败", message); err != nil {
		log.Printf("create failure notification %s: %v", task.TaskID, err)
	}
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
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
