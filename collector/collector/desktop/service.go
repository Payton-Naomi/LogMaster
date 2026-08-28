package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/collector"
	"logmaster-agent/agent/internal/config"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

type Service struct {
	ctx                    context.Context
	cancel                 context.CancelFunc
	manager                *collector.Manager
	store                  *spool.Store
	worker                 *backend.Worker
	backendClient          *backend.Client
	close                  sync.Once
	mu                     sync.RWMutex
	configs                map[string]DeviceConfigDTO
	configPath             string
	rootDirectory          string
	catalogPath            string
	catalogDirectory       string
	cloudKeywordPath       string
	settingsPath           string
	catalog                CatalogConfig
	appSettings            AppSettingsDTO
	pendingImports         map[string]pendingCatalogImport
	connectErrors          map[string]string
	configDirty            map[string]bool
	uploadProgress         map[string]UploadProgressDTO
	spoolDirectory         string
	maxDiskBytes           int64
	workerCancel           context.CancelFunc
	workerCtx              context.Context
	workerDone             chan struct{}
	workerStarted          chan struct{}
	workerStart            sync.Once
	logger                 *slog.Logger
	logWriter              *rotatingFile
	reportedServerFailures map[string]struct{}
	previousConfigs        map[string]DeviceConfigDTO
	manualSavedSessions    map[string]struct{}
	startupWarnings        []string
	runtimeStarted         bool
}

type PortInfo struct {
	Name         string `json:"name"`
	VID          string `json:"vid"`
	PID          string `json:"pid"`
	USBSerial    string `json:"usbSerial"`
	Location     string `json:"location"`
	Manufacturer string `json:"manufacturer"`
	Product      string `json:"product"`
	IsUSB        bool   `json:"isUSB"`
}
type DeviceConfigDTO struct {
	DeviceID                string   `json:"deviceId"`
	Name                    string   `json:"name"`
	PortName                string   `json:"portName"`
	BaudRate                int      `json:"baudRate"`
	DataBits                int      `json:"dataBits"`
	StopBits                int      `json:"stopBits"`
	Parity                  string   `json:"parity"`
	Handshake               string   `json:"handshake"`
	Encoding                string   `json:"encoding"`
	DTR                     bool     `json:"dtr"`
	RTS                     bool     `json:"rts"`
	ReadTimeoutMs           int      `json:"readTimeoutMs"`
	WriteTimeoutMs          int      `json:"writeTimeoutMs"`
	IdleGapMs               int      `json:"idleGapMs"`
	MaxFrameBytes           int      `json:"maxFrameBytes"`
	Configured              bool     `json:"configured"`
	ProjectID               string   `json:"projectId"`
	ProjectName             string   `json:"projectName"`
	Version                 string   `json:"version"`
	TestTaskID              string   `json:"testTaskId"`
	TestTaskName            string   `json:"testTaskName"`
	UploaderName            string   `json:"uploaderName"`
	UploaderEmail           string   `json:"uploaderEmail"`
	Remark                  string   `json:"remark"`
	ScenarioIDs             []string `json:"scenarioIds"`
	KeywordProfileID        string   `json:"keywordProfileId"`
	KeywordRuleIDs          []string `json:"keywordRuleIds"`
	KeywordMatchingEnabled  bool     `json:"keywordMatchingEnabled"`
	SaveEnabled             bool     `json:"saveEnabled"`
	UploadEnabled           bool     `json:"uploadEnabled"`
	NoLogTimeoutSeconds     int      `json:"noLogTimeoutSeconds"`
	VID                     string   `json:"vid"`
	PID                     string   `json:"pid"`
	USBSerial               string   `json:"usbSerial"`
	Location                string   `json:"location"`
	UploadSessionID         string   `json:"uploadSessionId,omitempty"`
	QueryCode               string   `json:"queryCode,omitempty"`
	UploadSetupID           string   `json:"uploadSetupId,omitempty"`
	UploadSetupState        string   `json:"uploadSetupState,omitempty"`
	UploadConfigFingerprint string   `json:"uploadConfigFingerprint,omitempty"`
	ConfigSnapshot          string   `json:"configSnapshot,omitempty"`
	PreviousConfigAvailable bool     `json:"previousConfigAvailable,omitempty"`
}

type DeviceConfigSaveResult struct {
	Saved       bool   `json:"saved"`
	UploadReady bool   `json:"uploadReady"`
	QueryCode   string `json:"queryCode,omitempty"`
	Message     string `json:"message,omitempty"`
}
type DeviceStateDTO struct {
	DeviceID       string            `json:"deviceId"`
	Name           string            `json:"name"`
	PortName       string            `json:"portName"`
	Status         string            `json:"status"`
	LastError      string            `json:"lastError,omitempty"`
	DroppedEvents  uint64            `json:"droppedEvents"`
	LinesReceived  uint64            `json:"linesReceived"`
	Reconnects     uint64            `json:"reconnects"`
	RuleCounts     map[string]uint64 `json:"ruleCounts"`
	Config         DeviceConfigDTO   `json:"config"`
	Enabled        bool              `json:"enabled"`
	Detected       bool              `json:"detected"`
	Persisting     bool              `json:"persisting"`
	UploadEnabled  bool              `json:"uploadEnabled"`
	NoLogAlert     bool              `json:"noLogAlert"`
	LastLogAt      *time.Time        `json:"lastLogAt,omitempty"`
	SessionID      string            `json:"sessionId"`
	ConfigStatus   string            `json:"configStatus"`
	StorageBytes   int64             `json:"storageBytes"`
	PendingUploads int64             `json:"pendingUploads"`
}
type UploadBatchDTO struct {
	ID             string     `json:"id"`
	State          string     `json:"state"`
	DeviceID       string     `json:"deviceId"`
	FileName       string     `json:"fileName"`
	SizeBytes      int64      `json:"sizeBytes"`
	SHA256         string     `json:"sha256"`
	AttemptCount   int        `json:"attemptCount"`
	LastError      string     `json:"lastError"`
	CreatedAt      time.Time  `json:"createdAt"`
	ProjectName    string     `json:"projectName"`
	Version        string     `json:"version"`
	SessionID      string     `json:"sessionId"`
	QueryCode      string     `json:"queryCode"`
	UploadPosition int        `json:"uploadPosition"`
	BytesTotal     int64      `json:"bytesTotal"`
	BytesSent      int64      `json:"bytesSent"`
	SpeedBytes     int64      `json:"speedBytes"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}
type UploadTaskDTO struct {
	ID            string     `json:"id"`
	State         string     `json:"state"`
	ProjectName   string     `json:"projectName"`
	UploaderName  string     `json:"uploaderName"`
	UploaderEmail string     `json:"uploaderEmail"`
	TestTaskName  string     `json:"testTaskName"`
	PortName      string     `json:"portName"`
	DeviceID      string     `json:"deviceId"`
	QueryCode     string     `json:"queryCode"`
	FileCount     int        `json:"fileCount"`
	BytesTotal    int64      `json:"bytesTotal"`
	BytesSent     int64      `json:"bytesSent"`
	SpeedBytes    int64      `json:"speedBytes"`
	LastError     string     `json:"lastError"`
	BatchIDs      []string   `json:"batchIds"`
	CreatedAt     time.Time  `json:"createdAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}
type desktopSettings struct {
	Version int               `json:"version"`
	Devices []DeviceConfigDTO `json:"devices"`
}
type QueueStatus struct {
	Pending          int64  `json:"pending"`
	Uploading        int64  `json:"uploading"`
	Uploaded         int64  `json:"uploaded"`
	Uncertain        int64  `json:"uncertain"`
	Dead             int64  `json:"dead"`
	DiskUsagePercent int    `json:"diskUsagePercent"`
	DiskUsageText    string `json:"diskUsageText"`
}
type UncertainCheckDTO struct {
	BatchID   string `json:"batchId"`
	QueryCode string `json:"queryCode"`
	Status    string `json:"status"`
	UploadID  string `json:"uploadId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	Matched   bool   `json:"matched"`
}
type LogRow struct {
	DeviceID   string              `json:"deviceId"`
	DeviceName string              `json:"deviceName"`
	Timestamp  string              `json:"timestamp"`
	Text       string              `json:"text"`
	Message    string              `json:"message"`
	Level      string              `json:"level"`
	Module     string              `json:"module"`
	CapturedAt time.Time           `json:"capturedAt"`
	Sequence   int64               `json:"sequence"`
	Hits       []collector.RuleHit `json:"hits,omitempty"`
}

func NewService() (*Service, error) {
	// Wails executes the application once while it generates Go bindings. That
	// process must not open an operator's active SQLite database. The build
	// script sets this private root only for that short-lived generation step;
	// normal desktop launches continue to use the portable/user runtime root.
	if buildRoot := strings.TrimSpace(os.Getenv("LOGMASTER_BUILD_BINDINGS_ROOT")); buildRoot != "" {
		return newServiceAt(filepath.Clean(buildRoot))
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	root := filepath.Join(base, "LogMaster")
	cfg, configured, err := loadDesktopRuntimeConfig(root)
	if err != nil {
		return nil, err
	}
	return newServiceAtConfig(root, cfg, configured)
}

func newServiceAt(root string) (*Service, error) {
	return newServiceAtConfig(root, defaultDesktopConfig(), false)
}

func defaultDesktopConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Agent.ID, cfg.Agent.Name = "desktop-collector", "LogMaster采集端"
	cfg.Backend.BaseURL, cfg.Backend.ProjectName, cfg.Backend.Version = "http://127.0.0.1:8080/api", "default", "V1.0.0"
	cfg.Serial.Ports = nil
	return cfg
}

func loadDesktopRuntimeConfig(root string) (config.Config, bool, error) {
	if explicit := strings.TrimSpace(os.Getenv("LOGMASTER_CONFIG")); explicit != "" {
		cfg, err := config.Load(explicit)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("load LOGMASTER_CONFIG %s: %w", explicit, err)
		}
		resolveDesktopStoragePaths(&cfg, filepath.Dir(explicit))
		return cfg, true, nil
	}
	candidates := []string{filepath.Join(root, "config.yaml")}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "config.yaml"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return config.Config{}, false, fmt.Errorf("inspect desktop config %s: %w", path, err)
		}
		cfg, err := config.Load(path)
		if err != nil {
			return config.Config{}, false, fmt.Errorf("load desktop config %s: %w", path, err)
		}
		resolveDesktopStoragePaths(&cfg, filepath.Dir(path))
		return cfg, true, nil
	}
	return defaultDesktopConfig(), false, nil
}

func resolveDesktopStoragePaths(cfg *config.Config, base string) {
	if !filepath.IsAbs(cfg.Spool.Directory) {
		cfg.Spool.Directory = filepath.Join(base, cfg.Spool.Directory)
	}
	if !filepath.IsAbs(cfg.Spool.SQLitePath) {
		cfg.Spool.SQLitePath = filepath.Join(base, cfg.Spool.SQLitePath)
	}
}

func newServiceAtConfig(root string, cfg config.Config, configured bool) (*Service, error) {
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	catalogFiles := catalogFilesForRoot(root)
	if err := os.MkdirAll(catalogFiles.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("create portable config directory: %w", err)
	}
	settingsPath := filepath.Join(catalogFiles.Directory, "settings-config.json")
	legacySettingsPath := filepath.Join(root, "app-settings.json")
	if _, statErr := os.Stat(settingsPath); errors.Is(statErr, os.ErrNotExist) {
		if legacyData, readErr := os.ReadFile(legacySettingsPath); readErr == nil {
			if writeErr := atomicWriteFile(settingsPath, legacyData, 0o644); writeErr != nil {
				return nil, fmt.Errorf("migrate app settings: %w", writeErr)
			}
		}
	}
	appSettings, settingsWarning, err := loadAppSettingsRecovered(settingsPath, root)
	if err != nil {
		return nil, err
	}
	if migrated, didMigrate, migrateErr := migrateLegacySettingsIntoUntouchedPortableConfig(settingsPath, legacySettingsPath, root); migrateErr != nil {
		// A legacy file must never prevent startup. The portable file remains the
		// source of truth if the old file cannot be read safely.
		settingsWarning = "旧版全局设置无法迁移，已使用同级 config 中的设置"
	} else if didMigrate {
		appSettings = migrated
		settingsWarning = "已自动迁移旧版全局设置到同级 config 目录"
	}
	if !configured {
		cfg.Spool.Directory = appSettings.DefaultLogDirectory
		cfg.Spool.SQLitePath = filepath.Join(dataDir, "collector.db")
		cfg.Spool.MaxDiskBytes = appSettings.MaxDiskBytes
		cfg.Backend.UploadInterval = durationSeconds(appSettings.UploadIntervalSeconds)
		cfg.Backend.UploadConcurrency = appSettings.UploadConcurrency
		cfg.Backend.UploadGzip = appSettings.UploadGzip
		if strings.TrimSpace(appSettings.BackendURL) != "" {
			cfg.Backend.BaseURL = strings.TrimSpace(appSettings.BackendURL)
		}
	} else {
		appSettings.DefaultLogDirectory = cfg.Spool.Directory
		appSettings.BackendURL = cfg.Backend.BaseURL
		appSettings.UploadIntervalSeconds = int(cfg.Backend.UploadInterval / time.Second)
		appSettings.UploadConcurrency = cfg.Backend.UploadConcurrency
		appSettings.UploadGzip = cfg.Backend.UploadGzip
		appSettings.MaxDiskBytes = cfg.Spool.MaxDiskBytes
	}
	// Always materialize a complete settings file. This migrates older partial
	// files and gives a fresh installation editable, non-empty defaults before
	// the frontend starts.
	normalizeAppSettings(&appSettings, root)
	if err := writeAppSettings(settingsPath, appSettings); err != nil {
		return nil, fmt.Errorf("persist app settings: %w", err)
	}
	if err := os.MkdirAll(cfg.Spool.Directory, 0o755); err != nil {
		return nil, err
	}
	store, err := spool.Open(cfg.Spool.SQLitePath)
	if err != nil {
		return nil, err
	}
	managerCfg := collector.Config{MaxDevices: collector.MaxSupportedDevices, EventCapacity: 2048, SpoolDirectory: cfg.Spool.Directory, MaxDiskBytes: cfg.Spool.MaxDiskBytes, StorageWarningPercent: appSettings.StorageWarningPercent, ProjectName: cfg.Backend.ProjectName, Version: cfg.Backend.Version, Reconnect: serialagent.ReconnectConfig{InitialDelay: cfg.Serial.Reconnect.InitialDelay, Multiplier: cfg.Serial.Reconnect.Multiplier, MaxDelay: cfg.Serial.Reconnect.MaxDelay, Jitter: cfg.Serial.Reconnect.Jitter}, DiskCheckEvery: 5 * time.Second}
	manager, err := collector.New(managerCfg, store, nil, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	uploadToken := ""
	if cfg.Backend.AuthorizationTokenEnv != "" {
		uploadToken = os.Getenv(cfg.Backend.AuthorizationTokenEnv)
	}
	logger, logWriter, err := newDesktopLoggerWithWriter(root)
	if err != nil {
		logger = slog.Default()
		logWriter = nil
	}
	logger.Info("desktop service initializing", "component", "desktop.service", "backend", cfg.Backend.BaseURL, "spool", cfg.Spool.Directory)
	client := backend.New(backend.Config{BaseURL: cfg.Backend.BaseURL, HealthPath: cfg.Backend.HealthPath, InspectPath: cfg.Backend.InspectPath, UploadPath: cfg.Backend.UploadPath, Timeout: cfg.Backend.RequestTimeout, Authorization: uploadToken, Gzip: cfg.Backend.UploadGzip})
	worker := backend.NewWorker(backend.WorkerConfig{Interval: cfg.Backend.UploadInterval, MaxFiles: 16, Concurrency: cfg.Backend.UploadConcurrency, InspectBeforeUpload: false, MaxAttempts: 5, BatchInterval: 200 * time.Millisecond}, store, client, logger)
	ctx, cancel := context.WithCancel(context.Background())
	workerCtx, workerCancel := context.WithCancel(context.Background())
	catalogPath := filepath.Join(root, "project-catalog.yaml")
	catalog, loadedCatalogFiles, err := loadRuntimeCatalog(root, catalogPath)
	if err != nil {
		cancel()
		workerCancel()
		manager.Close()
		store.Close()
		return nil, err
	}
	configs := make(map[string]DeviceConfigDTO, len(cfg.Serial.Ports))
	for i, port := range cfg.Serial.Ports {
		if strings.TrimSpace(port.PortName) == "" {
			continue
		}
		id := strings.TrimSpace(port.DeviceSN)
		if id == "" {
			id = serialDeviceID(port.PortName, i)
		}
		isConfigured := strings.TrimSpace(cfg.Backend.ProjectName) != "" && strings.TrimSpace(cfg.Backend.Version) != ""
		configs[id] = DeviceConfigDTO{DeviceID: id, Name: defaultSerialChannelName(port.PortName, i+1), PortName: port.PortName, BaudRate: port.BaudRate, DataBits: port.DataBits, StopBits: port.StopBits, Parity: port.Parity, Handshake: port.Handshake, DTR: port.DTR, RTS: port.RTS, Configured: isConfigured, ProjectID: cfg.Backend.ProjectName, ProjectName: cfg.Backend.ProjectName, Version: cfg.Backend.Version, SaveEnabled: isConfigured, UploadEnabled: isConfigured, NoLogTimeoutSeconds: appSettings.NoLogTimeoutSeconds, VID: port.PortMatch.VID, PID: port.PortMatch.PID, USBSerial: port.PortMatch.USBSerial, Location: port.PortMatch.PhysicalLocation}
	}
	service := &Service{manager: manager, store: store, worker: worker, backendClient: client, ctx: ctx, cancel: cancel, configs: configs, configPath: filepath.Join(root, "desktop-config.json"), rootDirectory: root, catalogPath: catalogPath, catalogDirectory: loadedCatalogFiles.Directory, cloudKeywordPath: filepath.Join(root, "cloud-keywords.json"), settingsPath: settingsPath, catalog: catalog, appSettings: appSettings, pendingImports: map[string]pendingCatalogImport{}, connectErrors: map[string]string{}, configDirty: map[string]bool{}, uploadProgress: map[string]UploadProgressDTO{}, spoolDirectory: cfg.Spool.Directory, maxDiskBytes: cfg.Spool.MaxDiskBytes, workerCtx: workerCtx, workerCancel: workerCancel, workerDone: make(chan struct{}), workerStarted: make(chan struct{}), logger: logger, logWriter: logWriter, reportedServerFailures: map[string]struct{}{}, previousConfigs: map[string]DeviceConfigDTO{}, manualSavedSessions: map[string]struct{}{}}
	if settingsWarning != "" {
		service.startupWarnings = append(service.startupWarnings, settingsWarning)
		logger.Warn("application settings recovered", "component", "desktop.settings", "warning", settingsWarning)
	}
	worker.SetProgressHandler(service.handleUploadProgress)
	if err := service.loadSettings(); err != nil {
		cancel()
		workerCancel()
		manager.Close()
		store.Close()
		return nil, err
	}
	service.loadCloudKeywordCache()
	for _, dto := range service.configs {
		if dto.UploadEnabled {
			ready := dto.UploadSetupState == "active" && dto.UploadSessionID != "" && dto.QueryCode != ""
			if err := store.SetDeviceUploadPaused(ctx, dto.DeviceID, !ready); err != nil {
				manager.Close()
				store.Close()
				return nil, fmt.Errorf("restore upload session policy for %s: %w", dto.DeviceID, err)
			}
		}
	}
	if _, err := store.Recover(context.Background(), 5*time.Minute); err != nil {
		cancel()
		workerCancel()
		manager.Close()
		store.Close()
		return nil, fmt.Errorf("recover upload queue: %w", err)
	}
	for _, dto := range service.configs {
		if strings.TrimSpace(dto.PortName) == "" {
			continue
		}
		if _, err := manager.RecoverDevice(context.Background(), service.toCollectorConfig(dto)); err != nil {
			cancel()
			workerCancel()
			manager.Close()
			store.Close()
			return nil, fmt.Errorf("recover device %s: %w", dto.DeviceID, err)
		}
	}
	return service, nil
}

func (s *Service) startup(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.runtimeStarted = true
	s.mu.Unlock()
	go s.pumpEvents()
	go s.monitorPorts()
	go s.cleanupLoop()
	go s.monitorUploadSessionFailures()
	s.workerStart.Do(func() { close(s.workerStarted); go func() { defer close(s.workerDone); s.worker.Run(s.workerCtx) }() })
}

func (s *Service) canEmitRuntimeEvents() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeStarted
}
func (s *Service) shutdown() {
	s.close.Do(func() {
		s.cancel()
		s.workerCancel()
		_ = s.manager.Close()
		select {
		case <-s.workerStarted:
			<-s.workerDone
		default:
		}
		s.cleanupUnexportedLocalFiles()
		_ = s.store.Close()
		if s.logger != nil {
			s.logger.Info("desktop service shut down", "component", "desktop.service")
		}
		if s.logWriter != nil {
			_ = s.logWriter.Close()
		}
	})
}

func (s *Service) markSessionManuallySaved(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.mu.Lock()
	if s.manualSavedSessions == nil {
		s.manualSavedSessions = make(map[string]struct{})
	}
	s.manualSavedSessions[sessionID] = struct{}{}
	s.mu.Unlock()
}

func (s *Service) sessionWasManuallySaved(sessionID string) bool {
	s.mu.RLock()
	_, ok := s.manualSavedSessions[sessionID]
	s.mu.RUnlock()
	return ok
}

// cleanupUnexportedLocalFiles removes only current-run records that stayed
// local-only and were never manually exported. Older history and upload queue
// records are deliberately left intact for resume and later review.
func (s *Service) cleanupUnexportedLocalFiles() {
	currentSessions := make(map[string]struct{})
	for _, state := range s.manager.GetDeviceStates() {
		if strings.TrimSpace(state.SessionID) != "" {
			currentSessions[state.SessionID] = struct{}{}
		}
	}
	if len(currentSessions) == 0 {
		return
	}
	files, err := s.store.ListLocalHistoryFiles(context.Background())
	if err != nil {
		if s.logger != nil {
			s.logger.Error("list local-only history files for shutdown cleanup failed", "component", "desktop.cleanup", "error", err)
		}
		return
	}
	removed := 0
	for _, file := range files {
		if _, ok := currentSessions[file.SessionID]; !ok {
			continue
		}
		if s.sessionWasManuallySaved(file.SessionID) {
			continue
		}
		temporaryPath := fmt.Sprintf("%s.deleting-%d", file.Path, time.Now().UnixNano())
		if err := os.Rename(file.Path, temporaryPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				temporaryPath = ""
			} else {
				if s.logger != nil {
					s.logger.Warn("stage local-only history file for shutdown cleanup failed", "component", "desktop.cleanup", "path", file.Path, "error", err)
				}
				continue
			}
		}
		if _, err := s.store.DeleteLocalHistoryRecord(context.Background(), file.ID); err != nil {
			if temporaryPath != "" {
				_ = os.Rename(temporaryPath, file.Path)
			}
			if s.logger != nil {
				s.logger.Warn("remove local-only history record on shutdown cleanup failed", "component", "desktop.cleanup", "id", file.ID, "error", err)
			}
			continue
		}
		if temporaryPath != "" {
			if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				if s.logger != nil {
					s.logger.Warn("remove staged local-only history file on shutdown cleanup failed", "component", "desktop.cleanup", "path", temporaryPath, "error", err)
				}
				continue
			}
		}
		if _, err := os.Stat(file.Path); err == nil {
			if s.logger != nil {
				s.logger.Warn("local-only history file remained after shutdown cleanup", "component", "desktop.cleanup", "path", file.Path)
			}
		}
		removed++
	}
	if removed > 0 && s.logger != nil {
		s.logger.Info("shutdown cleanup removed local-only history files", "component", "desktop.cleanup", "count", removed)
	}
}

// cleanupLoop periodically applies the user-configurable retention policies:
// deleting already-uploaded spool files and pruning old keyword-hit records.
func (s *Service) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	s.runCleanup()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runCleanup()
		}
	}
}

func (s *Service) runCleanup() {
	s.cleanupUploadedFiles()
	if _, err := s.store.PruneKeywordHits(s.ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
		s.logger.Error("prune keyword hits failed", "component", "desktop.cleanup", "error", err)
	}
}

func (s *Service) cleanupUploadedFiles() {
	s.mu.RLock()
	enabled := s.appSettings.AutoDeleteUploaded
	hours := s.appSettings.UploadedRetentionHours
	s.mu.RUnlock()
	if !enabled || hours <= 0 {
		return
	}
	before := time.Now().Add(-time.Duration(hours) * time.Hour)
	deleted, err := s.store.DeleteExpiredUploaded(s.ctx, before)
	if err != nil {
		s.logger.Error("cleanup uploaded files failed", "component", "desktop.cleanup", "error", err)
		return
	}
	if deleted > 0 {
		s.logger.Info("cleanup removed uploaded batches", "component", "desktop.cleanup", "count", deleted)
	}
}

func (s *Service) monitorPorts() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		ports, err := s.ScanPorts()
		if err == nil {
			s.disconnectRemovedPorts(ports)
			current := portSignature(ports)
			if current != last {
				last = current
				runtime.EventsEmit(s.ctx, "serial:ports", ports)
			}
		}
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) pumpEvents() {
	ch, unsubscribe := s.manager.SubscribeLogEvents()
	defer unsubscribe()
	flush := time.NewTicker(75 * time.Millisecond)
	defer flush.Stop()
	batch := make([]LogRow, 0, 128)
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if event.State != "" {
				runtime.EventsEmit(s.ctx, "collector:state", event)
				continue
			}
			module := ""
			if len(event.Hits) > 0 {
				module = event.Hits[0].Module
			}
			for _, hit := range event.Hits {
				if event.SessionID != "" {
					_, _ = s.store.RecordKeywordHit(context.Background(), spool.KeywordHit{SessionID: event.SessionID, DeviceSN: event.DeviceID, RuleID: hit.RuleID, RuleName: hit.RuleName, MatchedAt: event.CapturedAt, Sequence: event.Sequence, LineText: event.Text})
				}
			}
			batch = append(batch, LogRow{DeviceID: event.DeviceID, DeviceName: event.DeviceName, Timestamp: event.CapturedAt.Local().Format("[2006-01-02 15:04:05.000]"), CapturedAt: event.CapturedAt, Text: event.Text, Message: event.Text, Level: "", Module: module, Sequence: event.Sequence, Hits: event.Hits})
			if len(batch) >= 128 {
				runtime.EventsEmit(s.ctx, "collector:logs", batch)
				batch = make([]LogRow, 0, 128)
			}
		case <-flush.C:
			if len(batch) > 0 {
				runtime.EventsEmit(s.ctx, "collector:logs", batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *Service) ScanPorts() ([]PortInfo, error) {
	ports, err := s.manager.ScanPorts(s.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PortInfo, len(ports))
	for i, p := range ports {
		result[i] = PortInfo{Name: p.Name, VID: p.VID, PID: p.PID, USBSerial: p.USBSerial, Location: p.Location, Manufacturer: p.Manufacturer, Product: p.Product, IsUSB: p.IsUSB}
	}
	return result, nil
}
func (s *Service) SubscribeLogEvents(deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("device id is required")
	}
	return nil
}
func (s *Service) GetDeviceStates() []DeviceStateDTO {
	ports, err := s.manager.ScanPorts(s.ctx)
	if err != nil {
		ports = nil
	}
	raw := s.manager.GetDeviceStates()
	byID := make(map[string]collector.DeviceState, len(raw))
	for _, state := range raw {
		byID[state.DeviceID] = state
	}
	storage, _ := s.store.StorageByDevice(s.ctx)
	pending := map[string]int64{}
	if batches, _, queryErr := s.store.ListBatches(s.ctx, spool.BatchFilter{IncludeUploaded: false, Limit: 200}); queryErr == nil {
		for _, batch := range batches {
			if len(batch.Files) > 0 {
				pending[batch.Files[0].DeviceSN]++
			}
		}
	}
	s.mu.Lock()
	result := make([]DeviceStateDTO, 0, len(ports)+len(s.configs))
	for i, port := range ports {
		info := PortInfo{Name: port.Name, VID: port.VID, PID: port.PID, USBSerial: port.USBSerial, Location: port.Location, Manufacturer: port.Manufacturer, Product: port.Product, IsUSB: port.IsUSB}
		cfg := s.configForPortLocked(info, i)
		s.configs[cfg.DeviceID] = cfg
		id := cfg.DeviceID
		item := DeviceStateDTO{DeviceID: id, Name: cfg.Name, PortName: cfg.PortName, Status: "disconnected", Config: cfg, RuleCounts: map[string]uint64{}, Detected: true, StorageBytes: storage[id], PendingUploads: pending[id], ConfigStatus: configStatus(cfg, s.catalog)}
		if state, ok := byID[id]; ok {
			item.Name = state.DeviceName
			item.PortName = state.PortName
			item.Status = string(state.State)
			item.LastError = state.LastError
			item.DroppedEvents = state.DroppedEvents
			item.LinesReceived = state.LinesReceived
			item.Reconnects = state.Reconnects
			item.RuleCounts = state.RuleCounts
			item.Enabled = state.Enabled
			item.Persisting = state.Persisting
			item.UploadEnabled = state.UploadEnabled
			item.NoLogAlert = state.NoLogAlert
			item.LastLogAt = state.LastLogAt
			item.SessionID = state.SessionID
		}
		if message := s.connectErrors[id]; message != "" && !item.Enabled {
			item.Status = "error"
			item.LastError = message
		}
		result = append(result, item)
	}
	s.mu.Unlock()
	sort.Slice(result, func(i, j int) bool { return naturalPortLess(result[i].PortName, result[j].PortName) })
	return result
}

func (s *Service) configForPortLocked(value any, index int) DeviceConfigDTO {
	port := PortInfo{}
	switch typed := value.(type) {
	case PortInfo:
		port = typed
	case string:
		port.Name = typed
	}
	name := strings.TrimSpace(port.Name)
	for _, cfg := range s.configs {
		// Only reuse an existing channel's in-session configuration when the
		// physical device is provably the same one: a stable USB identity match,
		// or (for adapters without a USB serial) the same port name. A different
		// device plugged into a previously used COM port must start from clean
		// defaults instead of inheriting the previous device's upload settings.
		stableMatch := port.USBSerial != "" && cfg.USBSerial != "" && strings.EqualFold(port.USBSerial, cfg.USBSerial) && strings.EqualFold(port.VID, cfg.VID) && strings.EqualFold(port.PID, cfg.PID)
		nameMatch := strings.TrimSpace(cfg.USBSerial) == "" && strings.TrimSpace(port.USBSerial) == "" && strings.EqualFold(strings.TrimSpace(cfg.PortName), name)
		if stableMatch {
			cfg.PortName = name
			cfg.VID, cfg.PID, cfg.USBSerial, cfg.Location = port.VID, port.PID, port.USBSerial, port.Location
			if strings.TrimSpace(cfg.Name) == "" {
				cfg.Name = defaultSerialChannelName(name, index+1)
			}
			return s.applyLogConfigDefaultsLocked(normalizeDeviceConfig(cfg))
		}
		if nameMatch {
			// Some USB-serial adapters do not expose a serial number. When the
			// port, VID/PID, and physical location all match, the saved record is
			// still tied to the same hardware and its full business configuration
			// can be restored safely. COM-only matches remain serial-only.
			hardwareMatch := strings.TrimSpace(cfg.VID) != "" && strings.TrimSpace(cfg.PID) != "" && strings.TrimSpace(cfg.Location) != "" &&
				strings.EqualFold(strings.TrimSpace(cfg.VID), strings.TrimSpace(port.VID)) &&
				strings.EqualFold(strings.TrimSpace(cfg.PID), strings.TrimSpace(port.PID)) &&
				strings.EqualFold(strings.TrimSpace(cfg.Location), strings.TrimSpace(port.Location))
			if hardwareMatch {
				cfg.PortName = name
				cfg.VID, cfg.PID, cfg.USBSerial, cfg.Location = port.VID, port.PID, port.USBSerial, port.Location
				return s.applyLogConfigDefaultsLocked(normalizeDeviceConfig(cfg))
			}
			if cfg.Configured || strings.TrimSpace(cfg.ProjectID) != "" || strings.TrimSpace(cfg.UploaderEmail) != "" || strings.TrimSpace(cfg.Remark) != "" {
				if s.previousConfigs == nil {
					s.previousConfigs = map[string]DeviceConfigDTO{}
				}
				s.previousConfigs[strings.ToUpper(name)] = cfg
			}
			inherited := serialOnlyInheritedConfig(cfg, port, index)
			inherited.PreviousConfigAvailable = s.previousConfigs[strings.ToUpper(name)].Configured
			return inherited
		}
	}
	defaults := defaultDesktopPortConfig()
	dto := DeviceConfigDTO{DeviceID: serialDeviceID(name, index), Name: defaultSerialChannelName(name, index+1), PortName: name, BaudRate: defaults.BaudRate, DataBits: defaults.DataBits, StopBits: defaults.StopBits, Parity: defaults.Parity, Handshake: defaults.Handshake, Encoding: "utf-8", DTR: defaults.DTR, RTS: defaults.RTS, SaveEnabled: s.appSettings.DefaultSaveEnabled, UploadEnabled: false, NoLogTimeoutSeconds: s.appSettings.NoLogTimeoutSeconds, VID: port.VID, PID: port.PID, USBSerial: port.USBSerial, Location: port.Location}
	dto.PreviousConfigAvailable = s.previousConfigs[strings.ToUpper(name)].Configured
	return s.applyLogConfigDefaultsLocked(dto)
}

func serialOnlyInheritedConfig(source DeviceConfigDTO, port PortInfo, index int) DeviceConfigDTO {
	dto := DeviceConfigDTO{
		DeviceID: serialDeviceID(port.Name, index), Name: source.Name, PortName: strings.TrimSpace(port.Name),
		BaudRate: source.BaudRate, DataBits: source.DataBits, StopBits: source.StopBits, Parity: source.Parity,
		Handshake: source.Handshake, Encoding: source.Encoding, DTR: source.DTR, RTS: source.RTS,
		SaveEnabled: source.SaveEnabled, UploadEnabled: false, NoLogTimeoutSeconds: source.NoLogTimeoutSeconds,
		VID: port.VID, PID: port.PID, USBSerial: port.USBSerial, Location: port.Location,
	}
	dto.Configured = source.Configured
	dto.ReadTimeoutMs, dto.WriteTimeoutMs, dto.IdleGapMs, dto.MaxFrameBytes = source.ReadTimeoutMs, source.WriteTimeoutMs, source.IdleGapMs, source.MaxFrameBytes
	if strings.TrimSpace(dto.Name) == "" {
		dto.Name = defaultSerialChannelName(port.Name, index+1)
	}
	return normalizeDeviceConfig(dto)
}

// ReusePreviousDeviceConfig explicitly copies business fields from the last
// saved configuration. It is intentionally opt-in when a device has no USB
// serial, because a COM port alone cannot prove device identity.
func (s *Service) ReusePreviousDeviceConfig(id string) (DeviceConfigDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.configs[id]
	if !ok {
		return DeviceConfigDTO{}, errors.New("未找到串口通道")
	}
	key := strings.ToUpper(strings.TrimSpace(current.PortName))
	previous, ok := s.previousConfigs[key]
	if !ok || (!previous.Configured && strings.TrimSpace(previous.ProjectID) == "" && strings.TrimSpace(previous.UploaderEmail) == "" && strings.TrimSpace(previous.Remark) == "") {
		return DeviceConfigDTO{}, errors.New("该串口没有可复用的历史配置")
	}
	previous.DeviceID = current.DeviceID
	previous.PortName, previous.VID, previous.PID, previous.USBSerial, previous.Location = current.PortName, current.VID, current.PID, current.USBSerial, current.Location
	previous.Configured = false
	previous.PreviousConfigAvailable = false
	previous.UploadSessionID, previous.QueryCode, previous.UploadSetupID = "", "", ""
	previous.UploadSetupState, previous.UploadConfigFingerprint, previous.ConfigSnapshot = "", "", ""
	return normalizeDeviceConfig(previous), nil
}

func defaultDesktopPortConfig() config.PortConfig {
	defaults := config.DefaultPortConfig()
	d := packagedDefaults.Serial
	defaults.BaudRate = d.BaudRate
	defaults.DataBits = d.DataBits
	defaults.StopBits = d.StopBits
	defaults.Parity = d.Parity
	defaults.DTR = d.DTR
	defaults.RTS = d.RTS
	defaults.SegmentMaxAge = time.Duration(packagedDefaults.Log.SegmentMinutes) * time.Minute
	defaults.SegmentMaxBytes = packagedDefaults.Log.SegmentSizeMB * 1024 * 1024
	return defaults
}

func (s *Service) applyLogConfigDefaultsLocked(dto DeviceConfigDTO) DeviceConfigDTO {
	if dto.Configured {
		return dto
	}
	dto.SaveEnabled = s.appSettings.DefaultSaveEnabled
	dto.UploadEnabled = false
	dto.KeywordMatchingEnabled = false
	dto.ProjectID = ""
	dto.ProjectName = ""
	dto.Version = ""
	dto.TestTaskID = ""
	dto.TestTaskName = ""
	dto.KeywordProfileID = ""
	dto.KeywordRuleIDs = nil
	if dto.NoLogTimeoutSeconds <= 0 {
		dto.NoLogTimeoutSeconds = s.appSettings.NoLogTimeoutSeconds
	}
	return dto
}

func configStatus(cfg DeviceConfigDTO, catalog CatalogConfig) string {
	if !cfg.Configured {
		return "unconfigured"
	}
	if !cfg.UploadEnabled {
		return "saved"
	}
	if catalogSelectionValidIn(catalog, cfg) {
		return "saved"
	}
	return "invalid"
}

func naturalPortLess(left, right string) bool {
	parse := func(value string) (int, bool) {
		upper := strings.ToUpper(strings.TrimSpace(value))
		if !strings.HasPrefix(upper, "COM") {
			return 0, false
		}
		number, err := strconv.Atoi(strings.TrimPrefix(upper, "COM"))
		return number, err == nil
	}
	a, aok := parse(left)
	b, bok := parse(right)
	if aok && bok {
		return a < b
	}
	return left < right
}
func (s *Service) GetUploadQueueStatus() (QueueStatus, error) {
	counts, err := s.store.Counts(s.ctx)
	if err != nil {
		return QueueStatus{}, err
	}
	used, err := directorySize(s.spoolDirectory)
	if err != nil {
		return QueueStatus{}, err
	}
	percent := 0
	if s.maxDiskBytes > 0 {
		percent = int((used * 100) / s.maxDiskBytes)
		if percent > 100 {
			percent = 100
		}
	}
	return QueueStatus{Pending: counts[spool.Pending], Uploading: counts[spool.Uploading], Uploaded: counts[spool.Uploaded], Uncertain: counts[spool.Uncertain], Dead: counts[spool.Dead], DiskUsagePercent: percent, DiskUsageText: fmt.Sprintf("%s / %s", formatBytes(used), formatBytes(s.maxDiskBytes))}, nil
}

// GetCurrentPendingUploadBytes intentionally scopes the footer number to the
// selected channel's active upload session. Historical queues are not useful
// while an operator is watching a live port.
func (s *Service) GetCurrentPendingUploadBytes(deviceID string) (int64, error) {
	s.mu.RLock()
	cfg, ok := s.configs[deviceID]
	s.mu.RUnlock()
	if !ok || strings.TrimSpace(cfg.UploadSessionID) == "" {
		return 0, nil
	}
	batches, _, err := s.store.ListBatches(s.ctx, spool.BatchFilter{DeviceSN: deviceID, IncludeUploaded: true, Limit: 200})
	if err != nil {
		return 0, err
	}
	var total int64
	for _, batch := range batches {
		if batch.UploadSessionID != cfg.UploadSessionID {
			continue
		}
		switch batch.State {
		case spool.Pending:
			total += batch.BytesTotal
		case spool.Uploading:
			dto := s.toUploadBatchDTO(batch)
			remaining := dto.BytesTotal - dto.BytesSent
			if remaining > 0 {
				total += remaining
			}
		}
	}
	return total, nil
}
func (s *Service) GetUploadQueueBatches() ([]UploadBatchDTO, error) {
	var result []UploadBatchDTO
	for _, state := range []spool.State{spool.Uncertain, spool.Dead} {
		batches, err := s.store.ListByState(s.ctx, state)
		if err != nil {
			return nil, err
		}
		for _, batch := range batches {
			item := UploadBatchDTO{ID: batch.ID, State: string(batch.State), AttemptCount: batch.AttemptCount, LastError: batch.LastError, CreatedAt: batch.CreatedAt}
			if len(batch.Files) > 0 {
				item.DeviceID = batch.Files[0].DeviceSN
				item.FileName = filepath.Base(batch.Files[0].Path)
				item.SizeBytes = batch.Files[0].SizeBytes
				item.SHA256 = batch.Files[0].SHA256
			}
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *Service) RetryUncertain(id string) error { return s.store.RetryUncertain(s.ctx, id) }
func (s *Service) RetryDeadBatch(id string) error {
	if err := s.store.RetryDead(s.ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.uploadProgress, id)
	s.mu.Unlock()
	return nil
}
func (s *Service) ConfirmUncertain(id, uploadID, taskID string) error {
	if uploadID == "" || taskID == "" {
		return errors.New("upload id and task id are required")
	}
	return s.store.ConfirmUncertain(s.ctx, id, uploadID, taskID)
}

// CheckUncertain queries the public backend status without changing local
// state. This gives the operator evidence before confirming or retrying.
func (s *Service) CheckUncertain(id string) (UncertainCheckDTO, error) {
	batch, err := s.store.GetBatch(s.ctx, id)
	if err != nil {
		return UncertainCheckDTO{}, err
	}
	if batch.State != spool.Uncertain {
		return UncertainCheckDTO{}, fmt.Errorf("batch %s is not uncertain", id)
	}
	result := UncertainCheckDTO{BatchID: id, QueryCode: batch.QueryCode, Status: "not_found"}
	if strings.TrimSpace(batch.QueryCode) == "" {
		return result, nil
	}
	status, err := s.backendClient.QueryUploadSession(s.ctx, batch.QueryCode)
	if err != nil {
		return result, err
	}
	result.Status = status.Status
	for _, item := range status.Batches {
		if item.ClientRequestID != batch.ClientRequestID {
			continue
		}
		result.UploadID, result.TaskID, result.Matched = item.UploadID, item.TaskID, item.UploadID != "" && item.TaskID != ""
		break
	}
	return result, nil
}
func (s *Service) ConnectDevice(dto DeviceConfigDTO) error {
	dto = normalizeDeviceConfig(dto)
	config := s.toCollectorConfig(dto)
	if err := config.Serial.Validate(); err != nil {
		return err
	}
	if err := s.requireDetectedPort(dto.PortName); err != nil {
		s.mu.Lock()
		s.connectErrors[dto.DeviceID] = err.Error()
		s.mu.Unlock()
		return err
	}
	if err := s.manager.ConnectDevice(config); err != nil {
		s.mu.Lock()
		s.connectErrors[dto.DeviceID] = err.Error()
		s.mu.Unlock()
		return err
	}
	if dto.UploadEnabled && dto.UploadSetupState == "active" && dto.UploadSessionID != "" && dto.QueryCode != "" {
		if err := s.store.SetDeviceUploadPaused(s.ctx, dto.DeviceID, false); err != nil {
			_ = s.manager.DisconnectDevice(dto.DeviceID)
			return fmt.Errorf("resume device upload: %w", err)
		}
	}
	s.mu.Lock()
	s.configs[dto.DeviceID] = dto
	delete(s.connectErrors, dto.DeviceID)
	s.mu.Unlock()
	return s.saveSettings()
}

func (s *Service) requireDetectedPort(name string) error {
	ports, err := s.manager.ScanPorts(s.ctx)
	if err != nil {
		return fmt.Errorf("scan serial ports before connecting: %w", err)
	}
	for _, port := range ports {
		if strings.EqualFold(strings.TrimSpace(port.Name), strings.TrimSpace(name)) {
			return nil
		}
	}
	return fmt.Errorf("serial port %s is not currently detected by Windows", name)
}
func (s *Service) DisconnectDevice(id string) error {
	return s.DisconnectDeviceWithUploadPolicy(id, true)
}
func (s *Service) DisconnectDeviceWithUploadPolicy(id string, continueUpload bool) error {
	if !continueUpload {
		if err := s.store.SetDeviceUploadPaused(s.ctx, id, true); err != nil {
			return fmt.Errorf("pause device upload: %w", err)
		}
	}
	err := s.manager.DisconnectDevice(id)
	if err != nil && !continueUpload {
		_ = s.store.SetDeviceUploadPaused(s.ctx, id, false)
	}
	s.mu.Lock()
	delete(s.connectErrors, id)
	s.mu.Unlock()
	return err
}
func (s *Service) UpdateDeviceConfig(id string, dto DeviceConfigDTO) error {
	return s.SaveDeviceConfig(id, dto)
}
func (s *Service) StartTask(id string) error { return s.manager.StartTask(id) }
func (s *Service) StopTask(id string) error  { return s.manager.StopTask(id) }
func (s *Service) SendCommand(id, command string) error {
	if command == "" {
		return errors.New("command is empty")
	}
	return s.manager.SendCommand(id, []byte(command+"\n"))
}

func baseCollectorConfig(dto DeviceConfigDTO) collector.DeviceConfig {
	readTimeout, writeTimeout, idleGap := time.Duration(dto.ReadTimeoutMs)*time.Millisecond, time.Duration(dto.WriteTimeoutMs)*time.Millisecond, time.Duration(dto.IdleGapMs)*time.Millisecond
	if readTimeout <= 0 {
		readTimeout = 200 * time.Millisecond
	}
	if writeTimeout <= 0 {
		writeTimeout = time.Second
	}
	if idleGap <= 0 {
		idleGap = 10 * time.Millisecond
	}
	maxFrameBytes := dto.MaxFrameBytes
	if maxFrameBytes <= 0 {
		maxFrameBytes = 10 * 1024
	}
	return collector.DeviceConfig{ID: dto.DeviceID, Name: dto.Name, Serial: serialagent.SerialConfig{PortName: dto.PortName, BaudRate: dto.BaudRate, DataBits: dto.DataBits, StopBits: dto.StopBits, Parity: serialagent.Parity(dto.Parity), Handshake: serialagent.HandshakeNone, DTR: dto.DTR, RTS: dto.RTS, ReadTimeout: readTimeout, WriteTimeout: writeTimeout, IdleGap: idleGap, MaxFrameBytes: maxFrameBytes, Encoding: serialagent.Encoding(dto.Encoding)}}
}

func (s *Service) toCollectorConfig(dto DeviceConfigDTO) collector.DeviceConfig {
	s.mu.RLock()
	settings := s.appSettings
	s.mu.RUnlock()
	result := baseCollectorConfig(dto)
	result.Persist = dto.Configured && dto.SaveEnabled
	result.UploadEnabled = dto.Configured && dto.SaveEnabled && dto.UploadEnabled
	result.PreviewOnly = !result.Persist
	result.LocalOnly = !result.UploadEnabled
	result.ProjectID = dto.ProjectID
	result.ProjectName = dto.ProjectName
	result.Version = dto.Version
	result.TestTaskID = dto.TestTaskID
	result.TestTaskName = dto.TestTaskName
	result.UploaderName = dto.UploaderName
	result.UploaderEmail = dto.UploaderEmail
	result.Remark = dto.Remark
	result.ScenarioIDs = append([]string(nil), dto.ScenarioIDs...)
	result.CollectorVersion = settings.ProgramVersion
	result.Timezone = localTimezoneName()
	result.UploadSessionID = dto.UploadSessionID
	result.QueryCode = dto.QueryCode
	result.ConfigSnapshot = dto.ConfigSnapshot
	result.Rules = s.catalogRulesFor(dto)
	result.ReconnectMode = collector.ReconnectDisabled
	result.NoLogTimeout = durationSeconds(dto.NoLogTimeoutSeconds)
	result.MaxAge = durationSeconds(settings.SegmentMaxAgeSeconds)
	result.MaxBytes = settings.SegmentMaxBytes
	return result
}

func normalizeDeviceConfig(dto DeviceConfigDTO) DeviceConfigDTO {
	dto.PortName = strings.TrimSpace(dto.PortName)
	dto.ProjectID = strings.TrimSpace(dto.ProjectID)
	dto.ProjectName = strings.TrimSpace(dto.ProjectName)
	dto.Version = strings.TrimSpace(dto.Version)
	dto.TestTaskID = strings.TrimSpace(dto.TestTaskID)
	dto.TestTaskName = strings.TrimSpace(dto.TestTaskName)
	dto.UploaderName = strings.TrimSpace(dto.UploaderName)
	dto.UploaderEmail = strings.ToLower(strings.TrimSpace(dto.UploaderEmail))
	dto.Remark = strings.TrimSpace(dto.Remark)
	legacyID := strings.HasPrefix(dto.DeviceID, "DUT-")
	if (dto.DeviceID == "" || legacyID) && dto.PortName != "" {
		dto.DeviceID = serialDeviceID(dto.PortName, 0)
	}
	if isLegacyChannelName(dto.Name) {
		dto.Name = defaultSerialChannelName(dto.PortName, 1)
	}
	if dto.BaudRate == 0 {
		dto.BaudRate = 115200
	}
	if dto.DataBits == 0 {
		dto.DataBits = 8
	}
	if dto.StopBits == 0 {
		dto.StopBits = 1
	}
	if dto.ReadTimeoutMs <= 0 {
		dto.ReadTimeoutMs = 200
	}
	if dto.WriteTimeoutMs <= 0 {
		dto.WriteTimeoutMs = 1000
	}
	if dto.IdleGapMs <= 0 {
		dto.IdleGapMs = 10
	}
	if dto.MaxFrameBytes <= 0 {
		dto.MaxFrameBytes = 10 * 1024
	}
	if dto.Parity == "" {
		dto.Parity = "none"
	}
	if dto.Handshake == "" {
		dto.Handshake = "none"
	}
	if dto.Encoding == "" {
		dto.Encoding = "utf-8"
	}
	if dto.NoLogTimeoutSeconds <= 0 {
		dto.NoLogTimeoutSeconds = 300
	}
	return dto
}

func localTimezoneName() string {
	name := time.Now().Location().String()
	if name == "Local" {
		return ""
	}
	return name
}

func isLegacyChannelName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "设备通道" {
		return true
	}
	if !strings.HasPrefix(name, "设备通道 ") {
		return false
	}
	_, err := strconv.Atoi(strings.TrimPrefix(name, "设备通道 "))
	return err == nil
}

func serialDeviceID(portName string, fallback int) string {
	name := strings.ToUpper(strings.TrimSpace(portName))
	if name != "" {
		return name
	}
	return fmt.Sprintf("SERIAL-%02d", fallback+1)
}

func defaultSerialChannelName(portName string, index int) string {
	name := strings.TrimSpace(portName)
	if name != "" {
		return "串口 " + name
	}
	return fmt.Sprintf("串口通道 %d", index)
}

func portSignature(ports []PortInfo) string {
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, strings.ToUpper(strings.TrimSpace(port.Name))+"|"+port.VID+"|"+port.PID+"|"+port.USBSerial+"|"+port.Location)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func (s *Service) disconnectRemovedPorts(ports []PortInfo) {
	present := make(map[string]struct{}, len(ports))
	for _, port := range ports {
		present[strings.ToUpper(strings.TrimSpace(port.Name))] = struct{}{}
	}
	for _, state := range s.manager.GetDeviceStates() {
		if _, ok := present[strings.ToUpper(strings.TrimSpace(state.PortName))]; !ok {
			_ = s.manager.DisconnectDevice(state.DeviceID)
		}
	}
}

func (s *Service) loadSettings() error {
	sourcePath := s.configPath
	data, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		backup := s.configPath + ".bak"
		data, err = os.ReadFile(backup)
		sourcePath = backup
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read desktop settings: %w", err)
	}
	var settings desktopSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		broken := s.configPath + "." + time.Now().Format("20060102-150405") + ".broken"
		if renameErr := os.Rename(sourcePath, broken); renameErr != nil && !errors.Is(renameErr, os.ErrNotExist) {
			return fmt.Errorf("backup broken desktop settings: %w", renameErr)
		}
		if s.logger != nil {
			s.logger.Error("desktop settings are invalid; defaults restored", "component", "desktop.settings", "backup", broken, "error", err)
		}
		return nil
	}
	for _, value := range settings.Devices {
		value = normalizeDeviceConfig(value)
		if strings.TrimSpace(value.USBSerial) == "" && (value.Configured || strings.TrimSpace(value.ProjectID) != "" || strings.TrimSpace(value.UploaderEmail) != "" || strings.TrimSpace(value.Remark) != "") {
			if s.previousConfigs == nil {
				s.previousConfigs = map[string]DeviceConfigDTO{}
			}
			s.previousConfigs[strings.ToUpper(strings.TrimSpace(value.PortName))] = value
		}
		if strings.TrimSpace(value.USBSerial) == "" {
			value = serialOnlyInheritedConfig(value, PortInfo{Name: value.PortName, VID: value.VID, PID: value.PID, Location: value.Location}, 0)
		}
		if value.UploadEnabled && (value.UploadSessionID == "" || value.QueryCode == "" || value.ConfigSnapshot == "") {
			value.UploadSetupState = "pending"
			value.UploadSessionID, value.QueryCode = "", ""
		}
		key := value.DeviceID
		if key == "" {
			key = fmt.Sprintf("persisted-%d", len(s.configs)+1)
		}
		s.configs[key] = value
	}
	return nil
}

func (s *Service) saveSettings() error {
	s.mu.RLock()
	byIdentity := make(map[string]DeviceConfigDTO, len(s.configs))
	for _, value := range s.configs {
		key := strings.ToUpper(strings.TrimSpace(value.PortName))
		if value.USBSerial != "" {
			key = strings.ToUpper(strings.TrimSpace(value.VID)) + "|" + strings.ToUpper(strings.TrimSpace(value.PID)) + "|" + strings.ToUpper(strings.TrimSpace(value.USBSerial))
		}
		byIdentity[key] = value
	}
	devices := make([]DeviceConfigDTO, 0, len(byIdentity))
	for _, value := range byIdentity {
		devices = append(devices, value)
	}
	s.mu.RUnlock()
	sort.Slice(devices, func(i, j int) bool { return naturalPortLess(devices[i].PortName, devices[j].PortName) })
	data, err := json.MarshalIndent(desktopSettings{Version: 1, Devices: devices}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode desktop settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o755); err != nil {
		return err
	}
	temporary := s.configPath + ".tmp"
	backup := s.configPath + ".bak"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write desktop settings: %w", err)
	}
	_ = os.Remove(backup)
	if err := os.Rename(s.configPath, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(temporary)
		return fmt.Errorf("backup desktop settings: %w", err)
	}
	if err := os.Rename(temporary, s.configPath); err != nil {
		_ = os.Rename(backup, s.configPath)
		return fmt.Errorf("commit desktop settings: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.Type().IsRegular() {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func formatBytes(value int64) string {
	const unit int64 = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := unit, int64(0)
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
