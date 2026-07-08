package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"logmaster-agent/agent/ai"
	"logmaster-agent/agent/config"
	"logmaster-agent/agent/rule"
	serialagent "logmaster-agent/agent/serial"
	"logmaster-agent/agent/uploader"
)

// OutputLine is the JSON output structure for each log line.
type OutputLine struct {
	Device      string        `json:"device"`
	Timestamp   string        `json:"timestamp"`
	Content     string        `json:"content"`
	Severity    string        `json:"severity"`
	Category    string        `json:"category"`
	RuleName    string        `json:"rule_name,omitempty"`
	AI          *ai.Diagnosis `json:"ai,omitempty"`
}

// Agent orchestrates serial collection, rule matching, and AI analysis.
type Agent struct {
	cfg       *config.Config
	collector *serialagent.Collector
	engine    *rule.Engine
	analyzer  *ai.Analyzer
	uploader  *uploader.Uploader
}

// New creates a new Agent from configuration.
func New(cfg *config.Config) *Agent {
	rules := make([]rule.Rule, len(cfg.Rules))
	for i, rc := range cfg.Rules {
		rules[i] = rule.Rule{
			Name:     rc.Name,
			Keywords: rc.Keywords,
			Pattern:  rc.Pattern,
			Severity: rc.Severity,
			Category: rc.Category,
		}
	}

	ollamaClient := ai.NewOllamaClient(cfg.Ollama.Endpoint, cfg.Ollama.Model)

	return &Agent{
		cfg:       cfg,
		collector: serialagent.NewCollector(),
		engine:    rule.NewEngine(rules),
		analyzer:  ai.NewAnalyzer(ollamaClient, cfg.Ollama.Model),
		uploader:  uploader.New(cfg.Upload.Endpoint, cfg.Upload.APIKey, cfg.Upload.Interval, cfg.Upload.BatchSize),
	}
}

// Run starts the agent pipeline and blocks until shutdown.
func (a *Agent) Run() error {
	// Start collectors for each device
	for _, dev := range a.cfg.Devices {
		if err := a.collector.Start(dev.Name, dev.BaudRate); err != nil {
			return fmt.Errorf("start collector for %s: %w", dev.Name, err)
		}
		fmt.Fprintf(os.Stderr, "Started collecting from %s at %d baud\n", dev.Name, dev.BaudRate)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Background goroutine: periodic upload flush
	stopUpload := make(chan struct{})
	go a.uploadLoop(stopUpload)

	// Process log lines
	go func() {
		for line := range a.collector.Lines() {
			output := a.processLine(line)
			data, _ := json.Marshal(output)
			fmt.Println(string(data))
		}
	}()

	<-sigCh
	fmt.Fprintln(os.Stderr, "\nShutting down...")
	close(stopUpload)
	a.collector.Stop()

	// Flush remaining upload queue
	if err := a.uploader.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Final upload flush failed: %v\n", err)
	}
	return nil
}

// uploadLoop periodically flushes the upload queue.
func (a *Agent) uploadLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(a.cfg.Upload.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := a.uploader.Flush(); err != nil {
				fmt.Fprintf(os.Stderr, "Upload flush failed: %v\n", err)
			}
		}
	}
}

// processLine applies rule matching and AI analysis to a log line.
func (a *Agent) processLine(line serialagent.LogLine) OutputLine {
	result := a.engine.Match(line.Content)
	output := OutputLine{
		Device:    line.Device,
		Timestamp: line.Timestamp.Format("2006-01-02T15:04:05.000"),
		Content:   line.Content,
		Severity:  result.Severity,
		Category:  result.Category,
		RuleName:  result.RuleName,
	}

	// Trigger AI analysis for ERROR and WARN severity
	if result.Severity == "ERROR" || result.Severity == "WARN" {
		diag, err := a.analyzer.Analyze(line.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AI analysis error: %v\n", err)
		} else {
			output.AI = diag
		}
	}

	// Enqueue for HTTP upload
	a.uploader.Enqueue(uploader.LogEntry{
		Device:    output.Device,
		Timestamp: output.Timestamp,
		Content:   output.Content,
		Severity:  output.Severity,
		Category:  output.Category,
		RuleName:  output.RuleName,
	})

	return output
}