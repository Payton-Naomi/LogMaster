package serialagent

import "testing"

func TestListPorts(t *testing.T) {
	ports, err := ListPorts()
	if err != nil {
		t.Fatalf("ListPorts() returned error: %v", err)
	}
	if ports == nil {
		t.Fatal("ListPorts() returned nil slice")
	}
	// ports may be empty if no serial ports exist, that's OK
	t.Logf("Found %d ports: %v", len(ports), ports)
}