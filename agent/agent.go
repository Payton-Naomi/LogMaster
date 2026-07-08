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

// OutputLine 是每条日志行的 JSON 输出结构。
type OutputLine struct {
	Device      string        `json:"device"`
	Timestamp   string        `json:"timestamp"`
	Content     string        `json:"content"`
	Severity    string        `json:"severity"`
	Category    string        `json:"category"`
	RuleName    string        `json:"rule_name,omitempty"`
	AI          *ai.Diagnosis `json:"ai,omitempty"`
}

// Agent 协调串口采集、规则匹配和 AI 分析整个流程。
type Agent struct {
	cfg       *config.Config
	collector *serialagent.Collector
	engine    *rule.Engine
	analyzer  *ai.Analyzer
	uploader  *uploader.Uploader
}

// New 根据配置创建一个新的 Agent。
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

// Run 启动 Agent 管道，阻塞直到关闭。
func (a *Agent) Run() error {
	// 为每个设备启动采集器
	for _, dev := range a.cfg.Devices {
		if err := a.collector.Start(dev.Name, dev.BaudRate); err != nil {
			return fmt.Errorf("start collector for %s: %w", dev.Name, err)
		}
		fmt.Fprintf(os.Stderr, "Started collecting from %s at %d baud\n", dev.Name, dev.BaudRate)
	}

	// 处理优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// 后台 goroutine：定时上传刷新
	stopUpload := make(chan struct{})
	go a.uploadLoop(stopUpload)

	// 处理日志行
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

	// 刷新剩余的上传队列
	if err := a.uploader.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Final upload flush failed: %v\n", err)
	}
	return nil
}

// uploadLoop 定期刷新上传队列。
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

// processLine 对日志行应用规则匹配和 AI 分析。
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

	// 对 ERROR 和 WARN 级别触发 AI 分析
	if result.Severity == "ERROR" || result.Severity == "WARN" {
		diag, err := a.analyzer.Analyze(line.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AI analysis error: %v\n", err)
		} else {
			output.AI = diag
		}
	}

	// 入队用于 HTTP 上传
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