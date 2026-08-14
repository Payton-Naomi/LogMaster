//go:build windows

package serial

import (
	"testing"

	"go.bug.st/serial/enumerator"
)

func TestMergePresentPortsExcludesRegistryOnlyNames(t *testing.T) {
	details := []*enumerator.PortDetails{{Name: "COM1"}}
	names := []string{"COM1", "COM24", "COM39"}
	present := map[string]string{"COM1": "", "COM39": "USBROOT(0)#USB(1)"}

	ports := mergePresentPorts(details, names, present)
	if len(ports) != 2 || ports[0].Name != "COM1" || ports[1].Name != "COM39" {
		t.Fatalf("unexpected present ports: %+v", ports)
	}
}
