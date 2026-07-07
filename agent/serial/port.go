package serialagent

import "go.bug.st/serial"

func ListPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	if ports == nil {
		return []string{}, nil
	}
	return ports, nil
}