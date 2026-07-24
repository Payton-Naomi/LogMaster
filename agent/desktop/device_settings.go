package main

import "errors"

func (s *Service) SaveDeviceConfig(id string, dto DeviceConfigDTO) error {
	dto.DeviceID = id
	if dto.BaudRate == 0 {
		dto.BaudRate = 115200
	}
	if dto.DataBits == 0 {
		dto.DataBits = 8
	}
	if dto.StopBits == 0 {
		dto.StopBits = 1
	}
	if dto.Parity == "" {
		dto.Parity = "none"
	}
	config := toCollectorConfig(dto)
	if err := config.Serial.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	s.configs[id] = dto
	s.mu.Unlock()
	if err := s.saveSettings(); err != nil {
		return err
	}
	for _, state := range s.manager.GetDeviceStates() {
		if state.DeviceID == id {
			return s.manager.UpdateDeviceConfig(id, config)
		}
	}
	return nil
}

func (s *Service) ValidateDeviceConfig(dto DeviceConfigDTO) error {
	if dto.DeviceID == "" {
		return errors.New("device id is required")
	}
	return toCollectorConfig(dto).Serial.Validate()
}
