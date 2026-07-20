package logs

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"logmaster-agent/internal/config"
)

type Service struct {
	config config.Config
	repo   *Repository
	agent  AgentAnalyzer
}

func NewService(cfg config.Config, repo *Repository) *Service {
	service := &Service{config: cfg, repo: repo}
	if cfg.AgentAnalysisURL != "" {
		service.agent = NewHTTPAgentAnalyzer(cfg.AgentAnalysisURL, cfg.AgentAnalysisToken, cfg.AgentAnalysisTimeout)
	}
	return service
}

func NewServiceWithAgent(cfg config.Config, repo *Repository, analyzer AgentAnalyzer) *Service {
	return &Service{config: cfg, repo: repo, agent: analyzer}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/logs/upload", s.uploadHandler)
	mux.HandleFunc("/api/logs/inspect", s.inspectHandler)
	mux.HandleFunc("/api/logs", s.listUploadsHandler)
	mux.HandleFunc("/api/logs/", s.logDetailHandler)
	mux.HandleFunc("/api/tasks", s.listTasksHandler)
	mux.HandleFunc("/api/tasks/", s.taskHandler)
	mux.HandleFunc("/api/dashboard/stats", s.dashboardHandler)
	mux.HandleFunc("/api/projects", s.projectsHandler)
	mux.HandleFunc("/api/system/com-ports", s.comPortsHandler)
	mux.HandleFunc("/api/rules", s.rulesHandler)
	mux.HandleFunc("/api/rules/", s.ruleHandler)
	mux.HandleFunc("/api/scenarios", s.scenariosHandler)
	mux.HandleFunc("/api/scenarios/", s.scenarioHandler)
}

func (s *Service) processUpload(uploadID string) {
	ctx := context.Background()
	taskID, files, err := s.repo.StartParsing(ctx, uploadID)
	if err != nil {
		s.repo.MarkFailed(ctx, uploadID, err.Error())
		return
	}
	for _, file := range files {
		path := filepath.Join(s.config.StorageDir, uploadID, filepath.FromSlash(file.RelativePath))
		input, err := os.Open(path)
		if err != nil {
			s.repo.MarkFailed(ctx, uploadID, fmt.Sprintf("open %s: %v", file.RelativePath, err))
			return
		}
		summary, parseErr := parseLog(input)
		input.Close()
		if parseErr != nil {
			s.repo.MarkFailed(ctx, uploadID, fmt.Sprintf("parse %s: %v", file.RelativePath, parseErr))
			return
		}
		if err := s.repo.SaveFileResults(ctx, taskID, file.ID, summary.Lines, summary.Errors, summary.Warnings, summary.Results); err != nil {
			s.repo.MarkFailed(ctx, uploadID, err.Error())
			return
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
	}
}
