package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"logmaster-agent/agent/internal/segment"
	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

var errDiskThreshold = errors.New("spool disk threshold reached")

type Manager struct {
	cfg       Config
	store     *spool.Store
	discovery Discovery
	factory   PortFactory
	broker    *Broker
	disk      *diskGuard

	mu         sync.RWMutex
	devices    map[string]*deviceRuntime
	connecting map[string]struct{}
	taskID     string
	closed     bool
}

// ApplyRuntimeConfig updates limits used by future and active channels.
func (m *Manager) ApplyRuntimeConfig(directory string, maxAge time.Duration, maxBytes int64, maxDiskBytes int64, warningPercent int) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg.SpoolDirectory = directory
	if maxDiskBytes > 0 {
		m.cfg.MaxDiskBytes = maxDiskBytes
		m.disk.mu.Lock()
		m.disk.directory = directory
		m.disk.limit = maxDiskBytes
		if warningPercent >= 1 && warningPercent <= 99 {
			m.disk.warningPercent = warningPercent
		}
		m.disk.nextCheck = time.Time{}
		m.disk.mu.Unlock()
	}
	for _, device := range m.devices {
		device.mu.Lock()
		device.config.MaxAge = maxAge
		device.config.MaxBytes = maxBytes
		device.mu.Unlock()
		device.writerMu.Lock()
		if device.writer != nil {
			if err := device.writer.ApplyConfig(context.Background(), directory, maxAge, maxBytes); err != nil {
				device.writerMu.Unlock()
				m.mu.Unlock()
				return err
			}
		}
		device.writerMu.Unlock()
	}
	m.mu.Unlock()
	return nil
}

type deviceRuntime struct {
	manager  *Manager
	config   DeviceConfig
	rules    []*compiledRule
	writer   *segment.Writer
	decoder  *serialagent.Decoder
	writerMu sync.Mutex

	mu          sync.RWMutex
	state       State
	lastErr     string
	taskID      string
	port        serialagent.Port
	cancel      context.CancelFunc
	done        chan struct{}
	dropped     uint64
	lines       uint64
	reconnects  uint64
	lastLogAt   time.Time
	connectedAt time.Time
	noLogAlert  bool
}

func New(cfg Config, store *spool.Store, discovery Discovery, factory PortFactory) (*Manager, error) {
	if store == nil {
		return nil, errors.New("collector spool store is required")
	}
	if strings.TrimSpace(cfg.SpoolDirectory) == "" {
		return nil, errors.New("collector spool directory is required")
	}
	if cfg.MaxDevices <= 0 {
		cfg.MaxDevices = MaxSupportedDevices
	}
	if cfg.MaxDevices > MaxSupportedDevices {
		return nil, fmt.Errorf("collector max devices cannot exceed %d", MaxSupportedDevices)
	}
	if discovery == nil {
		discovery = serialagent.SystemDiscovery{}
	}
	if factory == nil {
		factory = serialagent.GoBugFactory{}
	}
	if err := os.MkdirAll(cfg.SpoolDirectory, 0o755); err != nil {
		return nil, err
	}
	return &Manager{
		cfg: cfg, store: store, discovery: discovery, factory: factory,
		broker: NewBroker(cfg.EventCapacity), disk: newDiskGuard(cfg.SpoolDirectory, cfg.MaxDiskBytes, cfg.StorageWarningPercent, cfg.DiskCheckEvery),
		devices: make(map[string]*deviceRuntime), connecting: make(map[string]struct{}),
	}, nil
}

func (m *Manager) ScanPorts(ctx context.Context) ([]serialagent.PortDescriptor, error) {
	return m.discovery.List(ctx)
}

func (m *Manager) SubscribeLogEvents() (<-chan Event, func()) { return m.broker.Subscribe() }

func (m *Manager) RecoverDevice(ctx context.Context, config DeviceConfig) (int, error) {
	if strings.TrimSpace(config.ID) == "" || strings.TrimSpace(config.Serial.PortName) == "" {
		return 0, errors.New("device id and serial port are required for recovery")
	}
	if !config.persistEnabled() {
		return 0, nil
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 5 * time.Minute
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 32 << 20
	}
	return segment.Recover(ctx, segment.Config{Directory: m.cfg.SpoolDirectory, DeviceSN: config.ID, PortName: config.Serial.PortName, SessionStart: time.Now().UTC(), MaxAge: config.MaxAge, MaxBytes: config.MaxBytes}, m.deliver(config))
}

func (m *Manager) ConnectDevice(config DeviceConfig) error {
	config.ID = strings.TrimSpace(config.ID)
	if config.ID == "" {
		return errors.New("device id is required")
	}
	if err := config.Serial.Validate(); err != nil {
		return err
	}
	if config.MaxAge <= 0 {
		config.MaxAge = 5 * time.Minute
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 32 << 20
	}
	rules, err := compileRules(config.Rules)
	if err != nil {
		return fmt.Errorf("compile device rules: %w", err)
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("collector manager is closed")
	}
	if existing := m.devices[config.ID]; existing != nil && existing.running() {
		m.mu.Unlock()
		return fmt.Errorf("device %s is already connected", config.ID)
	}
	if _, exists := m.connecting[config.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("device %s is connecting", config.ID)
	}
	active := len(m.connecting)
	for id, runtime := range m.devices {
		if id != config.ID && runtime.running() {
			active++
		}
	}
	if active >= m.cfg.MaxDevices {
		m.mu.Unlock()
		return fmt.Errorf("collector device limit %d reached", m.cfg.MaxDevices)
	}
	m.connecting[config.ID] = struct{}{}
	taskID := m.taskID
	m.mu.Unlock()
	reserved := true
	defer func() {
		if reserved {
			m.mu.Lock()
			delete(m.connecting, config.ID)
			m.mu.Unlock()
		}
	}()

	var writer *segment.Writer
	if config.persistEnabled() {
		if config.StartedAt.IsZero() {
			config.StartedAt = time.Now().UTC()
		}
		sessionID, sessionErr := m.store.StartSession(context.Background(), sessionFromConfig(config))
		if sessionErr != nil {
			return fmt.Errorf("start collection session: %w", sessionErr)
		}
		config.SessionID = sessionID
		var err error
		writer, err = m.newWriter(config)
		if err != nil {
			return err
		}
	}
	decoder, err := serialagent.NewDecoderWithLimit(config.Serial.Encoding, config.Serial.MaxFrameBytes)
	if err != nil {
		if writer != nil {
			_ = writer.Close(context.Background())
		}
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	port, err := m.factory.Open(ctx, config.Serial)
	if err != nil {
		cancel()
		if writer != nil {
			_ = writer.Close(context.Background())
		}
		return fmt.Errorf("open serial port %s: %w", config.Serial.PortName, err)
	}
	now := time.Now().UTC()
	runtime := &deviceRuntime{manager: m, config: config, rules: rules, writer: writer, decoder: decoder, state: StateCollecting, taskID: taskID, port: port, cancel: cancel, done: make(chan struct{}), connectedAt: now}
	m.mu.Lock()
	delete(m.connecting, config.ID)
	if m.closed {
		m.mu.Unlock()
		cancel()
		_ = port.Close()
		if writer != nil {
			_ = writer.Close(context.Background())
		}
		return errors.New("collector manager is closed")
	}
	m.devices[config.ID] = runtime
	m.mu.Unlock()
	reserved = false
	runtime.publish(Event{State: StateCollecting})
	go runtime.run(ctx, port)
	return nil
}

func (m *Manager) newWriter(config DeviceConfig) (*segment.Writer, error) {
	deliver := m.deliver(config)
	segmentConfig := segment.Config{Directory: m.cfg.SpoolDirectory, DeviceSN: config.ID, PortName: config.Serial.PortName, SessionStart: time.Now().UTC(), MaxAge: config.MaxAge, MaxBytes: config.MaxBytes}
	if _, err := segment.Recover(context.Background(), segmentConfig, deliver); err != nil {
		return nil, fmt.Errorf("recover device segments: %w", err)
	}
	return segment.NewWriter(segmentConfig, deliver)
}

func (m *Manager) DisconnectDevice(deviceID string) error {
	m.mu.RLock()
	runtime := m.devices[deviceID]
	m.mu.RUnlock()
	if runtime == nil {
		return fmt.Errorf("device %s is not configured", deviceID)
	}
	runtime.stop()
	m.mu.Lock()
	if m.devices[deviceID] == runtime {
		delete(m.devices, deviceID)
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) deliver(config DeviceConfig) segment.Deliver {
	return func(ctx context.Context, completed segment.Completed) error {
		logFileID, err := m.store.RegisterLogFile(ctx, sessionFromConfig(config), spool.LogFileRecord{SessionID: config.SessionID, Path: completed.Path, DeviceSN: config.ID, PortName: config.Serial.PortName, FirstSequence: completed.FirstSequence, LastSequence: completed.LastSequence, SizeBytes: completed.SizeBytes, SHA256: completed.SHA256, UploadEligible: config.uploadEnabled(), CreatedAt: completed.CreatedAt, CompletedAt: completed.CompletedAt})
		if err != nil {
			return err
		}
		if !config.uploadEnabled() {
			return nil
		}
		project, version := config.ProjectName, config.Version
		if strings.TrimSpace(project) == "" {
			project = m.cfg.ProjectName
		}
		if strings.TrimSpace(version) == "" {
			version = m.cfg.Version
		}
		createdAt := config.StartedAt
		metadata := spool.UploadMetadata{ProjectID: config.ProjectID, ProjectName: project, Version: version, TestTaskID: config.TestTaskID, TestTaskName: config.TestTaskName, UploaderName: config.UploaderName, UploaderEmail: config.UploaderEmail, Remark: config.Remark, CollectorVersion: config.CollectorVersion, Timezone: config.Timezone, CreatedAt: &createdAt, StartedAt: &createdAt, ScenarioIDs: append([]string(nil), config.ScenarioIDs...), UploadSessionID: config.UploadSessionID, QueryCode: config.QueryCode, ConfigSnapshot: config.ConfigSnapshot}
		_, err = m.store.EnqueueFileWithMetadata(ctx, metadata, spool.File{LogFileID: logFileID, SessionID: config.SessionID, Path: completed.Path, SHA256: completed.SHA256, SizeBytes: completed.SizeBytes, DeviceSN: config.ID, FirstSequence: completed.FirstSequence, LastSequence: completed.LastSequence})
		return err
	}
}

func sessionFromConfig(config DeviceConfig) spool.Session {
	return spool.Session{ID: config.SessionID, DeviceSN: config.ID, PortName: config.Serial.PortName, ProjectID: config.ProjectID, ProjectName: config.ProjectName, Version: config.Version, TestTaskID: config.TestTaskID, TestTaskName: config.TestTaskName, UploaderName: config.UploaderName, UploaderEmail: config.UploaderEmail, Remark: config.Remark, ScenarioIDs: append([]string(nil), config.ScenarioIDs...), CollectorVersion: config.CollectorVersion, Timezone: config.Timezone, SaveEnabled: true, UploadEnabled: config.uploadEnabled(), StartedAt: config.StartedAt}
}

func (m *Manager) UpdateDeviceConfig(deviceID string, config DeviceConfig) error {
	m.mu.RLock()
	runtime := m.devices[deviceID]
	m.mu.RUnlock()
	if runtime == nil {
		return fmt.Errorf("device %s is not configured", deviceID)
	}
	config.ID = deviceID
	wasRunning := runtime.running()
	if !wasRunning {
		rules, err := compileRules(config.Rules)
		if err != nil {
			return err
		}
		runtime.mu.Lock()
		runtime.config, runtime.rules = config, rules
		runtime.mu.Unlock()
		return nil
	}
	runtime.mu.RLock()
	serialUnchanged := runtime.config.Serial == config.Serial
	runtime.mu.RUnlock()
	if !serialUnchanged {
		runtime.stop()
		return m.ConnectDevice(config)
	}
	rules, err := compileRules(config.Rules)
	if err != nil {
		return err
	}
	runtime.mu.RLock()
	oldSessionID := runtime.config.SessionID
	runtime.mu.RUnlock()
	runtime.writerMu.Lock()
	if runtime.writer != nil {
		_ = runtime.writer.Close(context.Background())
		runtime.writer = nil
	}
	if config.persistEnabled() {
		config.StartedAt = time.Now().UTC()
		sessionID, sessionErr := m.store.StartSession(context.Background(), sessionFromConfig(config))
		if sessionErr != nil {
			runtime.writerMu.Unlock()
			return sessionErr
		}
		config.SessionID = sessionID
		writer, createErr := m.newWriter(config)
		if createErr != nil {
			runtime.writerMu.Unlock()
			return createErr
		}
		runtime.writer = writer
	}
	runtime.writerMu.Unlock()
	_ = m.store.EndSession(context.Background(), oldSessionID, time.Now().UTC())
	runtime.mu.Lock()
	runtime.config, runtime.rules = config, rules
	runtime.mu.Unlock()
	return nil
}

func (m *Manager) StartTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id is required")
	}
	m.mu.Lock()
	m.taskID = taskID
	for _, runtime := range m.devices {
		runtime.mu.Lock()
		runtime.taskID = taskID
		runtime.mu.Unlock()
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) StopTask(taskID string) error {
	m.mu.Lock()
	if taskID != "" && m.taskID != taskID {
		m.mu.Unlock()
		return fmt.Errorf("task %s is not active", taskID)
	}
	m.taskID = ""
	runtimes := make([]*deviceRuntime, 0, len(m.devices))
	for _, runtime := range m.devices {
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	for _, runtime := range runtimes {
		runtime.stop()
	}
	return nil
}

func (m *Manager) SendCommand(deviceID string, command []byte) error {
	m.mu.RLock()
	runtime := m.devices[deviceID]
	m.mu.RUnlock()
	if runtime == nil {
		return fmt.Errorf("device %s is not configured", deviceID)
	}
	runtime.mu.RLock()
	port := runtime.port
	runtime.mu.RUnlock()
	if port == nil {
		return fmt.Errorf("device %s is not connected", deviceID)
	}
	written, err := port.Write(command)
	if err != nil {
		return err
	}
	if written != len(command) {
		return io.ErrShortWrite
	}
	return nil
}

func (m *Manager) RotateDevice(deviceID string) error {
	m.mu.RLock()
	runtime := m.devices[deviceID]
	m.mu.RUnlock()
	if runtime == nil {
		return fmt.Errorf("device %s is not configured", deviceID)
	}
	runtime.writerMu.Lock()
	defer runtime.writerMu.Unlock()
	if runtime.writer == nil {
		return nil
	}
	return runtime.writer.Rotate(context.Background())
}

func (m *Manager) GetDeviceStates() []DeviceState {
	m.mu.RLock()
	runtimes := make([]*deviceRuntime, 0, len(m.devices))
	for _, runtime := range m.devices {
		runtimes = append(runtimes, runtime)
	}
	m.mu.RUnlock()
	states := make([]DeviceState, 0, len(runtimes))
	for _, runtime := range runtimes {
		states = append(states, runtime.snapshot())
	}
	return states
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	runtimes := make([]*deviceRuntime, 0, len(m.devices))
	for _, runtime := range m.devices {
		runtimes = append(runtimes, runtime)
	}
	m.mu.Unlock()
	for _, runtime := range runtimes {
		runtime.stop()
	}
	return nil
}

func (d *deviceRuntime) run(ctx context.Context, initialPort serialagent.Port) {
	defer close(d.done)
	defer func() {
		d.mu.RLock()
		sessionID := d.config.SessionID
		d.mu.RUnlock()
		_ = d.manager.store.EndSession(context.Background(), sessionID, time.Now().UTC())
	}()
	defer func() {
		if line, ok := d.decoder.Flush(time.Now().UTC()); ok {
			_ = d.handleLine(context.Background(), line)
		}
		d.writerMu.Lock()
		if d.writer != nil {
			_ = d.writer.Close(context.Background())
		}
		d.writerMu.Unlock()
		d.mu.RLock()
		state := d.state
		d.mu.RUnlock()
		if state != StateDiskFull && state != StateError {
			d.setState(StateDisconnected, "")
		}
	}()
	rotateDone := make(chan struct{})
	go func() {
		interval := d.config.MaxAge
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(rotateDone)
		noLogTick := time.NewTicker(time.Second)
		defer noLogTick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.writerMu.Lock()
				var err error
				if d.writer != nil {
					err = d.writer.Rotate(context.Background())
				}
				d.writerMu.Unlock()
				if err != nil {
					d.publish(Event{Error: err.Error()})
				}
			case <-noLogTick.C:
				d.checkNoLog(time.Now().UTC())
			}
		}
	}()
	reconnect := serialagent.NewReconnectManager(d.manager.cfg.Reconnect)
	port := initialPort
	first := true
	for ctx.Err() == nil {
		if !first {
			d.mu.Lock()
			d.reconnects++
			d.mu.Unlock()
			d.setState(StateReconnecting, d.lastErr)
			opened, openErr := d.manager.factory.Open(ctx, d.config.Serial)
			if openErr != nil {
				d.setState(StateError, errorText(openErr))
				break
			}
			port = opened
			d.mu.Lock()
			d.port = port
			d.connectedAt = time.Now().UTC()
			d.mu.Unlock()
			d.setState(StateCollecting, "")
		}
		started := time.Now()
		err := d.readLoop(ctx, port)
		_ = port.Close()
		d.mu.Lock()
		if d.port == port {
			d.port = nil
		}
		d.mu.Unlock()
		if err != nil {
			if recoverErr := d.recoverSegments(context.Background()); recoverErr != nil {
				err = errors.Join(err, recoverErr)
			}
		}
		if ctx.Err() != nil {
			break
		}
		if errors.Is(err, errDiskThreshold) {
			d.setState(StateDiskFull, err.Error())
			break
		}
		if d.config.ReconnectMode == ReconnectDisabled {
			d.setState(StateError, errorText(err))
			break
		}
		d.mu.Lock()
		d.lastErr = errorText(err)
		d.mu.Unlock()
		delay := reconnect.FailureDelay(time.Since(started))
		d.setState(StateReconnecting, errorText(err))
		select {
		case <-ctx.Done():
		case <-time.After(delay):
		}
		first = false
	}
	<-rotateDone
}

func (d *deviceRuntime) recoverSegments(ctx context.Context) error {
	d.writerMu.Lock()
	var rotateErr error
	if d.writer != nil {
		rotateErr = d.writer.Rotate(ctx)
	}
	d.writerMu.Unlock()
	d.mu.RLock()
	config := d.config
	d.mu.RUnlock()
	_, recoverErr := d.manager.RecoverDevice(ctx, config)
	return errors.Join(rotateErr, recoverErr)
}

func (d *deviceRuntime) readLoop(ctx context.Context, port serialagent.Port) error {
	framer := serialagent.NewIdleGapFramer(d.config.Serial.IdleGap, d.config.Serial.MaxFrameBytes)
	buffer := make([]byte, 4096)
	defer func() {
		if frame, ok := framer.Flush(); ok {
			_ = d.handleFrame(context.Background(), frame, time.Now().UTC())
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		n, readErr := port.Read(buffer)
		now := time.Now().UTC()
		if n < 0 || n > len(buffer) {
			return fmt.Errorf("invalid serial read count %d", n)
		}
		if n > 0 {
			for _, frame := range framer.Push(now, buffer[:n]) {
				if err := d.handleFrame(ctx, frame, now); err != nil {
					return err
				}
			}
		}
		if frame, ok := framer.FlushIfIdle(now); ok {
			if err := d.handleFrame(ctx, frame, now); err != nil {
				return err
			}
		}
		if readErr != nil && !serialagent.IsReadTimeout(readErr) {
			return readErr
		}
	}
}

func (d *deviceRuntime) handleFrame(ctx context.Context, frame []byte, at time.Time) error {
	for _, line := range d.decoder.PushFrame(frame, at) {
		if err := d.handleLine(ctx, line); err != nil {
			return err
		}
	}
	return nil
}

func (d *deviceRuntime) handleLine(ctx context.Context, line serialagent.DecodedLine) error {
	d.mu.Lock()
	d.lines++
	sequence := int64(d.lines)
	persist := d.config.persistEnabled()
	d.lastLogAt = line.CapturedAt
	alertChanged := d.noLogAlert
	d.noLogAlert = false
	d.mu.Unlock()
	if persist {
		var err error
		diskState, measureErr := d.manager.disk.State(time.Now())
		if measureErr != nil {
			// Measurement failures are observable but are not a reason to tear
			// down the serial connection. Continue with the last safe state.
			d.publish(Event{Error: fmt.Sprintf("check spool disk usage: %v", measureErr)})
		}
		switch diskState {
		case DiskFull:
			d.setState(StateDiskFull, "spool disk threshold reached")
			return errDiskThreshold
		case DiskReadOnly:
			d.setState(StateDiskReadOnly, "spool disk usage is above 90%; local persistence paused")
			persist = false
		case DiskWarning:
			d.setState(StateDiskWarning, "spool disk usage is above 80%")
		case DiskNormal:
			// Do not overwrite an active terminal state, but clear a previous
			// warning once the queue has recovered.
			d.mu.RLock()
			current := d.state
			d.mu.RUnlock()
			if current == StateDiskWarning || current == StateDiskReadOnly {
				d.setState(StateCollecting, "")
			}
		}
		sequence, err = d.manager.store.NextSequence(ctx, d.config.ID)
		if err != nil {
			return err
		}
		d.writerMu.Lock()
		writer := d.writer
		if writer != nil {
			err = writer.Write(ctx, segment.Entry{Sequence: sequence, CapturedAt: line.CapturedAt, Text: line.Text})
		}
		d.writerMu.Unlock()
		if err != nil {
			return err
		}
	}
	hits := make([]RuleHit, 0)
	for _, rule := range d.rules {
		if rule.match(line.Text) {
			count := rule.count.Add(1)
			hits = append(hits, RuleHit{RuleID: rule.rule.ID, RuleName: rule.rule.Name, Severity: rule.rule.Severity, Module: rule.rule.Module, Count: count})
		}
	}
	d.publish(Event{CapturedAt: line.CapturedAt, Text: line.Text, Sequence: sequence, Hits: hits})
	if alertChanged {
		d.publish(Event{State: StateCollecting})
	}
	return nil
}

func (d *deviceRuntime) checkNoLog(now time.Time) {
	d.mu.Lock()
	timeout := d.config.NoLogTimeout
	if timeout <= 0 || d.state != StateCollecting {
		d.mu.Unlock()
		return
	}
	base := d.connectedAt
	if !d.lastLogAt.IsZero() {
		base = d.lastLogAt
	}
	alert := !base.IsZero() && now.Sub(base) >= timeout
	changed := alert != d.noLogAlert
	d.noLogAlert = alert
	d.mu.Unlock()
	if changed {
		d.publish(Event{State: StateCollecting})
	}
}

func (d *deviceRuntime) publish(event Event) {
	d.mu.RLock()
	event.DeviceID, event.DeviceName, event.TaskID, event.SessionID = d.config.ID, d.config.Name, d.taskID, d.config.SessionID
	d.mu.RUnlock()
	if event.CapturedAt.IsZero() {
		event.CapturedAt = time.Now().UTC()
	}
	if dropped := d.manager.broker.Publish(event); dropped > 0 {
		d.mu.Lock()
		d.dropped += dropped
		d.mu.Unlock()
	}
}

func (d *deviceRuntime) setState(state State, message string) {
	d.mu.Lock()
	d.state, d.lastErr = state, message
	d.mu.Unlock()
	d.publish(Event{State: state, Error: message})
}

func (d *deviceRuntime) running() bool {
	d.mu.RLock()
	cancel, done := d.cancel, d.done
	d.mu.RUnlock()
	if cancel == nil || done == nil {
		return false
	}
	select {
	case <-done:
		return false
	default:
		return true
	}
}

func (d *deviceRuntime) stop() {
	d.mu.Lock()
	cancel, done, port := d.cancel, d.done, d.port
	d.cancel = nil
	d.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if port != nil {
		_ = port.Close()
	}
	<-done
	d.mu.RLock()
	sessionID := d.config.SessionID
	d.mu.RUnlock()
	_ = d.manager.store.EndSession(context.Background(), sessionID, time.Now().UTC())
}

func (d *deviceRuntime) snapshot() DeviceState {
	d.mu.RLock()
	state := DeviceState{DeviceID: d.config.ID, DeviceName: d.config.Name, PortName: d.config.Serial.PortName, TaskID: d.taskID, State: d.state, LastError: d.lastErr, DroppedEvents: d.dropped, LinesReceived: d.lines, Reconnects: d.reconnects, RuleCounts: make(map[string]uint64, len(d.rules)), Enabled: d.running(), Persisting: d.config.persistEnabled(), UploadEnabled: d.config.uploadEnabled(), NoLogAlert: d.noLogAlert, SessionID: d.config.SessionID}
	if !d.lastLogAt.IsZero() {
		value := d.lastLogAt
		state.LastLogAt = &value
	}
	rules := append([]*compiledRule(nil), d.rules...)
	d.mu.RUnlock()
	for _, rule := range rules {
		state.RuleCounts[rule.rule.Name] = rule.count.Load()
	}
	return state
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
