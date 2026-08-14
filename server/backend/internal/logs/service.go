package logs

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"logmaster-agent/internal/config"
)

type Service struct {
	config              config.Config
	repo                *Repository
	agent               AgentAnalyzer
	notifier            AnalysisNotifier
	currentUserResolver func(*http.Request) (string, bool)
	uploadToken         string
	uploadOwnerOpenID   string
}

const (
	builtinUploadToken       = "logmaster-internal-collector-v1"
	builtinUploadOwnerOpenID = "logmaster-internal-collector"
)

func NewService(cfg config.Config, repo *Repository) *Service {
	service := &Service{config: cfg, repo: repo}
	if cfg.AgentAnalysisURL != "" {
		service.agent = NewHTTPAgentAnalyzer(cfg.AgentAnalysisURL, cfg.AgentAnalysisToken, cfg.AgentAnalysisTimeout)
	}
	service.startStaleTaskMonitor()
	return service
}

func NewServiceWithAgent(cfg config.Config, repo *Repository, analyzer AgentAnalyzer) *Service {
	return &Service{config: cfg, repo: repo, agent: analyzer}
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
		if s.agent != nil {
			file.LineCount = summary.Lines
			result, agentErr := s.agent.Analyze(ctx, AgentAnalysisRequest{
				TaskID: taskID, UploadID: uploadID, File: file, TotalLines: summary.Lines, Matches: summary.Results,
			})
			if err := s.repo.SaveAgentAnalysis(ctx, taskID, file.ID, s.agent.Provider(), result, agentErr); err != nil {
				log.Printf("save agent analysis for %s: %v", file.RelativePath, err)
			}
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
