package serialagent

import "go.bug.st/serial"

// Port 表示一个打开的串口连接。
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// OpenPort 使用给定的名称和波特率打开串口。
func OpenPort(name string, baudRate int) (Port, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
	}
	return serial.Open(name, mode)
}