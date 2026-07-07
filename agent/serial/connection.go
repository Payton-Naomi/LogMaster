package serialagent

import "go.bug.st/serial"

// Port represents an open serial port connection.
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// OpenPort opens a serial port with the given name and baud rate.
func OpenPort(name string, baudRate int) (Port, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
	}
	return serial.Open(name, mode)
}