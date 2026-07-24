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
	ctx        context.Context
	cancel     context.CancelFunc
	manager    *collector.Manager
	store      *spool.Store
	worker     *backend.Worker
	close      sync.Once
	mu         sync.RWMutex
	configs    map[string]DeviceConfigDTO
	configPath string
	spoolDirectory string
	maxDiskBytes int64
	workerCancel context.CancelFunc
	workerCtx context.Context
	workerDone chan struct{}
	workerStarted chan struct{}
	workerStart sync.Once
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
	DeviceID  string `json:"deviceId"`
	Name      string `json:"name"`
	PortName  string `json:"portName"`
	BaudRate  int    `json:"baudRate"`
	DataBits  int    `json:"dataBits"`
	StopBits  int    `json:"stopBits"`
	Parity    string `json:"parity"`
	Handshake string `json:"handshake"`
	DTR       bool   `json:"dtr"`
	RTS       bool   `json:"rts"`
}
type DeviceStateDTO struct {
	DeviceID      string            `json:"deviceId"`
	Name          string            `json:"name"`
	PortName      string            `json:"portName"`
	Status        string            `json:"status"`
	LastError     string            `json:"lastError,omitempty"`
	DroppedEvents uint64            `json:"droppedEvents"`
	LinesReceived uint64            `json:"linesReceived"`
	Reconnects    uint64            `json:"reconnects"`
	RuleCounts    map[string]uint64 `json:"ruleCounts"`
	Config        DeviceConfigDTO   `json:"config"`
}
type UploadBatchDTO struct {
	ID           string    `json:"id"`
	State        string    `json:"state"`
	DeviceID     string    `json:"deviceId"`
	FileName     string    `json:"fileName"`
	SizeBytes    int64     `json:"sizeBytes"`
	SHA256       string    `json:"sha256"`
	AttemptCount int       `json:"attemptCount"`
	LastError    string    `json:"lastError"`
	CreatedAt    time.Time `json:"createdAt"`
}
type desktopSettings struct {
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
type LogRow struct {
	DeviceID   string              `json:"deviceId"`
	DeviceName string              `json:"deviceName"`
	Timestamp  string              `json:"timestamp"`
	Text       string              `json:"text"`
	Message    string              `json:"message"`
	Level      string              `json:"level"`
	Module     string              `json:"module"`
	CapturedAt time.Time           `json:"capturedAt"`
	Hits       []collector.RuleHit `json:"hits,omitempty"`
}

func NewService() (*Service, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return newServiceAt(filepath.Join(base, "LogMaster"))
}

func newServiceAt(root string) (*Service, error) {
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	store, err := spool.Open(filepath.Join(dataDir, "collector.db"))
	if err != nil {
		return nil, err
	}
	cfg := config.DefaultConfig()
	cfg.Agent.ID, cfg.Agent.Name = "desktop-collector", "LogMaster 采集端"
	cfg.Backend.BaseURL, cfg.Backend.ProjectName, cfg.Backend.Version = "http://127.0.0.1:8080/api", "default", "V1.0.0"
	cfg.Spool.Directory, cfg.Spool.SQLitePath = filepath.Join(dataDir, "spool"), filepath.Join(dataDir, "collector.db")
	cfg.Serial.Ports = make([]config.PortConfig, 4)
	for i := range cfg.Serial.Ports {
		cfg.Serial.Ports[i] = config.DefaultPortConfig()
		cfg.Serial.Ports[i].Enabled = false
		cfg.Serial.Ports[i].DeviceSN = fmt.Sprintf("DUT-%02d", i+1)
		cfg.Serial.Ports[i].PortName = fmt.Sprintf("COM%d", 10+i*2)
	}
	managerCfg := collector.Config{MaxDevices: 8, EventCapacity: 2048, SpoolDirectory: cfg.Spool.Directory, MaxDiskBytes: cfg.Spool.MaxDiskBytes, ProjectName: cfg.Backend.ProjectName, Version: cfg.Backend.Version, Reconnect: serialagent.ReconnectConfig{InitialDelay: cfg.Serial.Reconnect.InitialDelay, Multiplier: cfg.Serial.Reconnect.Multiplier, MaxDelay: cfg.Serial.Reconnect.MaxDelay, Jitter: cfg.Serial.Reconnect.Jitter}, DiskCheckEvery: 5 * time.Second}
	manager, err := collector.New(managerCfg, store, nil, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	client := backend.New(backend.Config{BaseURL: cfg.Backend.BaseURL, HealthPath: cfg.Backend.HealthPath, InspectPath: cfg.Backend.InspectPath, UploadPath: cfg.Backend.UploadPath, Timeout: cfg.Backend.RequestTimeout, Gzip: cfg.Backend.UploadGzip})
	worker := backend.NewWorker(backend.WorkerConfig{Interval: cfg.Backend.UploadInterval, MaxFiles: 16, Concurrency: cfg.Backend.UploadConcurrency, InspectBeforeUpload: false}, store, client, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	workerCtx, workerCancel := context.WithCancel(context.Background())
	configs := make(map[string]DeviceConfigDTO, 4)
	for i, port := range cfg.Serial.Ports {
		configs[port.DeviceSN] = DeviceConfigDTO{DeviceID: port.DeviceSN, Name: fmt.Sprintf("设备通道 %d", i+1), PortName: port.PortName, BaudRate: port.BaudRate, DataBits: port.DataBits, StopBits: port.StopBits, Parity: port.Parity, Handshake: port.Handshake, DTR: port.DTR, RTS: port.RTS}
	}
	service := &Service{manager: manager, store: store, worker: worker, ctx: ctx, cancel: cancel, configs: configs, configPath: filepath.Join(root, "desktop-config.json"), spoolDirectory:cfg.Spool.Directory, maxDiskBytes:cfg.Spool.MaxDiskBytes, workerCtx:workerCtx, workerCancel:workerCancel, workerDone:make(chan struct{}), workerStarted:make(chan struct{})}
	if err := service.loadSettings(); err != nil {
		manager.Close()
		store.Close()
		return nil, err
	}
	if _, err := store.Recover(context.Background(), 0); err != nil { manager.Close(); store.Close(); return nil, fmt.Errorf("recover upload queue: %w",err) }
	for _, dto := range service.configs {
		if _, err := manager.RecoverDevice(context.Background(), toCollectorConfig(dto)); err != nil { manager.Close(); store.Close(); return nil, fmt.Errorf("recover device %s: %w",dto.DeviceID,err) }
	}
	return service, nil
}

func (s *Service) startup(ctx context.Context) {
	s.ctx = ctx
	go s.pumpEvents()
	s.workerStart.Do(func(){ close(s.workerStarted); go func(){ defer close(s.workerDone); s.worker.Run(s.workerCtx) }() })
}
func (s *Service) shutdown() {
	s.close.Do(func() {
		s.cancel(); s.workerCancel(); _ = s.manager.Close()
		select { case <-s.workerStarted: <-s.workerDone; default: }
		_ = s.store.Close()
	})
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
			if !ok { return }
			if event.State != "" {
				runtime.EventsEmit(s.ctx, "collector:state", event)
				continue
			}
			level, module := "INFO", ""
			if len(event.Hits) > 0 {
				level, module = event.Hits[0].Severity, event.Hits[0].Module
			}
			batch = append(batch, LogRow{DeviceID: event.DeviceID, DeviceName: event.DeviceName, Timestamp: event.CapturedAt.Local().Format("15:04:05.000"), CapturedAt: event.CapturedAt, Text: event.Text, Message: event.Text, Level: level, Module: module, Hits: event.Hits})
			if len(batch) >= 128 { runtime.EventsEmit(s.ctx, "collector:logs", batch); batch = make([]LogRow,0,128) }
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
	s.mu.RLock(); _, ok := s.configs[deviceID]; s.mu.RUnlock()
	if !ok { return fmt.Errorf("device %s is not configured",deviceID) }
	return nil
}
func (s *Service) GetDeviceStates() []DeviceStateDTO {
	raw := s.manager.GetDeviceStates()
	byID := make(map[string]collector.DeviceState, len(raw))
	for _, state := range raw {
		byID[state.DeviceID] = state
	}
	s.mu.RLock()
	result := make([]DeviceStateDTO, 0, len(s.configs))
	for id, cfg := range s.configs {
		item := DeviceStateDTO{DeviceID: id, Name: cfg.Name, PortName: cfg.PortName, Status: "disconnected", Config: cfg, RuleCounts: map[string]uint64{}}
		if state, ok := byID[id]; ok {
			item.Name = state.DeviceName
			item.PortName = state.PortName
			item.Status = string(state.State)
			item.LastError = state.LastError
			item.DroppedEvents = state.DroppedEvents
			item.LinesReceived = state.LinesReceived
			item.Reconnects = state.Reconnects
			item.RuleCounts = state.RuleCounts
		}
		result = append(result, item)
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].DeviceID < result[j].DeviceID })
	return result
}
func (s *Service) GetUploadQueueStatus() (QueueStatus, error) {
	counts, err := s.store.Counts(s.ctx)
	if err != nil {
		return QueueStatus{}, err
	}
	used, err := directorySize(s.spoolDirectory); if err != nil { return QueueStatus{},err }
	percent := 0; if s.maxDiskBytes > 0 { percent = int((used*100)/s.maxDiskBytes); if percent>100 {percent=100} }
	return QueueStatus{Pending: counts[spool.Pending], Uploading: counts[spool.Uploading], Uploaded: counts[spool.Uploaded], Uncertain: counts[spool.Uncertain], Dead: counts[spool.Dead], DiskUsagePercent:percent, DiskUsageText:fmt.Sprintf("%s / %s",formatBytes(used),formatBytes(s.maxDiskBytes))}, nil
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
func (s *Service) ConfirmUncertain(id, uploadID, taskID string) error {
	if uploadID == "" || taskID == "" {
		return errors.New("upload id and task id are required")
	}
	return s.store.ConfirmUncertain(s.ctx, id, uploadID, taskID)
}
func (s *Service) ConnectDevice(dto DeviceConfigDTO) error {
	if dto.BaudRate == 0 {
		dto.BaudRate = 115200
	}
	if dto.DataBits == 0 {
		dto.DataBits = 8
	}
	if dto.StopBits == 0 {
		dto.StopBits = 1
	}
	if dto.Parity == "" {
		dto.Parity = "none"
	}
	config := toCollectorConfig(dto)
	if err := config.Serial.Validate(); err != nil { return err }
	if err := s.manager.ConnectDevice(config); err != nil { return err }
	s.mu.Lock(); s.configs[dto.DeviceID] = dto; s.mu.Unlock()
	return s.saveSettings()
}
func (s *Service) DisconnectDevice(id string) error { return s.manager.DisconnectDevice(id) }
func (s *Service) UpdateDeviceConfig(id string, dto DeviceConfigDTO) error {
	return s.SaveDeviceConfig(id,dto)
}
func (s *Service) StartTask(id string) error { return s.manager.StartTask(id) }
func (s *Service) StopTask(id string) error  { return s.manager.StopTask(id) }
func (s *Service) SendCommand(id, command string) error {
	if command == "" {
		return errors.New("command is empty")
	}
	return s.manager.SendCommand(id, []byte(command+"\n"))
}

func toCollectorConfig(dto DeviceConfigDTO) collector.DeviceConfig {
	return collector.DeviceConfig{ID: dto.DeviceID, Name: dto.Name, Serial: serialagent.SerialConfig{PortName: dto.PortName, BaudRate: dto.BaudRate, DataBits: dto.DataBits, StopBits: dto.StopBits, Parity: serialagent.Parity(dto.Parity), Handshake: serialagent.HandshakeNone, DTR: dto.DTR, RTS: dto.RTS, ReadTimeout: 200 * time.Millisecond, WriteTimeout: time.Second, IdleGap: 10 * time.Millisecond, MaxFrameBytes: 10 * 1024, Encoding: serialagent.EncodingUTF8}}
}

func (s *Service) loadSettings() error {
	data, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		return s.saveSettings()
	}
	if err != nil {
		return fmt.Errorf("read desktop settings: %w", err)
	}
	var settings desktopSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse desktop settings: %w", err)
	}
	if len(settings.Devices) > collector.MaxSupportedDevices {
		return errors.New("desktop settings exceed eight devices")
	}
	for _, device := range settings.Devices {
		if device.DeviceID == "" {
			return errors.New("desktop device id is required")
		}
		s.configs[device.DeviceID] = device
	}
	return nil
}

func (s *Service) saveSettings() error {
	s.mu.RLock()
	devices := make([]DeviceConfigDTO, 0, len(s.configs))
	for _, device := range s.configs {
		devices = append(devices, device)
	}
	s.mu.RUnlock()
	sort.Slice(devices, func(i, j int) bool { return devices[i].DeviceID < devices[j].DeviceID })
	data, err := json.MarshalIndent(desktopSettings{Devices: devices}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(s.configPath), "desktop-config-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = atomicReplace(tempPath, s.configPath); err != nil {
		return fmt.Errorf("save desktop settings: %w", err)
	}
	return nil
}

func directorySize(root string) (int64,error) {
	var total int64
	err := filepath.WalkDir(root,func(_ string,entry fs.DirEntry,err error) error { if err!=nil {if errors.Is(err,os.ErrNotExist){return nil};return err}; if entry.Type().IsRegular(){info,infoErr:=entry.Info();if infoErr!=nil{return infoErr};total+=info.Size()};return nil })
	return total,err
}

func formatBytes(value int64) string {
	const unit int64 = 1024
	if value < unit { return fmt.Sprintf("%d B",value) }
	div,exp := unit,int64(0); for n:=value/unit;n>=unit;n/=unit {div*=unit;exp++}
	return fmt.Sprintf("%.1f %ciB",float64(value)/float64(div),"KMGTPE"[exp])
}
