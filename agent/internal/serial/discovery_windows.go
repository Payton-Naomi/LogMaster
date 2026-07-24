//go:build windows

package serial

import (
	"errors"
	"strings"
	"unsafe"

	"go.bug.st/serial/enumerator"
	"golang.org/x/sys/windows"
)

func discoverSystemPorts() ([]PortDescriptor, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	locations, locationErr := windowsPortLocations()
	ports := make([]PortDescriptor, 0, len(details))
	for _, detail := range details {
		ports = append(ports, PortDescriptor{
			Name:         detail.Name,
			VID:          detail.VID,
			PID:          detail.PID,
			USBSerial:    detail.SerialNumber,
			Location:     locations[strings.ToUpper(detail.Name)],
			Manufacturer: detail.Manufacturer,
			Product:      detail.Product,
			IsUSB:        detail.IsUSB,
		})
	}
	if len(ports) == 0 && locationErr != nil {
		return nil, locationErr
	}
	return ports, nil
}

func windowsPortLocations() (map[string]string, error) {
	guids, err := windows.SetupDiClassGuidsFromNameEx("Ports", "")
	if err != nil {
		return nil, err
	}
	locations := make(map[string]string)
	var queryErrs []error
	for _, guid := range guids {
		set, err := windows.SetupDiGetClassDevsEx(&guid, "", 0, windows.DIGCF_PRESENT, 0, "")
		if err != nil {
			queryErrs = append(queryErrs, err)
			continue
		}
		for index := 0; ; index++ {
			device, enumErr := set.EnumDeviceInfo(index)
			if enumErr != nil {
				break
			}
			key, keyErr := set.OpenDevRegKey(device, windows.DICS_FLAG_GLOBAL, 0, windows.DIREG_DEV, windows.KEY_READ)
			if keyErr != nil {
				continue
			}
			var nameBuffer [256]uint16
			nameBytes := uint32(len(nameBuffer) * 2)
			valueErr := windows.RegQueryValueEx(key, windows.StringToUTF16Ptr("PortName"), nil, nil, (*byte)(unsafe.Pointer(&nameBuffer[0])), &nameBytes)
			_ = windows.RegCloseKey(key)
			if valueErr != nil {
				continue
			}
			location := devicePropertyString(set, device, windows.SPDRP_LOCATION_PATHS)
			if location == "" {
				location = devicePropertyString(set, device, windows.SPDRP_LOCATION_INFORMATION)
			}
			name := strings.TrimSpace(windows.UTF16ToString(nameBuffer[:]))
			if name != "" && location != "" {
				locations[strings.ToUpper(name)] = location
			}
		}
		_ = set.Close()
	}
	return locations, errors.Join(queryErrs...)
}

func devicePropertyString(set windows.DevInfo, device *windows.DevInfoData, property windows.SPDRP) string {
	value, err := set.DeviceRegistryProperty(device, property)
	if err != nil {
		return ""
	}
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case []string:
		return strings.Join(value, ";")
	default:
		return ""
	}
}
