package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"logmaster-agent/agent/internal/backend"
)

type AppSettingsDTO struct {
	SchemaVersion          int    `json:"schemaVersion"`
	DefaultLogDirectory    string `json:"defaultLogDirectory"`
	DefaultSaveEnabled     bool   `json:"defaultSaveEnabled"`
	DefaultUploadEnabled   bool   `json:"defaultUploadEnabled"`
	SegmentMaxAgeSeconds   int    `json:"segmentMaxAgeSeconds"`
	SegmentMaxBytes        int64  `json:"segmentMaxBytes"`
	NoLogTimeoutSeconds    int    `json:"noLogTimeoutSeconds"`
	MaxLogLines            int    `json:"maxLogLines"`
	LogFontSize            int    `json:"logFontSize"`
	AutoWrap               bool   `json:"autoWrap"`
	MaxDiskBytes           int64  `json:"maxDiskBytes"`
	StorageWarningPercent  int    `json:"storageWarningPercent"`
	AutoDeleteUploaded     bool   `json:"autoDeleteUploaded"`
	UploadedRetentionHours int    `json:"uploadedRetentionHours"`
	BackendURL             string `json:"backendUrl"`
	UploadIntervalSeconds  int    `json:"uploadIntervalSeconds"`
	UploadConcurrency      int    `json:"uploadConcurrency"`
	UploadGzip             bool   `json:"uploadGzip"`
	ProgramName            string `json:"programName"`
	ProgramVersion         string `json:"programVersion"`
	BuildVersion           string `json:"buildVersion"`
	UpdateDate             string `json:"updateDate"`
	CompanyName            string `json:"companyName"`
	CommunityTitle         string `json:"communityTitle"`
	CommunityText          string `json:"communityText"`
	CommunityURL           string `json:"communityUrl"`
}

func defaultAppSettings(root string) AppSettingsDTO {
	d := packagedDefaults
	directory := defaultLogDirectory(root)
	return AppSettingsDTO{SchemaVersion: 1, DefaultLogDirectory: directory, DefaultSaveEnabled: d.Log.SaveEnabled, DefaultUploadEnabled: d.Log.UploadEnabled, SegmentMaxAgeSeconds: d.Log.SegmentMinutes * 60, SegmentMaxBytes: d.Log.SegmentSizeMB * 1024 * 1024, NoLogTimeoutSeconds: d.Log.NoLogTimeoutSeconds, MaxLogLines: d.Log.MaxWindowLines, LogFontSize: d.Log.FontSize, AutoWrap: d.Log.AutoWrap, MaxDiskBytes: d.Storage.LimitGB * 1024 * 1024 * 1024, StorageWarningPercent: d.Storage.WarningPercent, AutoDeleteUploaded: false, UploadedRetentionHours: 0, BackendURL: d.Upload.BackendURL, UploadIntervalSeconds: d.Upload.IntervalSeconds, UploadConcurrency: d.Upload.Concurrency, UploadGzip: d.Upload.Gzip, ProgramName: "LogMaster采集端", ProgramVersion: "0.0.10", BuildVersion: "0.0.10", UpdateDate: "2026-08-14", CompanyName: "上海七十迈数字科技有限公司", CommunityTitle: "飞书交流群", CommunityText: "使用说明、问题反馈、获取最新版本请扫码加入飞书交流群。"}
}

func defaultLogDirectory(root string) string {
	if base, err := os.UserConfigDir(); err == nil && strings.EqualFold(filepath.Clean(root), filepath.Join(base, "LogMaster")) {
		if configured := strings.TrimSpace(packagedDefaults.Log.Directory); configured != "" {
			return filepath.Clean(configured)
		}
		return filepath.Clean("D:/LogMaster/LocalLog")
	}
	return filepath.Join(root, "data", "spool")
}

func loadAppSettings(path, root string) (AppSettingsDTO, error) {
	settings := defaultAppSettings(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return AppSettingsDTO{}, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return AppSettingsDTO{}, fmt.Errorf("解析程序设置: %w", err)
	}
	normalizeAppSettings(&settings, root)
	return settings, nil
}

func loadAppSettingsRecovered(path, root string) (AppSettingsDTO, string, error) {
	settings, err := loadAppSettings(path, root)
	if err == nil {
		return settings, "", nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return AppSettingsDTO{}, "", err
	}
	broken := path + ".broken-" + time.Now().Format("20060102-150405")
	if renameErr := os.Rename(path, broken); renameErr != nil {
		return AppSettingsDTO{}, "", fmt.Errorf("备份损坏设置失败: %w（原错误: %v）", renameErr, err)
	}
	settings = defaultAppSettings(root)
	normalizeAppSettings(&settings, root)
	return settings, fmt.Sprintf("全局设置文件无法解析，已备份为 %s 并恢复默认值", filepath.Base(broken)), nil
}

func normalizeAppSettings(settings *AppSettingsDTO, root string) {
	defaults := defaultAppSettings(root)
	if settings.SchemaVersion <= 0 {
		settings.SchemaVersion = defaults.SchemaVersion
	}
	// 0.0.10 早期构建曾写入一组错误的全局预设。它们虽然数值合法，
	// 但组合特征明确，因此只对这组旧预设做一次迁移，不覆盖用户的正常修改。
	if settings.SegmentMaxAgeSeconds == 120 &&
		settings.SegmentMaxBytes == 10*1024*1024 &&
		settings.MaxDiskBytes == 10*1024*1024 &&
		settings.NoLogTimeoutSeconds == 3000 &&
		settings.UploadConcurrency == 2 &&
		!settings.UploadGzip {
		settings.DefaultLogDirectory = defaults.DefaultLogDirectory
		settings.DefaultSaveEnabled = defaults.DefaultSaveEnabled
		settings.DefaultUploadEnabled = defaults.DefaultUploadEnabled
		settings.SegmentMaxAgeSeconds = defaults.SegmentMaxAgeSeconds
		settings.SegmentMaxBytes = defaults.SegmentMaxBytes
		settings.NoLogTimeoutSeconds = defaults.NoLogTimeoutSeconds
		settings.MaxLogLines = defaults.MaxLogLines
		settings.LogFontSize = defaults.LogFontSize
		settings.AutoWrap = defaults.AutoWrap
		settings.MaxDiskBytes = defaults.MaxDiskBytes
		settings.StorageWarningPercent = defaults.StorageWarningPercent
		settings.AutoDeleteUploaded = defaults.AutoDeleteUploaded
		settings.UploadedRetentionHours = defaults.UploadedRetentionHours
		settings.BackendURL = defaults.BackendURL
		settings.UploadIntervalSeconds = defaults.UploadIntervalSeconds
		settings.UploadConcurrency = defaults.UploadConcurrency
		settings.UploadGzip = defaults.UploadGzip
	}
	if settings.DefaultLogDirectory == "" {
		settings.DefaultLogDirectory = defaults.DefaultLogDirectory
	}
	if settings.SegmentMaxAgeSeconds <= 0 || settings.SegmentMaxAgeSeconds == 300 {
		settings.SegmentMaxAgeSeconds = defaults.SegmentMaxAgeSeconds
	}
	if settings.SegmentMaxBytes <= 0 || settings.SegmentMaxBytes == 32*1024*1024 {
		settings.SegmentMaxBytes = defaults.SegmentMaxBytes
	}
	if settings.NoLogTimeoutSeconds <= 0 {
		settings.NoLogTimeoutSeconds = defaults.NoLogTimeoutSeconds
	}
	if settings.MaxLogLines < 100 {
		settings.MaxLogLines = defaults.MaxLogLines
	}
	if settings.LogFontSize < 10 || settings.LogFontSize > 24 {
		settings.LogFontSize = defaults.LogFontSize
	}
	if settings.MaxDiskBytes <= 0 {
		settings.MaxDiskBytes = defaults.MaxDiskBytes
	}
	if settings.StorageWarningPercent < 1 || settings.StorageWarningPercent > 99 {
		settings.StorageWarningPercent = defaults.StorageWarningPercent
	}
	if settings.AutoDeleteUploaded && settings.UploadedRetentionHours <= 0 {
		settings.UploadedRetentionHours = 24
	}
	if settings.UploadIntervalSeconds <= 0 {
		settings.UploadIntervalSeconds = defaults.UploadIntervalSeconds
	}
	if settings.UploadConcurrency < 1 {
		settings.UploadConcurrency = defaults.UploadConcurrency
	}
	if strings.TrimSpace(settings.BackendURL) == "" {
		settings.BackendURL = defaults.BackendURL
	}
	if strings.TrimSpace(settings.ProgramName) == "" {
		settings.ProgramName = defaults.ProgramName
	}
	if strings.TrimSpace(settings.CompanyName) == "" {
		settings.CompanyName = defaults.CompanyName
	}
	if strings.TrimSpace(settings.CommunityTitle) == "" {
		settings.CommunityTitle = defaults.CommunityTitle
	}
	if strings.TrimSpace(settings.CommunityText) == "" {
		settings.CommunityText = defaults.CommunityText
	}
	settings.ProgramName = "LogMaster采集端"
	settings.ProgramVersion = "0.0.10"
	settings.BuildVersion = "0.0.10"
	settings.UpdateDate = "2026-08-14"
	settings.CompanyName = "上海七十迈数字科技有限公司"
}

// migrateLegacySettingsIntoUntouchedPortableConfig preserves user settings when
// upgrading from builds that stored them below %APPDATA%. A package now ships
// with a complete config file, so migration is allowed only before that file
// has been marked as opened and only when it still equals the product defaults.
func migrateLegacySettingsIntoUntouchedPortableConfig(settingsPath, legacyPath, root string) (AppSettingsDTO, bool, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return AppSettingsDTO{}, false, err
	}
	var raw AppSettingsDTO
	if err := json.Unmarshal(data, &raw); err != nil {
		return AppSettingsDTO{}, false, err
	}
	if raw.SchemaVersion > 0 {
		return AppSettingsDTO{}, false, nil
	}
	portable, err := loadAppSettings(settingsPath, root)
	if err != nil {
		return AppSettingsDTO{}, false, err
	}
	defaults := defaultAppSettings(root)
	if !sameUserSettings(portable, defaults) {
		return AppSettingsDTO{}, false, nil
	}
	legacy, err := loadAppSettings(legacyPath, root)
	if errors.Is(err, os.ErrNotExist) {
		return AppSettingsDTO{}, false, nil
	}
	if err != nil {
		return AppSettingsDTO{}, false, err
	}
	if sameUserSettings(legacy, defaults) {
		return AppSettingsDTO{}, false, nil
	}
	legacy.SchemaVersion = defaults.SchemaVersion
	return legacy, true, nil
}

func sameUserSettings(left, right AppSettingsDTO) bool {
	left.SchemaVersion, right.SchemaVersion = 0, 0
	return reflect.DeepEqual(left, right)
}

func (s *Service) GetAppSettings() AppSettingsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appSettings
}

func (s *Service) GetStartupWarnings() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.startupWarnings...)
}

func (s *Service) ReloadAppSettings() (AppSettingsDTO, error) {
	settings, err := loadAppSettings(s.settingsPath, s.rootDirectory)
	if err != nil {
		return AppSettingsDTO{}, err
	}
	if err := s.SaveAppSettings(settings); err != nil {
		return AppSettingsDTO{}, err
	}
	return settings, nil
}

func (s *Service) SaveAppSettings(settings AppSettingsDTO) error {
	normalizeAppSettings(&settings, s.rootDirectory)
	if err := writeAppSettings(s.settingsPath, settings); err != nil {
		return err
	}
	s.mu.Lock()
	s.appSettings = settings
	s.maxDiskBytes = settings.MaxDiskBytes
	s.mu.Unlock()
	if s.manager != nil {
		if err := s.manager.ApplyRuntimeConfig(settings.DefaultLogDirectory, durationSeconds(settings.SegmentMaxAgeSeconds), settings.SegmentMaxBytes, settings.MaxDiskBytes, settings.StorageWarningPercent); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.spoolDirectory = settings.DefaultLogDirectory
	s.mu.Unlock()
	if s.backendClient != nil {
		s.backendClient.ApplyConfig(backend.Config{BaseURL: settings.BackendURL, Gzip: settings.UploadGzip, Timeout: 30 * time.Second})
	}
	if s.worker != nil {
		s.worker.ApplyConfig(backend.WorkerConfig{Interval: durationSeconds(settings.UploadIntervalSeconds), Concurrency: settings.UploadConcurrency, MaxFiles: 16, BatchInterval: 200 * time.Millisecond})
	}
	return nil
}

func writeAppSettings(path string, settings AppSettingsDTO) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0o644)
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	defer os.Remove(tempPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return err
	}
	return atomicReplace(tempPath, path)
}

func durationSeconds(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}
