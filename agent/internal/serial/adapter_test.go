package serial

import (
	"testing"

	seriallib "go.bug.st/serial"
)

func TestLibraryModeMapping(t *testing.T) {
	config := validSerialConfig()
	config.Parity = ParityEven
	config.StopBits = 2
	mode, err := libraryMode(config)
	if err != nil {
		t.Fatal(err)
	}
	if mode.BaudRate != 115200 || mode.DataBits != 8 || mode.Parity != seriallib.EvenParity || mode.StopBits != seriallib.TwoStopBits {
		t.Fatalf("unexpected serial mode: %#v", mode)
	}
}
