//go:build !windows

package serial

import "go.bug.st/serial/enumerator"

func discoverSystemPorts() ([]PortDescriptor, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	ports := make([]PortDescriptor, 0, len(details))
	for _, detail := range details {
		ports = append(ports, PortDescriptor{
			Name:         detail.Name,
			VID:          detail.VID,
			PID:          detail.PID,
			USBSerial:    detail.SerialNumber,
			Manufacturer: detail.Manufacturer,
			Product:      detail.Product,
			IsUSB:        detail.IsUSB,
		})
	}
	return ports, nil
}
