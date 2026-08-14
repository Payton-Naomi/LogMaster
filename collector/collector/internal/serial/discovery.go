package serial

import (
	"context"
	"fmt"
)

type SystemDiscovery struct{}

func (SystemDiscovery) List(ctx context.Context) ([]PortDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ports, err := discoverSystemPorts()
	if err != nil {
		return nil, fmt.Errorf("discover serial ports: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ports, nil
}
