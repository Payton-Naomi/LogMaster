package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"logmaster-agent/agent/internal/config"
	"logmaster-agent/agent/internal/logfile"
	"logmaster-agent/agent/internal/pipeline"
	"logmaster-agent/agent/internal/store"
	"logmaster-agent/agent/internal/upload"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to YAML configuration")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logger, closeLog, err := newLogger(cfg.Storage.LogDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeLog()
	storage, err := store.Open(cfg.Storage.SQLitePath)
	if err != nil {
		logger.Error("open storage", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app := pipeline.New(cfg, storage, logfile.New(cfg.Storage.LogDir), upload.NewHTTP(cfg.Upload.URL, cfg.Upload.Token, cfg.Upload.Timeout), logger)
	logger.Info("agent starting", "agent_id", cfg.AgentID, "devices", len(cfg.Devices))
	if err := app.Run(ctx); err != nil {
		logger.Error("agent stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("agent stopped")
}

func newLogger(dir string) (*slog.Logger, func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "agent-runtime.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	writer := io.MultiWriter(os.Stdout, file)
	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})), func() { _ = file.Close() }, nil
}
