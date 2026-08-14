package serial

import (
	"errors"
	"testing"
)

func TestMatchPortPriorityAndFallback(t *testing.T) {
	ports := []PortDescriptor{
		{Name: "COM3", VID: "1A86", PID: "7523", USBSerial: "SERIAL-A", Location: "Port_#0001.Hub_#0002"},
		{Name: "COM8", VID: "1A86", PID: "7523", USBSerial: "SERIAL-B", Location: "Port_#0002.Hub_#0002"},
	}
	matched, err := MatchPort(PortMatch{USBSerial: "serial-b", VID: "1A86", PID: "7523", Location: ports[0].Location, PortName: "COM3"}, ports)
	if err != nil || matched.Name != "COM8" {
		t.Fatalf("USB serial did not win: %#v, %v", matched, err)
	}

	matched, err = MatchPort(PortMatch{USBSerial: "missing", VID: "1a86", PID: "7523", Location: " port_#0001.hub_#0002 "}, ports)
	if err != nil || matched.Name != "COM3" {
		t.Fatalf("VID/PID/location fallback failed: %#v, %v", matched, err)
	}

	matched, err = MatchPort(PortMatch{PortName: "com8"}, ports)
	if err != nil || matched.Name != "COM8" {
		t.Fatalf("explicit port fallback failed: %#v, %v", matched, err)
	}
}

func TestMatchPortRejectsAmbiguity(t *testing.T) {
	ports := []PortDescriptor{{Name: "COM8", USBSerial: "same"}, {Name: "COM3", USBSerial: "same"}}
	_, err := MatchPort(PortMatch{USBSerial: "same", PortName: "COM8"}, ports)
	if !errors.Is(err, ErrAmbiguousPort) {
		t.Fatalf("expected ambiguous match, got %v", err)
	}
}

func TestMatchPortDoesNotUseDescriptions(t *testing.T) {
	ports := []PortDescriptor{{Name: "COM3", Product: "USB Serial"}}
	_, err := MatchPort(PortMatch{}, ports)
	if !errors.Is(err, ErrPortNotFound) {
		t.Fatalf("unexpected fuzzy match: %v", err)
	}
}
