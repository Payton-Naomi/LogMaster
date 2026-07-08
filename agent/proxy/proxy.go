package proxy

import (
	"bufio"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	serialagent "logmaster-agent/agent/serial"
)

// Status 表示当前串口连接的状态。
type Status struct {
	Connected bool   `json:"connected"`
	Device    string `json:"device"`
	BaudRate  int    `json:"baud_rate"`
	DataBits  int    `json:"data_bits"`
	StopBits  int    `json:"stop_bits"`
	Parity    string `json:"parity"`
	RxCount   int64  `json:"rx_count"`
	TxCount   int64  `json:"tx_count"`
}

// Proxy 管理串口连接状态，提供统一的控制接口。
type Proxy struct {
	mu          sync.RWMutex
	port        serialagent.Port
	connected   bool
	device      string
	baudRate    int
	dataBits    int
	stopBits    int
	parity      string
	rxCount     atomic.Int64
	txCount     atomic.Int64
	stopCh      chan struct{}
	subscribers map[chan serialagent.LogLine]struct{}
	subMu       sync.RWMutex
}

// New 创建一个新的 Proxy 实例。
func New() *Proxy {
	return &Proxy{
		subscribers: make(map[chan serialagent.LogLine]struct{}),
	}
}

// ListPorts 返回系统中可用的串口列表。
func (p *Proxy) ListPorts() ([]string, error) {
	return serialagent.ListPorts()
}

// Connect 连接到指定的串口设备。
func (p *Proxy) Connect(device string, baudRate, dataBits, stopBits int, parity string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connected {
		_ = p.disconnectLocked()
	}

	port, err := serialagent.OpenPort(device, baudRate, dataBits, stopBits, parity)
	if err != nil {
		return fmt.Errorf("打开串口 %s: %w", device, err)
	}

	p.port = port
	p.connected = true
	p.device = device
	p.baudRate = baudRate
	p.dataBits = dataBits
	p.stopBits = stopBits
	p.parity = parity
	p.rxCount.Store(0)
	p.txCount.Store(0)
	p.stopCh = make(chan struct{})

	go p.readLoop()
	return nil
}

// Disconnect 断开当前串口连接。
func (p *Proxy) Disconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.disconnectLocked()
}

func (p *Proxy) disconnectLocked() error {
	if !p.connected {
		return nil
	}
	close(p.stopCh)
	err := p.port.Close()
	p.connected = false
	p.port = nil
	return err
}

// Send 通过串口发送数据。
func (p *Proxy) Send(data []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.connected {
		return 0, fmt.Errorf("串口未连接")
	}

	n, err := p.port.Write(data)
	if n > 0 {
		p.txCount.Add(int64(n))
	}
	return n, err
}

// Status 返回当前连接状态。
func (p *Proxy) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return Status{
		Connected: p.connected,
		Device:    p.device,
		BaudRate:  p.baudRate,
		DataBits:  p.dataBits,
		StopBits:  p.stopBits,
		Parity:    p.parity,
		RxCount:   p.rxCount.Load(),
		TxCount:   p.txCount.Load(),
	}
}

// Subscribe 订阅日志行，返回一个接收通道。
func (p *Proxy) Subscribe() chan serialagent.LogLine {
	ch := make(chan serialagent.LogLine, 256)
	p.subMu.Lock()
	p.subscribers[ch] = struct{}{}
	p.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅。
func (p *Proxy) Unsubscribe(ch chan serialagent.LogLine) {
	p.subMu.Lock()
	delete(p.subscribers, ch)
	p.subMu.Unlock()
	close(ch)
}

// readLoop 从串口读取数据并广播给所有订阅者。
func (p *Proxy) readLoop() {
	reader := bufio.NewReader(p.port)

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			// 读取失败，断开连接
			p.mu.Lock()
			if p.connected {
				p.port.Close()
				p.connected = false
				p.port = nil
			}
			p.mu.Unlock()
			return
		}

		p.rxCount.Add(1)

		logLine := serialagent.LogLine{
			Device:    p.device,
			Timestamp: time.Now(),
			Content:   line,
		}

		// 广播给所有订阅者
		p.subMu.RLock()
		for ch := range p.subscribers {
			select {
			case ch <- logLine:
			default:
				// 订阅者通道已满，丢弃
			}
		}
		p.subMu.RUnlock()
	}
}