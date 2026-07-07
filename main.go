package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	serialagent "serial-agent/serialagent"
)

func main() {
	portName := flag.String("port", "", "Serial port name (e.g., /dev/ttyUSB0, COM3)")
	baudRate := flag.Int("baud", 9600, "Baud rate (e.g., 9600, 115200)")
	listPorts := flag.Bool("list", false, "List available serial ports")
	flag.Parse()

	if *listPorts {
		ports, err := serialagent.ListPorts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing ports: %v\n", err)
			os.Exit(1)
		}
		if len(ports) == 0 {
			fmt.Println("No serial ports found.")
		} else {
			fmt.Println("Available serial ports:")
			for _, p := range ports {
				fmt.Printf("  %s\n", p)
			}
		}
		return
	}

	if *portName == "" {
		flag.Usage()
		os.Exit(1)
	}

	port, err := serialagent.OpenPort(*portName, *baudRate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening port %s: %v\n", *portName, err)
		os.Exit(1)
	}
	defer port.Close()

	fmt.Printf("Connected to %s at %d baud\n", *portName, *baudRate)
	fmt.Println("Press Ctrl+C to exit.")

	// Handle Ctrl+C gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	// Read from serial port in background
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := port.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
				}
				return
			}
			if n > 0 {
				fmt.Printf("RX: %s\n", string(buf[:n]))
			}
		}
	}()

	<-sigCh
	fmt.Println("\nDisconnecting...")
}