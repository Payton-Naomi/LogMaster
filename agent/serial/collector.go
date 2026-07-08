package serialagent

import (
	"bufio"
	"fmt"
	"sync"
	"time"
)

// LogLine 表示来自串口设备的一条日志行。
type LogLine struct {
	Device    string
	Timestamp time.Time
	Content   string
}

// Collector 管理多个串口设备连接并采集日志行。
type Collector struct {
	mu       sync.Mutex
	lines    chan LogLine
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// NewCollector 创建一个新的 Collector。
func NewCollector() *Collector {
	return &Collector{
		lines:  make(chan LogLine, 256),
		stopCh: make(chan struct{}),
	}
}

// Lines 返回一个只读的日志行通道。
func (c *Collector) Lines() <-chan LogLine {
	return c.lines
}

// Start 开始从指定设备采集日志。
func (c *Collector) Start(deviceName string, baudRate int) error {
	port, err := OpenPort(deviceName, baudRate)
	if err != nil {
		return fmt.Errorf("open port %s: %w", deviceName, err)
	}

	c.wg.Add(1)
	go c.readLoop(deviceName, port, baudRate)
	return nil
}

func (c *Collector) readLoop(deviceName string, port Port, baudRate int) {
	defer c.wg.Done()
	defer port.Close()

	reader := bufio.NewReader(port)
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			// 尝试重连
			port.Close()
			time.Sleep(2 * time.Second)
			newPort, err := OpenPort(deviceName, baudRate)
			if err != nil {
				continue
			}
			port = newPort
			reader = bufio.NewReader(port)
			continue
		}

		c.lines <- LogLine{
			Device:    deviceName,
			Timestamp: time.Now(),
			Content:   line,
		}
	}
}

// Stop 停止所有采集协程。
func (c *Collector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.wg.Wait()
		close(c.lines)
	})
}