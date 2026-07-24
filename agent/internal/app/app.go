package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"logmaster-agent/agent/internal/analyzer"
	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/config"
	"logmaster-agent/agent/internal/observability"
	"logmaster-agent/agent/internal/segment"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

const Version = "0.1.0-mvp"

type App struct {
	cfg     config.Config
	store   *spool.Store
	worker  *backend.Worker
	server  *http.Server
	metrics *observability.Registry
	logger  *slog.Logger
	demo    bool
}

func New(cfg config.Config, store *spool.Store, logger *slog.Logger, demo bool) (*App, error) {
	token := ""
	if cfg.Agent.AnalysisTokenEnv != "" {
		token = os.Getenv(cfg.Agent.AnalysisTokenEnv)
	}
	analysisCfg := analyzer.DefaultConfig()
	analysisCfg.Path = cfg.Agent.AnalysisPath
	analysisCfg.Token = token
	analysisCfg.MaxRequestBytes = cfg.Agent.MaxRequestBytes
	analysisCfg.Timeout = cfg.AI.Timeout
	analysisCfg.MaxConcurrent = cfg.Agent.AnalysisConcurrency
	analysisCfg.MaxFindings = cfg.AI.MaxFindings
	analysisCfg.Mode = analyzer.Mode(cfg.AI.Mode)
	analysisCfg.Ollama = analyzer.ModelConfig{BaseURL: cfg.AI.OllamaURL, Model: cfg.AI.OllamaModel}
	analysisCfg.Qwen = analyzer.ModelConfig{BaseURL: cfg.AI.QwenBaseURL, Model: cfg.AI.QwenModel}
	if cfg.AI.QwenAPIKeyEnv != "" {
		analysisCfg.Qwen.APIKey = os.Getenv(cfg.AI.QwenAPIKeyEnv)
	}
	analysisService, err := analyzer.New(analysisCfg, &analysisCache{store: store})
	if err != nil {
		return nil, fmt.Errorf("configure analyzer: %w", err)
	}
	client := backend.New(backend.Config{
		BaseURL: cfg.Backend.BaseURL, HealthPath: cfg.Backend.HealthPath,
		InspectPath: cfg.Backend.InspectPath, UploadPath: cfg.Backend.UploadPath,
		Timeout: cfg.Backend.RequestTimeout,
	})
	worker := backend.NewWorker(backend.WorkerConfig{Interval: 2 * time.Second, MaxFiles: 16, InspectBeforeUpload: cfg.Backend.InspectBeforeUpload}, store, client, logger)
	metrics := observability.NewRegistry(cfg.Agent.ID, Version)
	mux := http.NewServeMux()
	analysisService.Register(mux)
	mux.Handle("/metrics", metrics.Handler())
	a := &App{cfg: cfg, store: store, worker: worker, metrics: metrics, logger: logger, demo: demo}
	mux.HandleFunc("/healthz", a.health)
	a.server = &http.Server{Addr: cfg.Agent.Listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: cfg.AI.Timeout + 5*time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		healthCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Health(healthCtx); err != nil {
			logger.Warn("backend unavailable; collection will continue locally", "component", "backend.health", "error", err)
		} else {
			logger.Info("backend health check passed", "component", "backend.health")
		}
	}()
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	serverErr := make(chan error, 1)
	go func() {
		a.logger.Info("Agent HTTP server started", "listen", a.cfg.Agent.Listen, "analysis_path", a.cfg.Agent.AnalysisPath)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	go a.worker.Run(ctx)

	var wg sync.WaitGroup
	for _, port := range a.cfg.Serial.Ports {
		if !port.Enabled {
			continue
		}
		port := port
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.runPort(ctx, port)
		}()
	}
	if a.demo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.runDemo(ctx)
		}()
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serverErr:
	}
	cancel()
	wg.Wait()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := a.server.Shutdown(shutdownCtx); runErr == nil && err != nil {
		runErr = err
	}
	return runErr
}

func (a *App) runPort(ctx context.Context, port config.PortConfig) {
	writer, decoder, err := a.pipelineForPort(port.DeviceSN, port.PortName, port.Encoding, port.SegmentMaxAge, port.SegmentMaxBytes)
	if err != nil {
		a.logger.Error("initialize serial pipeline", "device_sn", port.DeviceSN, "error", err)
		return
	}
	defer writer.Close(context.WithoutCancel(ctx))
	serialCfg := serialagent.SerialConfig{
		PortName: port.PortName, BaudRate: port.BaudRate, DataBits: port.DataBits, StopBits: port.StopBits,
		Parity: serialagent.Parity(port.Parity), Handshake: serialagent.Handshake(port.Handshake), DTR: port.DTR, RTS: port.RTS,
		ReadTimeout: port.ReadTimeout, WriteTimeout: port.WriteTimeout, IdleGap: port.IdleGap, MaxFrameBytes: port.MaxFrameBytes, Encoding: serialagent.Encoding(port.Encoding),
	}
	reconnect := serialagent.NewReconnectManager(serialagent.ReconnectConfig{InitialDelay: a.cfg.Serial.Reconnect.InitialDelay, Multiplier: a.cfg.Serial.Reconnect.Multiplier, MaxDelay: a.cfg.Serial.Reconnect.MaxDelay, Jitter: a.cfg.Serial.Reconnect.Jitter, StableReset: 60 * time.Second})
	for ctx.Err() == nil {
		started := time.Now()
		session, err := serialagent.NewSession(serialCfg, serialagent.GoBugFactory{}, func(frameCtx context.Context, frame serialagent.Frame) error {
			a.metrics.AddSerialBytes(port.DeviceSN, uint64(len(frame.Data)))
			for _, line := range decoder.Push(frame.Data, frame.CapturedAt) {
				if err := a.writeLine(frameCtx, writer, port.DeviceSN, line); err != nil {
					return err
				}
			}
			return nil
		})
		if err == nil {
			a.metrics.SetSerialConnected(port.DeviceSN, port.PortName, true)
			err = session.Run(ctx)
			a.metrics.SetSerialConnected(port.DeviceSN, port.PortName, false)
		}
		if ctx.Err() != nil {
			break
		}
		delay := reconnect.FailureDelay(time.Since(started))
		a.metrics.IncReconnect(port.DeviceSN, "read_or_open")
		a.logger.Warn("serial disconnected; retrying", "device_sn", port.DeviceSN, "port_name", port.PortName, "retry_in", delay, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
	if line, ok := decoder.Flush(time.Now().UTC()); ok {
		_ = a.writeLine(context.WithoutCancel(ctx), writer, port.DeviceSN, line)
	}
}

func (a *App) runDemo(ctx context.Context) {
	port := config.DefaultPortConfig()
	port.DeviceSN, port.PortName = "DEMO-DVR-0001", "DEMO"
	port.SegmentMaxAge = 3 * time.Second
	port.SegmentMaxBytes = 4096
	writer, _, err := a.pipelineForPort(port.DeviceSN, port.PortName, port.Encoding, port.SegmentMaxAge, port.SegmentMaxBytes)
	if err != nil {
		a.logger.Error("initialize demo pipeline", "error", err)
		return
	}
	defer writer.Close(context.WithoutCancel(ctx))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lines := []string{"INFO recorder started", "WARN storage latency high", "ERROR camera initialization failed", "INFO gps synchronized"}
	index := 0
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			line := serialagent.DecodedLine{CapturedAt: now.UTC(), Text: lines[index%len(lines)]}
			if err := a.writeLine(ctx, writer, port.DeviceSN, line); err != nil {
				a.logger.Error("write demo log", "error", err)
				return
			}
			index++
		}
	}
}

func (a *App) pipelineForPort(deviceSN, portName, encoding string, maxAge time.Duration, maxBytes int64) (*segment.Writer, *serialagent.Decoder, error) {
	decoder, err := serialagent.NewDecoder(serialagent.Encoding(encoding))
	if err != nil {
		return nil, nil, err
	}
	writer, err := segment.NewWriter(segment.Config{Directory: a.cfg.Spool.Directory, DeviceSN: deviceSN, PortName: portName, SessionStart: time.Now().UTC(), MaxAge: maxAge, MaxBytes: maxBytes}, func(ctx context.Context, completed segment.Completed) error {
		_, err := a.store.EnqueueFile(ctx, a.cfg.Backend.ProjectName, a.cfg.Backend.Version, spool.File{Path: completed.Path, SHA256: completed.SHA256, SizeBytes: completed.SizeBytes, DeviceSN: completed.DeviceSN, FirstSequence: completed.FirstSequence, LastSequence: completed.LastSequence})
		if err == nil {
			a.logger.Info("segment queued", "device_sn", completed.DeviceSN, "path", completed.Path, "bytes", completed.SizeBytes)
		}
		return err
	})
	return writer, decoder, err
}

func (a *App) writeLine(ctx context.Context, writer *segment.Writer, deviceSN string, line serialagent.DecodedLine) error {
	sequence, err := a.store.NextSequence(ctx, deviceSN)
	if err != nil {
		return err
	}
	return writer.Write(ctx, segment.Entry{Sequence: sequence, CapturedAt: line.CapturedAt, Text: line.Text})
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	counts, err := a.store.Counts(context.Background())
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "version": Version, "upload_paused": a.worker.Paused(), "spool": counts})
}

type analysisCache struct{ store *spool.Store }

func (c *analysisCache) Get(ctx context.Context, key string) (analyzer.AnalysisResponse, bool, error) {
	raw, ok, err := c.store.Get(ctx, key)
	if err != nil || !ok {
		return analyzer.AnalysisResponse{}, ok, err
	}
	var response analyzer.AnalysisResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return analyzer.AnalysisResponse{}, false, err
	}
	return response, true, nil
}

func (c *analysisCache) Set(ctx context.Context, key string, response analyzer.AnalysisResponse, expiresAt time.Time) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return c.store.Put(ctx, key, raw, time.Until(expiresAt))
}
