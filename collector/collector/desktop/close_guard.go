package main

import "fmt"

func (s *Service) CloseWarnings() []string {
	warnings := make([]string, 0, 4)
	active := 0
	for _, state := range s.manager.GetDeviceStates() {
		if state.Enabled {
			active++
		}
	}
	if active > 0 {
		warnings = append(warnings, fmt.Sprintf("%d 个串口正在采集，退出时会封口日志并释放串口", active))
	}
	s.mu.RLock()
	dirty := 0
	for _, value := range s.configDirty {
		if value {
			dirty++
		}
	}
	s.mu.RUnlock()
	if dirty > 0 {
		warnings = append(warnings, fmt.Sprintf("%d 个通道有未保存的配置修改，这些修改会丢失", dirty))
	}
	status, err := s.GetUploadQueueStatus()
	if err == nil {
		if status.Uploading > 0 {
			warnings = append(warnings, fmt.Sprintf("%d 个批次正在上传，退出后将于下次启动恢复处理", status.Uploading))
		}
		if status.Uncertain > 0 {
			warnings = append(warnings, fmt.Sprintf("%d 个批次处于待人工核对状态", status.Uncertain))
		}
	}
	return warnings
}
