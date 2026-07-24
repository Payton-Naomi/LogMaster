package backend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"logmaster-agent/agent/internal/spool"
)

type WorkerConfig struct {
	Interval            time.Duration
	MaxFiles            int
	Concurrency         int
	InspectBeforeUpload bool
}

type Worker struct {
	cfg    WorkerConfig
	store  *spool.Store
	client *Client
	logger *slog.Logger
	paused atomic.Bool
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
	return &Worker{cfg: cfg, store: store, client: client, logger: logger}
}

func (w *Worker) Paused() bool { return w.paused.Load() }

func (w *Worker) Run(ctx context.Context) {
	if w.cfg.Concurrency == 1 {
		w.runLoop(ctx)
		return
	}
	done := make(chan struct{}, w.cfg.Concurrency)
	for i := 0; i < w.cfg.Concurrency; i++ {
		go func() {
			w.runLoop(ctx)
			done <- struct{}{}
		}()
	}
	for i := 0; i < w.cfg.Concurrency; i++ {
		<-done
	}
}

func (w *Worker) runLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
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
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	batch, err := w.store.ClaimReady(ctx, w.cfg.MaxFiles)
	if err != nil || batch == nil {
		return false, err
	}
	for _, file := range batch.Files {
		if err := spool.VerifyFile(file); err != nil {
			return true, w.store.MarkDead(ctx, batch.ID, "spool file invalid: "+err.Error())
		}
		if w.cfg.InspectBeforeUpload {
			if err := w.client.Inspect(ctx, file); err != nil {
				return true, w.handleFailure(ctx, *batch, err)
			}
		}
	}
	accepted, err := w.client.Upload(ctx, *batch)
	if err != nil {
		return true, w.handleFailure(ctx, *batch, err)
	}
	if err := w.store.MarkUploaded(ctx, batch.ID, accepted.UploadID, accepted.TaskID); err != nil {
		return true, fmt.Errorf("persist upload acknowledgement: %w", err)
	}
	w.logger.Info("batch uploaded", "component", "backend.upload", "batch_id", batch.ID, "upload_id", accepted.UploadID, "task_id", accepted.TaskID, "files", len(batch.Files))
	return true, nil
}

func (w *Worker) handleFailure(ctx context.Context, batch spool.Batch, err error) error {
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
