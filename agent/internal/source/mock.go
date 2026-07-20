package source

import (
	"context"
	"fmt"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/config"
)

type Mock struct{ cfg config.DeviceConfig }

func NewMock(cfg config.DeviceConfig) *Mock { return &Mock{cfg: cfg} }

func (m *Mock) Run(ctx context.Context, emit func(string) error) error {
	ticker := time.NewTicker(m.cfg.MockInterval)
	defer ticker.Stop()
	levels := []string{"INFO", "INFO", "WARN", "ERROR"}
	index := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			line := fmt.Sprintf("[%s] [%s] simulated device log sequence=%d", now.Format("2006-01-02 15:04:05.000"), levels[index%len(levels)], index+1)
			if err := emit(line); err != nil {
				return err
			}
			index++
		}
	}
}
