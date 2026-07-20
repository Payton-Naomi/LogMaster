package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/config"
	"github.com/Payton-Naomi/LogMaster/agent/internal/logfile"
	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
	"github.com/Payton-Naomi/LogMaster/agent/internal/source"
	"github.com/Payton-Naomi/LogMaster/agent/internal/store"
	"github.com/Payton-Naomi/LogMaster/agent/internal/upload"
)

type App struct {
	cfg      config.Config
	store    *store.Store
	files    *logfile.Writer
	uploader upload.Uploader
	logger   *slog.Logger
}

type capturedLine struct {
	deviceSN string
	time     time.Time
	content  string
}

func New(cfg config.Config, storage *store.Store, files *logfile.Writer, uploader upload.Uploader, logger *slog.Logger) *App {
	return &App{cfg: cfg, store: storage, files: files, uploader: uploader, logger: logger}
}

func (a *App) Run(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	lines := make(chan capturedLine, 4096)
	wakeUpload := make(chan struct{}, 1)
	fatal := make(chan error, 1)

	var persistWG sync.WaitGroup
	persistWG.Add(1)
	go func() {
		defer persistWG.Done()
		if err := a.persist(ctx, lines, wakeUpload); err != nil && !errors.Is(err, context.Canceled) {
			select {
			case fatal <- err:
			default:
			}
		}
	}()

	var uploadWG sync.WaitGroup
	uploadWG.Add(1)
	go func() { defer uploadWG.Done(); a.uploadLoop(ctx, wakeUpload) }()

	var sourceWG sync.WaitGroup
	for _, device := range a.cfg.Devices {
		src, err := source.New(device, a.logger)
		if err != nil {
			cancel()
			sourceWG.Wait()
			close(lines)
			persistWG.Wait()
			uploadWG.Wait()
			return err
		}
		device := device
		sourceWG.Add(1)
		go func() {
			defer sourceWG.Done()
			err := src.Run(ctx, func(content string) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case lines <- capturedLine{deviceSN: device.DeviceSN, time: time.Now().UTC(), content: content}:
					return nil
				}
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				select {
				case fatal <- fmt.Errorf("source %s: %w", device.DeviceSN, err):
				default:
				}
			}
		}()
	}

	var runErr error
	select {
	case <-parent.Done():
		runErr = nil
	case runErr = <-fatal:
	}
	cancel()
	sourceWG.Wait()
	close(lines)
	persistWG.Wait()
	uploadWG.Wait()
	return runErr
}

func (a *App) persist(ctx context.Context, lines <-chan capturedLine, wake chan<- struct{}) error {
	sequences := make(map[string]int64, len(a.cfg.Devices))
	for _, device := range a.cfg.Devices {
		sequence, err := a.store.MaxSequence(context.Background(), device.DeviceSN)
		if err != nil {
			return fmt.Errorf("load sequence for %s: %w", device.DeviceSN, err)
		}
		sequences[device.DeviceSN] = sequence
	}
	for line := range lines {
		sequences[line.deviceSN]++
		entry := model.LogEntry{DeviceSN: line.deviceSN, Sequence: sequences[line.deviceSN], CapturedAt: line.time, Content: line.content}
		if err := a.files.Append(entry); err != nil {
			return fmt.Errorf("append raw log: %w", err)
		}
		if err := a.store.Append(context.Background(), entry); err != nil {
			return fmt.Errorf("persist outbox: %w", err)
		}
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	return nil
}

type retryState struct {
	attempts  int
	next      time.Time
	lastFlush time.Time
}

func (a *App) uploadLoop(ctx context.Context, wake <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	states := map[string]*retryState{}
	for _, d := range a.cfg.Devices {
		states[d.DeviceSN] = &retryState{lastFlush: time.Now()}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-ticker.C:
		}
		now := time.Now()
		for _, device := range a.cfg.Devices {
			state := states[device.DeviceSN]
			if now.Before(state.next) {
				continue
			}
			count, err := a.store.PendingCount(ctx, device.DeviceSN)
			if err != nil {
				a.logger.Error("count pending logs", "device", device.DeviceSN, "error", err)
				continue
			}
			if count == 0 {
				state.attempts = 0
				continue
			}
			due := count >= a.cfg.Upload.BatchSize || now.Sub(state.lastFlush) >= a.cfg.Upload.Interval || state.attempts > 0
			if !due {
				continue
			}
			if err := a.uploadDevice(ctx, device.DeviceSN); err != nil {
				state.attempts++
				delay := retryDelay(state.attempts)
				state.next = time.Now().Add(delay)
				a.logger.Warn("upload failed; batch retained", "device", device.DeviceSN, "retry_in", delay, "error", err)
				continue
			}
			state.attempts = 0
			state.next = time.Time{}
			state.lastFlush = time.Now()
		}
	}
}

func (a *App) uploadDevice(ctx context.Context, deviceSN string) error {
	for i := 0; i < 100; i++ {
		batch, err := a.store.ClaimBatch(ctx, deviceSN, a.cfg.Upload.BatchSize)
		if err != nil {
			return err
		}
		if batch == nil {
			return nil
		}
		batch.AgentID = a.cfg.AgentID
		batch.ProjectID = a.cfg.ProjectID
		batch.SentAt = time.Now().UTC()
		if err := a.uploader.Upload(ctx, *batch); err != nil {
			return err
		}
		deleted, err := a.store.Acknowledge(ctx, batch.BatchID)
		if err != nil {
			return err
		}
		if deleted != int64(len(batch.Logs)) {
			return fmt.Errorf("acknowledged %d rows for %d-log batch %s", deleted, len(batch.Logs), batch.BatchID)
		}
		a.logger.Info("batch uploaded", "device", deviceSN, "batch_id", batch.BatchID, "logs", len(batch.Logs))
	}
	return errors.New("upload drain limit reached")
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
