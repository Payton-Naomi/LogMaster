package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"unicode/utf8"

	"logmaster-agent/agent/internal/backend"
)

func (s *Service) SaveDeviceConfig(id string, dto DeviceConfigDTO) error {
	_, err := s.SaveDeviceConfigWithResult(id, dto)
	return err
}

func (s *Service) SaveDeviceConfigWithResult(id string, dto DeviceConfigDTO) (DeviceConfigSaveResult, error) {
	dto.DeviceID = id
	dto = normalizeDeviceConfig(dto)
	dto.Configured = true
	if dto.UploadEnabled {
		if err := validateUploadPrerequisites(dto); err != nil {
			return DeviceConfigSaveResult{}, err
		}
		if err := validateUploadFields(dto); err != nil {
			return DeviceConfigSaveResult{}, err
		}
	}
	if !s.catalogSelectionValid(dto) {
		return DeviceConfigSaveResult{}, errors.New("选择的项目、测试任务或关键字方案不在内置配置中")
	}
	id = dto.DeviceID
	config := s.toCollectorConfig(dto)
	if err := config.Serial.Validate(); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	s.mu.RLock()
	previous := s.configs[id]
	settings := s.appSettings
	s.mu.RUnlock()
	if !dto.UploadEnabled {
		dto.UploadSessionID, dto.QueryCode, dto.UploadSetupID, dto.UploadConfigFingerprint, dto.ConfigSnapshot = "", "", "", "", ""
		dto.UploadSetupState = "disabled"
		if err := s.store.SetDeviceUploadPaused(s.ctx, id, true); err != nil {
			return DeviceConfigSaveResult{}, err
		}
		if err := s.persistDeviceConfig(id, dto); err != nil {
			return DeviceConfigSaveResult{}, err
		}
		if previous.UploadSessionID != "" {
			_ = s.backendClient.CompleteUploadSession(s.ctx, previous.UploadSessionID)
		}
		return DeviceConfigSaveResult{Saved: true}, nil
	}
	request := uploadSessionRequest(dto, settings.ProgramVersion)
	fingerprint, _, err := uploadSessionFingerprint(request)
	if err != nil {
		return DeviceConfigSaveResult{}, err
	}
	if previous.UploadSetupState == "active" && previous.UploadConfigFingerprint == fingerprint && previous.UploadSessionID != "" && previous.QueryCode != "" {
		dto.UploadSessionID, dto.QueryCode = previous.UploadSessionID, previous.QueryCode
		dto.UploadSetupID, dto.UploadSetupState = previous.UploadSetupID, "active"
		dto.UploadConfigFingerprint, dto.ConfigSnapshot = fingerprint, previous.ConfigSnapshot
		if err := s.persistDeviceConfig(id, dto); err != nil {
			return DeviceConfigSaveResult{}, err
		}
		if err := s.store.SetDeviceUploadPaused(s.ctx, id, false); err != nil {
			return DeviceConfigSaveResult{}, err
		}
		return DeviceConfigSaveResult{Saved: true, UploadReady: true, QueryCode: dto.QueryCode}, nil
	}
	setupID := previous.UploadSetupID
	if previous.UploadConfigFingerprint != fingerprint || setupID == "" {
		setupID, err = newUploadSetupID()
		if err != nil {
			return DeviceConfigSaveResult{}, err
		}
	}
	request.ClientRequestID = setupID
	encoded, err := json.Marshal(request)
	if err != nil {
		return DeviceConfigSaveResult{}, err
	}
	dto.UploadSessionID, dto.QueryCode = "", ""
	dto.UploadSetupID, dto.UploadSetupState = setupID, "pending"
	dto.UploadConfigFingerprint, dto.ConfigSnapshot = fingerprint, string(encoded)
	if err := s.store.SetDeviceUploadPaused(s.ctx, id, true); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	if err := s.persistDeviceConfig(id, dto); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	accepted, createErr := s.backendClient.CreateUploadSession(s.ctx, request)
	if createErr != nil {
		s.logger.Error("create upload session failed", "component", "desktop.upload", "device_id", dto.DeviceID, "port_name", dto.PortName, "error", createErr)
		return DeviceConfigSaveResult{Saved: true, Message: uploadSessionFailureMessage(createErr)}, nil
	}
	dto.UploadSessionID, dto.QueryCode, dto.UploadSetupState = accepted.UploadSessionID, accepted.QueryCode, "active"
	dto.UploaderName, dto.UploaderEmail = accepted.UploaderName, accepted.UploaderEmail
	if len(accepted.ConfigSnapshot) > 0 {
		dto.ConfigSnapshot = string(accepted.ConfigSnapshot)
	}
	canonicalRequest := uploadSessionRequest(dto, settings.ProgramVersion)
	dto.UploadConfigFingerprint, _, _ = uploadSessionFingerprint(canonicalRequest)
	if err := s.persistDeviceConfig(id, dto); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	if err := s.store.BindPendingUploads(s.ctx, id, accepted.UploadSessionID, accepted.QueryCode, dto.UploaderName, dto.UploaderEmail, dto.ConfigSnapshot); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	if err := s.store.SetDeviceUploadPaused(s.ctx, id, false); err != nil {
		return DeviceConfigSaveResult{}, err
	}
	if previous.UploadSessionID != "" && previous.UploadSessionID != accepted.UploadSessionID {
		_ = s.backendClient.CompleteUploadSession(s.ctx, previous.UploadSessionID)
	}
	return DeviceConfigSaveResult{Saved: true, UploadReady: true, QueryCode: accepted.QueryCode}, nil
}

func (s *Service) persistDeviceConfig(id string, dto DeviceConfigDTO) error {
	s.mu.Lock()
	s.configs[id] = dto
	s.configDirty[id] = false
	s.mu.Unlock()
	if err := s.saveSettings(); err != nil {
		return err
	}
	config := s.toCollectorConfig(dto)
	for _, state := range s.manager.GetDeviceStates() {
		if state.DeviceID == id {
			return s.manager.UpdateDeviceConfig(id, config)
		}
	}
	return nil
}

func uploadSessionRequest(dto DeviceConfigDTO, collectorVersion string) backend.UploadSessionRequest {
	return backend.UploadSessionRequest{DeviceID: dto.DeviceID, Name: dto.Name, PortName: dto.PortName, BaudRate: dto.BaudRate, DataBits: dto.DataBits, StopBits: dto.StopBits, Parity: dto.Parity, Handshake: dto.Handshake, DTR: dto.DTR, RTS: dto.RTS, ProjectID: numericPlatformProjectID(dto.ProjectID), ProjectName: dto.ProjectName, Version: dto.Version, TestTaskID: dto.TestTaskID, TestTaskName: dto.TestTaskName, UploaderName: dto.UploaderName, UploaderEmail: dto.UploaderEmail, Remark: dto.Remark, ScenarioIDs: append([]string(nil), dto.ScenarioIDs...), KeywordProfileID: dto.KeywordProfileID, KeywordRuleIDs: append([]string(nil), dto.KeywordRuleIDs...), KeywordMatching: dto.KeywordMatchingEnabled, SaveEnabled: dto.SaveEnabled, UploadEnabled: dto.UploadEnabled, NoLogTimeoutSeconds: dto.NoLogTimeoutSeconds, VID: dto.VID, PID: dto.PID, USBSerial: dto.USBSerial, Location: dto.Location, CollectorVersion: collectorVersion, Timezone: localTimezoneName()}
}

func numericPlatformProjectID(value string) string {
	for _, character := range strings.TrimSpace(value) {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return strings.TrimSpace(value)
}

func uploadSessionFingerprint(request backend.UploadSessionRequest) (string, string, error) {
	request.ClientRequestID = ""
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), string(encoded), nil
}

func newUploadSetupID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// uploadSessionFailureMessage turns a CreateUploadSession error into a
// user-facing message. Previously every failure collapsed into a generic
// "上传失败，请重试" which left testers nothing to act on; now the backend's
// own message (for example a project-name mismatch or missing uploader) is
// surfaced, and transport errors are explained separately.
func uploadSessionFailureMessage(err error) string {
	var transport *url.Error
	if errors.As(err, &transport) {
		return "无法连接云端服务，请检查网络或后端地址：" + transport.Err.Error()
	}
	return "创建上传会话失败：" + err.Error()
}

func (s *Service) SetDeviceConfigDirty(id string, dirty bool) {
	s.mu.Lock()
	s.configDirty[id] = dirty
	s.mu.Unlock()
}

func (s *Service) ValidateDeviceConfig(dto DeviceConfigDTO) error {
	dto = normalizeDeviceConfig(dto)
	if dto.DeviceID == "" {
		return errors.New("device id is required")
	}
	if err := s.toCollectorConfig(dto).Serial.Validate(); err != nil {
		return err
	}
	if dto.UploadEnabled {
		if err := validateUploadPrerequisites(dto); err != nil {
			return err
		}
		if err := validateUploadFields(dto); err != nil {
			return err
		}
	}
	if !s.catalogSelectionValid(dto) {
		return errors.New("选择的项目、测试任务或关键字方案不在内置配置中")
	}
	return nil
}

func validateUploadPrerequisites(dto DeviceConfigDTO) error {
	if !dto.SaveEnabled {
		return errors.New("开启云端上传前必须先开启本地日志保存")
	}
	if strings.TrimSpace(dto.ProjectID) == "" || strings.TrimSpace(dto.Version) == "" {
		return errors.New("开启云端上传前必须选择项目并填写版本号")
	}
	if strings.TrimSpace(dto.UploaderEmail) == "" {
		return errors.New("开启云端上传前必须填写上传人企业邮箱")
	}
	address, err := mail.ParseAddress(dto.UploaderEmail)
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(dto.UploaderEmail)) {
		return errors.New("请输入完整有效的上传人企业邮箱")
	}
	return nil
}

func validateUploadFields(dto DeviceConfigDTO) error {
	limits := []struct {
		name  string
		value string
		max   int
	}{
		{"项目名称", dto.ProjectName, 128},
		{"版本号", dto.Version, 64},
		{"测试任务 ID", dto.TestTaskID, 128},
		{"测试任务名称", dto.TestTaskName, 256},
		{"上传人", dto.UploaderName, 128},
		{"上传人企业邮箱", dto.UploaderEmail, 320},
		{"测试备注", dto.Remark, 4000},
	}
	for _, field := range limits {
		if utf8.RuneCountInString(field.value) > field.max {
			return fmt.Errorf("%s不能超过 %d 个字符", field.name, field.max)
		}
	}
	if len(dto.ScenarioIDs) > 20 {
		return errors.New("测试场景不能超过 20 个")
	}
	return nil
}
