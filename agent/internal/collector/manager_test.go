package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	serialagent "logmaster-agent/agent/internal/serial"
	"logmaster-agent/agent/internal/spool"
)

type fakeDiscovery struct{ ports []serialagent.PortDescriptor }

func (f fakeDiscovery) List(context.Context) ([]serialagent.PortDescriptor, error) {
	return append([]serialagent.PortDescriptor(nil), f.ports...), nil
}

type fakePort struct {
	reads  chan []byte
	closed chan struct{}
	once   sync.Once
	mu     sync.Mutex
	writes [][]byte
}

func newFakePort() *fakePort {
	return &fakePort{reads: make(chan []byte, 4), closed: make(chan struct{})}
}
func (p *fakePort) Read(buffer []byte) (int, error) {
	timer := time.NewTimer(2 * time.Millisecond)
	defer timer.Stop()
	select {
	case data := <-p.reads:
		return copy(buffer, data), nil
	case <-p.closed:
		return 0, io.EOF
	case <-timer.C:
		return 0, os.ErrDeadlineExceeded
	}
}
func (p *fakePort) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writes = append(p.writes, append([]byte(nil), data...))
	return len(data), nil
}
func (*fakePort) SetDTR(bool) error        { return nil }
func (*fakePort) SetRTS(bool) error        { return nil }
func (*fakePort) ResetInputBuffer() error  { return nil }
func (*fakePort) ResetOutputBuffer() error { return nil }
func (p *fakePort) Close() error           { p.once.Do(func() { close(p.closed) }); return nil }

type fakeFactory struct {
	mu    sync.Mutex
	ports []*fakePort
}

type mappedFakeFactory struct {
	mu    sync.Mutex
	ports map[string]*fakePort
	used  map[string]bool
}

func (f *mappedFakeFactory) Open(ctx context.Context, cfg serialagent.SerialConfig) (serialagent.Port, error) {
	f.mu.Lock()
	port, used := f.ports[cfg.PortName], f.used[cfg.PortName]
	if port != nil && !used {
		f.used[cfg.PortName] = true
		f.mu.Unlock()
		return port, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *fakeFactory) Open(ctx context.Context, _ serialagent.SerialConfig) (serialagent.Port, error) {
	f.mu.Lock()
	if len(f.ports) > 0 {
		port := f.ports[0]
		f.ports = f.ports[1:]
		f.mu.Unlock()
		return port, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func serialConfig(port string) serialagent.SerialConfig {
	return serialagent.SerialConfig{PortName: port, BaudRate: 115200, DataBits: 8, StopBits: 1, Parity: serialagent.ParityNone, Handshake: serialagent.HandshakeNone, ReadTimeout: 10 * time.Millisecond, WriteTimeout: time.Second, IdleGap: time.Millisecond, MaxFrameBytes: 1024, Encoding: serialagent.EncodingUTF8}
}

func newTestManager(t *testing.T, factory PortFactory, maxDevices int) (*Manager, *spool.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := spool.Open(filepath.Join(dir, "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Config{MaxDevices: maxDevices, EventCapacity: 4, SpoolDirectory: filepath.Join(dir, "spool"), MaxDiskBytes: 1 << 30, ProjectName: "p", Version: "v", Reconnect: serialagent.ReconnectConfig{InitialDelay: time.Millisecond, MaxDelay: time.Millisecond, Multiplier: 1}}, store, fakeDiscovery{ports: []serialagent.PortDescriptor{{Name: "COM3"}}}, factory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(); _ = store.Close() })
	return manager, store
}

func TestManagerCollectsRulesQueuesSegmentAndSendsCommand(t *testing.T) {
	port := newFakePort()
	factory := &fakeFactory{ports: []*fakePort{port}}
	manager, store := newTestManager(t, factory, 2)
	events, unsubscribe := manager.SubscribeLogEvents()
	defer unsubscribe()
	if err := manager.StartTask("task-1"); err != nil {
		t.Fatal(err)
	}
	if err := manager.ConnectDevice(DeviceConfig{ID: "D1", Name: "front", Serial: serialConfig("COM3"), MaxAge: time.Hour, MaxBytes: 1 << 20, Rules: []Rule{{Name: "boom", Keywords: []string{"error"}, Pattern: `boom`, Severity: "error", Module: "camera"}}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if err := manager.SendCommand("D1", []byte("ping")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("device did not connect")
		}
		time.Sleep(time.Millisecond)
	}
	port.reads <- []byte("ERROR boom\n")
	var line Event
	for deadline := time.After(2 * time.Second); ; {
		select {
		case event := <-events:
			if event.Text != "" {
				line = event
				goto received
			}
		case <-deadline:
			t.Fatal("log event not received")
		}
	}
received:
	if len(line.Hits) != 1 || line.Hits[0].Count != 1 || line.TaskID != "task-1" {
		t.Fatalf("event=%+v", line)
	}
	states := manager.GetDeviceStates()
	if len(states) != 1 || states[0].RuleCounts["boom"] != 1 {
		t.Fatalf("states=%+v", states)
	}
	if err := manager.DisconnectDevice("D1"); err != nil {
		t.Fatal(err)
	}
	counts, err := store.Counts(context.Background())
	if err != nil || counts[spool.Pending] != 1 {
		t.Fatalf("counts=%v err=%v", counts, err)
	}
}

func TestManagerEnforcesDeviceAndDiskLimits(t *testing.T) {
	port := newFakePort()
	manager, store := newTestManager(t, &fakeFactory{ports: []*fakePort{port}}, 1)
	manager.disk.interval = 0
	manager.disk.measure = func(string) (int64, error) { return manager.cfg.MaxDiskBytes, nil }
	if err := manager.ConnectDevice(DeviceConfig{ID: "D1", Serial: serialConfig("COM3")}); err != nil {
		t.Fatal(err)
	}
	if err := manager.ConnectDevice(DeviceConfig{ID: "D2", Serial: serialConfig("COM4")}); err == nil {
		t.Fatal("expected device limit")
	}
	port.reads <- []byte("line\n")
	deadline := time.Now().Add(2 * time.Second)
	for {
		states := manager.GetDeviceStates()
		if len(states) == 1 && states[0].State == StateDiskFull {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("disk state not reached: %+v", states)
		}
		time.Sleep(time.Millisecond)
	}
	counts, err := store.Counts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if counts[spool.Pending] != 0 {
		t.Fatalf("disk-full line was queued: %v", counts)
	}
}

func TestBrokerNeverBlocksSlowSubscriber(t *testing.T) {
	broker := NewBroker(1)
	_, unsubscribe := broker.Subscribe()
	defer unsubscribe()
	if dropped := broker.Publish(Event{}); dropped != 0 {
		t.Fatal(dropped)
	}
	if dropped := broker.Publish(Event{}); dropped != 1 {
		t.Fatalf("dropped=%d", dropped)
	}
}

func TestScanPortsDoesNotDependOnConnectedDevices(t *testing.T) {
	manager, _ := newTestManager(t, &fakeFactory{}, 2)
	ports, err := manager.ScanPorts(context.Background())
	if err != nil || len(ports) != 1 || ports[0].Name != "COM3" {
		t.Fatalf("ports=%+v err=%v", ports, err)
	}
	if err := manager.DisconnectDevice("missing"); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFourDevicesCollectIndependentlyWhenOneDisconnects(t *testing.T) {
	ports := []*fakePort{newFakePort(), newFakePort(), newFakePort(), newFakePort()}
	factory := &mappedFakeFactory{ports: map[string]*fakePort{"COM1": ports[0], "COM2": ports[1], "COM3": ports[2], "COM4": ports[3]}, used: map[string]bool{}}
	manager, _ := newTestManager(t, factory, 8)
	manager.broker = NewBroker(32)
	for i := range ports {
		id := fmt.Sprintf("D%d", i+1)
		if err := manager.ConnectDevice(DeviceConfig{ID: id, Name: id, Serial: serialConfig(fmt.Sprintf("COM%d", i+1)), MaxAge: time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		states, connected := manager.GetDeviceStates(), 0
		for _, state := range states {
			if state.State == StateCollecting {
				connected++
			}
		}
		if connected == 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("four devices did not connect: %+v", states)
		}
		time.Sleep(time.Millisecond)
	}
	events, unsubscribe := manager.SubscribeLogEvents()
	defer unsubscribe()
	_ = ports[0].Close()
	for i := 1; i < 4; i++ {
		ports[i].reads <- []byte(fmt.Sprintf("device-%d-line\n", i+1))
	}
	received, timeout := map[string]bool{}, time.After(2*time.Second)
	for len(received) < 3 {
		select {
		case event := <-events:
			if event.Text != "" {
				received[event.DeviceID] = true
			}
		case <-timeout:
			t.Fatalf("healthy devices blocked after one disconnect: received=%v states=%+v", received, manager.GetDeviceStates())
		}
	}
	if len(received) != 3 {
		t.Fatalf("expected three healthy devices after one disconnect: %v", received)
	}
}
