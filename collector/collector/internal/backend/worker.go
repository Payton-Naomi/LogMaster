package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"logmaster-agent/agent/internal/spool"
)

type WorkerConfig struct {
	Interval            time.Duration
	MaxFiles            int
	Concurrency         int
	InspectBeforeUpload bool
	MaxAttempts         int
	BatchInterval       time.Duration
}

type Worker struct {
	cfg      WorkerConfig
	cfgMu    sync.RWMutex
	store    *spool.Store
	client   *Client
	logger   *slog.Logger
	paused   atomic.Bool
	progress func(UploadProgress)
}

type UploadProgress struct {
	BatchID      string
	SentBytes    int64
	TotalBytes   int64
	SpeedBytes   int64
	StartedAt    time.Time
	AttemptCount int
}

func NewWorker(cfg WorkerConfig, store *spool.Store, client *Client, logger *slog.Logger) *Worker {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 16
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.BatchInterval <= 0 {
		cfg.BatchInterval = 200 * time.Millisecond
	}
	return &Worker{cfg: cfg, store: store, client: client, logger: logger}
}

func (w *Worker) Paused() bool { return w.paused.Load() }

func (w *Worker) SetProgressHandler(handler func(UploadProgress)) { w.progress = handler }

// ApplyConfig hot-updates values used by the upload loop. Concurrency is
// applied to newly started workers; interval and client settings apply on the
// next loop tick without restarting the desktop application.
func (w *Worker) ApplyConfig(cfg WorkerConfig) {
	w.cfgMu.Lock()
	defer w.cfgMu.Unlock()
	w.cfg.Interval = cfg.Interval
	if cfg.MaxFiles > 0 {
		w.cfg.MaxFiles = cfg.MaxFiles
	}
	if cfg.Concurrency > 0 {
		w.cfg.Concurrency = cfg.Concurrency
	}
	if cfg.BatchInterval > 0 {
		w.cfg.BatchInterval = cfg.BatchInterval
	}
	w.cfg.InspectBeforeUpload = cfg.InspectBeforeUpload
}

func (w *Worker) config() WorkerConfig { w.cfgMu.RLock(); defer w.cfgMu.RUnlock(); return w.cfg }

func (w *Worker) Concurrency() int { return w.config().Concurrency }

func (w *Worker) Run(ctx context.Context) {
	const maxDynamicConcurrency = 16
	done := make(chan struct{}, maxDynamicConcurrency)
	for i := 0; i < maxDynamicConcurrency; i++ {
		go func(index int) {
			w.runLoop(ctx, index)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < maxDynamicConcurrency; i++ {
		<-done
	}
}

func (w *Worker) runLoop(ctx context.Context, index int) {
	for {
		cfg := w.config()
		if index >= cfg.Concurrency {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}
		if !w.paused.Load() {
			for i := 0; i < 100; i++ {
				processed, err := w.ProcessOne(ctx)
				if err != nil {
					w.logger.Error("upload worker failed", "component", "backend.upload", "error", err)
					break
				}
				if !processed {
					break
				}
				// 错峰：批次之间留一个小间隔，避免多并发时瞬间连发打崩后端。
				time.Sleep(cfg.BatchInterval)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Interval):
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	cfg := w.config()
	batch, err := w.store.ClaimReady(ctx, cfg.MaxFiles)
	if err != nil || batch == nil {
		return false, err
	}
	for _, file := range batch.Files {
		if err := spool.VerifyFile(file); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if file.LogFileID != "" {
					if removeErr := w.store.RemoveMissingLogFile(ctx, file.LogFileID); removeErr != nil {
						w.logger.Warn("missing local log cleanup failed", "component", "backend.upload", "batch_id", batch.ID, "log_file_id", file.LogFileID, "error", removeErr)
					}
				}
				w.logger.Warn("local log file missing; upload batch stopped", "component", "backend.upload", "batch_id", batch.ID, "path", file.Path)
				return true, w.store.MarkDead(ctx, batch.ID, "本地文件已不存在，记录已清理")
			}
			return true, w.store.MarkDead(ctx, batch.ID, "spool file invalid: "+err.Error())
		}
		if cfg.InspectBeforeUpload {
			if err := w.client.Inspect(ctx, file); err != nil {
				return true, w.handleFailure(ctx, *batch, err)
			}
		}
	}
	started := time.Now().UTC()
	lastAt := started
	var lastSent int64
	accepted, err := w.client.UploadWithProgress(ctx, *batch, func(sent, total int64) {
		now := time.Now().UTC()
		elapsed := now.Sub(lastAt).Seconds()
		speed := int64(0)
		if elapsed > 0 {
			speed = int64(float64(sent-lastSent) / elapsed)
		}
		lastAt = now
		lastSent = sent
		if w.progress != nil {
			w.progress(UploadProgress{BatchID: batch.ID, SentBytes: sent, TotalBytes: total, SpeedBytes: speed, StartedAt: started, AttemptCount: batch.AttemptCount})
		}
	})
	if err != nil {
		return true, w.handleFailure(ctx, *batch, err)
	}
	if err := w.store.MarkUploaded(ctx, batch.ID, accepted.UploadID, accepted.TaskID); err != nil {
		return true, fmt.Errorf("persist upload acknowledgement: %w", err)
	}
	if accepted.QueryCode != "" {
		if err := w.store.SetQueryCode(ctx, batch.ID, accepted.QueryCode); err != nil {
			return true, fmt.Errorf("persist upload query code: %w", err)
		}
	}
	w.logger.Info("batch uploaded", "component", "backend.upload", "batch_id", batch.ID, "upload_id", accepted.UploadID, "task_id", accepted.TaskID, "files", len(batch.Files))
	return true, nil
}

func (w *Worker) handleFailure(ctx context.Context, batch spool.Batch, err error) error {
	cfg := w.config()
	var failure *Failure
	if !errors.As(err, &failure) {
		failure = &Failure{Kind: Retryable, Message: err.Error(), Err: err}
	}
	switch failure.Kind {
	case Uncertain:
		w.logger.Error("upload result uncertain", "component", "backend.upload", "code", "UPLOAD_UNCERTAIN", "batch_id", batch.ID, "error", failure.Error())
		return w.store.MarkUncertain(ctx, batch.ID, failure.Error())
	case Rejected:
		return w.store.MarkDead(ctx, batch.ID, failure.Error())
	case Pause:
		w.paused.Store(true)
		w.logger.Error("uploads paused after authorization failure", "component", "backend.upload", "code", "UPLOAD_REJECTED", "status", failure.StatusCode)
		return w.store.MarkPending(ctx, batch.ID, failure.Error(), time.Now().UTC().Add(24*time.Hour))
	case Split:
		if len(batch.Files) == 1 {
			return w.store.MarkDead(ctx, batch.ID, failure.Error())
		}
		return w.store.SplitUploading(ctx, batch.ID)
	default:
		if cfg.MaxAttempts > 0 && batch.AttemptCount >= cfg.MaxAttempts {
			w.logger.Error("upload batch exceeded max attempts", "component", "backend.upload", "batch_id", batch.ID, "attempts", batch.AttemptCount, "error", failure.Error())
			return w.store.MarkDead(ctx, batch.ID, "达到最大重试次数: "+failure.Error())
		}
		delay := retryDelay(batch.AttemptCount)
		if failure.RetryAfter > delay {
			delay = failure.RetryAfter
		}
		return w.store.MarkPending(ctx, batch.ID, failure.Error(), time.Now().UTC().Add(delay))
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << min(attempt-1, 5)
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}
