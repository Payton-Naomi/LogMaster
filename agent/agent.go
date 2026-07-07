package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"logmaster-agent/agent/ai"
	"logmaster-agent/agent/config"
	"logmaster-agent/agent/rule"
	serialagent "logmaster-agent/agent/serial"
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
	a.collector.Stop()
	return nil
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

	return output
}