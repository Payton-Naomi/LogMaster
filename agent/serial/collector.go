package serialagent

import (
	"bufio"
	"fmt"
	"sync"
	"time"
)

// LogLine represents a single line of log from a serial device.
type LogLine struct {
	Device    string
	Timestamp time.Time
	Content   string
}

// Collector manages multiple serial device connections and collects log lines.
type Collector struct {
	mu     sync.Mutex
	lines  chan LogLine
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewCollector creates a new Collector.
func NewCollector() *Collector {
	return &Collector{
		lines:  make(chan LogLine, 256),
		stopCh: make(chan struct{}),
	}
}

// Lines returns a read-only channel of collected log lines.
func (c *Collector) Lines() <-chan LogLine {
	return c.lines
}

// Start begins collecting from a device.
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
			// Try reconnect
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

// Stop stops all collection goroutines.
func (c *Collector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
	close(c.lines)
}