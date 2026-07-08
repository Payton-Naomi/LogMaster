package serialagent

import "go.bug.st/serial"

// Port 表示一个打开的串口连接。
type Port interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// parityMap 将字符串转换为 serial.Parity 枚举值。
var parityMap = map[string]serial.Parity{
	"none":  serial.NoParity,
	"odd":   serial.OddParity,
	"even":  serial.EvenParity,
	"mark":  serial.MarkParity,
	"space": serial.SpaceParity,
}

// OpenPort 使用给定的名称、波特率、数据位、停止位和校验位打开串口。
func OpenPort(name string, baudRate, dataBits, stopBits int, parity string) (Port, error) {
	p, ok := parityMap[parity]
	if !ok {
		p = serial.NoParity
	}
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: dataBits,
		StopBits: serial.StopBits(stopBits),
		Parity:   p,
	}
	return serial.Open(name, mode)
}