package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AppSettingsDTO struct {
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
	return AppSettingsDTO{DefaultLogDirectory: directory, DefaultSaveEnabled: d.Log.SaveEnabled, DefaultUploadEnabled: d.Log.UploadEnabled, SegmentMaxAgeSeconds: d.Log.SegmentMinutes * 60, SegmentMaxBytes: d.Log.SegmentSizeMB * 1024 * 1024, NoLogTimeoutSeconds: d.Log.NoLogTimeoutSeconds, MaxLogLines: d.Log.MaxWindowLines, LogFontSize: d.Log.FontSize, AutoWrap: d.Log.AutoWrap, MaxDiskBytes: d.Storage.LimitGB * 1024 * 1024 * 1024, StorageWarningPercent: d.Storage.WarningPercent, AutoDeleteUploaded: false, UploadedRetentionHours: 0, BackendURL: d.Upload.BackendURL, UploadIntervalSeconds: d.Upload.IntervalSeconds, UploadConcurrency: d.Upload.Concurrency, UploadGzip: d.Upload.Gzip, ProgramName: "LogMaster采集端", ProgramVersion: "0.0.10", BuildVersion: "0.0.10", UpdateDate: "2026-08-14", CompanyName: "上海七十迈数字科技有限公司", CommunityTitle: "飞书交流群", CommunityText: "使用说明、问题反馈、获取最新版本请扫码加入飞书交流群。"}
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

func normalizeAppSettings(settings *AppSettingsDTO, root string) {
	if settings.DefaultLogDirectory == "" {
		settings.DefaultLogDirectory = defaultLogDirectory(root)
	}
	if settings.SegmentMaxAgeSeconds <= 0 || settings.SegmentMaxAgeSeconds == 300 {
		settings.SegmentMaxAgeSeconds = 1800
	}
	if settings.SegmentMaxBytes <= 0 || settings.SegmentMaxBytes == 32*1024*1024 {
		settings.SegmentMaxBytes = 128 * 1024 * 1024
	}
	if settings.NoLogTimeoutSeconds <= 0 {
		settings.NoLogTimeoutSeconds = 300
	}
	if settings.MaxLogLines < 100 {
		settings.MaxLogLines = 2000
	}
	if settings.LogFontSize < 10 || settings.LogFontSize > 24 {
		settings.LogFontSize = 12
	}
	if settings.MaxDiskBytes <= 0 {
		settings.MaxDiskBytes = 20 * 1024 * 1024 * 1024
	}
	if settings.StorageWarningPercent < 1 || settings.StorageWarningPercent > 99 {
		settings.StorageWarningPercent = 80
	}
	if settings.AutoDeleteUploaded && settings.UploadedRetentionHours <= 0 {
		settings.UploadedRetentionHours = 24
	}
	if settings.UploadIntervalSeconds <= 0 {
		settings.UploadIntervalSeconds = 300
	}
	if settings.UploadConcurrency < 1 {
		settings.UploadConcurrency = 2
	}
	settings.ProgramName = "LogMaster采集端"
	settings.ProgramVersion = "0.0.10"
	settings.BuildVersion = "0.0.10"
	settings.UpdateDate = "2026-08-14"
	settings.CompanyName = "上海七十迈数字科技有限公司"
}

func (s *Service) GetAppSettings() AppSettingsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appSettings
}

func (s *Service) SaveAppSettings(settings AppSettingsDTO) error {
	normalizeAppSettings(&settings, s.rootDirectory)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := atomicWriteFile(s.settingsPath, data, 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.appSettings = settings
	s.maxDiskBytes = settings.MaxDiskBytes
	s.mu.Unlock()
	return nil
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
