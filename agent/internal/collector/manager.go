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

	mu      sync.RWMutex
	devices map[string]*deviceRuntime
	connecting map[string]struct{}
	taskID  string
	closed  bool
}

type deviceRuntime struct {
	manager *Manager
	config  DeviceConfig
	rules   []*compiledRule
	writer  *segment.Writer
	decoder *serialagent.Decoder

	mu         sync.RWMutex
	state      State
	lastErr    string
	taskID     string
	port       serialagent.Port
	cancel     context.CancelFunc
	done       chan struct{}
	dropped    uint64
	lines      uint64
	reconnects uint64
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
		broker: NewBroker(cfg.EventCapacity), disk: newDiskGuard(cfg.SpoolDirectory, cfg.MaxDiskBytes, cfg.DiskCheckEvery),
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
	if config.MaxAge <= 0 { config.MaxAge = 5 * time.Minute }
	if config.MaxBytes <= 0 { config.MaxBytes = 32 << 20 }
	return segment.Recover(ctx, segment.Config{Directory:m.cfg.SpoolDirectory, DeviceSN:config.ID, PortName:config.Serial.PortName, SessionStart:time.Now().UTC(), MaxAge:config.MaxAge, MaxBytes:config.MaxBytes}, m.deliver(config))
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
		if id != config.ID && runtime.running() { active++ }
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
		if reserved { m.mu.Lock(); delete(m.connecting, config.ID); m.mu.Unlock() }
	}()

	deliver := m.deliver(config)
	segmentConfig := segment.Config{Directory: m.cfg.SpoolDirectory, DeviceSN: config.ID, PortName: config.Serial.PortName, SessionStart: time.Now().UTC(), MaxAge: config.MaxAge, MaxBytes: config.MaxBytes}
	if _, err := segment.Recover(context.Background(), segmentConfig, deliver); err != nil {
		return fmt.Errorf("recover device segments: %w", err)
	}
	writer, err := segment.NewWriter(segmentConfig, deliver)
	if err != nil {
		return err
	}
	decoder, err := serialagent.NewDecoder(config.Serial.Encoding)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &deviceRuntime{manager: m, config: config, rules: rules, writer: writer, decoder: decoder, state: StateConnecting, taskID: taskID, cancel: cancel, done: make(chan struct{})}
	m.mu.Lock()
	delete(m.connecting, config.ID)
	if m.closed {
		m.mu.Unlock()
		cancel()
		_ = writer.Close(context.Background())
		return errors.New("collector manager is closed")
	}
	m.devices[config.ID] = runtime
	m.mu.Unlock()
	reserved = false
	runtime.publish(Event{State: StateConnecting})
	go runtime.run(ctx)
	return nil
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
	if m.devices[deviceID] == runtime { delete(m.devices, deviceID) }
	m.mu.Unlock()
	return nil
}

func (m *Manager) deliver(config DeviceConfig) segment.Deliver {
	return func(ctx context.Context, completed segment.Completed) error {
		_, err := m.store.EnqueueFile(ctx, m.cfg.ProjectName, m.cfg.Version, spool.File{Path:completed.Path, SHA256:completed.SHA256, SizeBytes:completed.SizeBytes, DeviceSN:config.ID, FirstSequence:completed.FirstSequence, LastSequence:completed.LastSequence})
		return err
	}
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
	runtime.stop()
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
	return m.ConnectDevice(config)
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

func (d *deviceRuntime) run(ctx context.Context) {
	defer close(d.done)
	defer func() {
		if line, ok := d.decoder.Flush(time.Now().UTC()); ok {
			_ = d.handleLine(context.Background(), line)
		}
		_ = d.writer.Close(context.Background())
		d.mu.RLock()
		state := d.state
		d.mu.RUnlock()
		if state != StateDiskFull {
			d.setState(StateDisconnected, "")
		}
	}()
	rotateDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(d.config.MaxAge)
		defer ticker.Stop()
		defer close(rotateDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := d.writer.Rotate(context.Background()); err != nil {
					d.publish(Event{Error: err.Error()})
				}
			}
		}
	}()
	reconnect := serialagent.NewReconnectManager(d.manager.cfg.Reconnect)
	first := true
	for ctx.Err() == nil {
		if first {
			d.setState(StateConnecting, "")
		} else {
			d.mu.Lock()
			d.reconnects++
			d.mu.Unlock()
			d.setState(StateReconnecting, d.lastErr)
		}
		started := time.Now()
		port, err := d.manager.factory.Open(ctx, d.config.Serial)
		if err == nil {
			d.mu.Lock()
			d.port = port
			d.mu.Unlock()
			d.setState(StateCollecting, "")
			err = d.readLoop(ctx, port)
			_ = port.Close()
			d.mu.Lock()
			if d.port == port {
				d.port = nil
			}
				d.mu.Unlock()
			}
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
	rotateErr := d.writer.Rotate(ctx)
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
	for _, line := range d.decoder.Push(frame, at) {
		if err := d.handleLine(ctx, line); err != nil {
			return err
		}
	}
	return nil
}

func (d *deviceRuntime) handleLine(ctx context.Context, line serialagent.DecodedLine) error {
	exceeded, err := d.manager.disk.Exceeded(time.Now())
	if err != nil {
		return fmt.Errorf("check spool disk usage: %w", err)
	}
	if exceeded {
		d.setState(StateDiskFull, "spool disk threshold reached")
		return errDiskThreshold
	}
	sequence, err := d.manager.store.NextSequence(ctx, d.config.ID)
	if err != nil {
		return err
	}
	if err := d.writer.Write(ctx, segment.Entry{Sequence: sequence, CapturedAt: line.CapturedAt, Text: line.Text}); err != nil {
		return err
	}
	d.mu.Lock()
	d.lines++
	d.mu.Unlock()
	hits := make([]RuleHit, 0)
	for _, rule := range d.rules {
		if rule.match(line.Text) {
			count := rule.count.Add(1)
			hits = append(hits, RuleHit{RuleName: rule.rule.Name, Severity: rule.rule.Severity, Module: rule.rule.Module, Count: count})
		}
	}
	d.publish(Event{CapturedAt: line.CapturedAt, Text: line.Text, Hits: hits})
	return nil
}

func (d *deviceRuntime) publish(event Event) {
	d.mu.RLock()
	event.DeviceID, event.DeviceName, event.TaskID = d.config.ID, d.config.Name, d.taskID
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
}

func (d *deviceRuntime) snapshot() DeviceState {
	d.mu.RLock()
	state := DeviceState{DeviceID: d.config.ID, DeviceName: d.config.Name, PortName: d.config.Serial.PortName, TaskID: d.taskID, State: d.state, LastError: d.lastErr, DroppedEvents: d.dropped, LinesReceived: d.lines, Reconnects: d.reconnects, RuleCounts: make(map[string]uint64, len(d.rules))}
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
