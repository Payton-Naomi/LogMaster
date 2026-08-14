package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"logmaster-agent/agent/internal/spool"
)

type HistoryQueryDTO struct {
	DeviceID   string `json:"deviceId"`
	ProjectID  string `json:"projectId"`
	Version    string `json:"version"`
	TestTaskID string `json:"testTaskId"`
	Search     string `json:"search"`
	State      string `json:"state"`
	From       string `json:"from"`
	To         string `json:"to"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
}
type HistoryFileDTO struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"sessionId"`
	Path          string    `json:"path"`
	FileName      string    `json:"fileName"`
	DeviceID      string    `json:"deviceId"`
	PortName      string    `json:"portName"`
	ProjectID     string    `json:"projectId"`
	ProjectName   string    `json:"projectName"`
	Version       string    `json:"version"`
	TestTaskID    string    `json:"testTaskId"`
	TestTaskName  string    `json:"testTaskName"`
	FirstSequence int64     `json:"firstSequence"`
	LastSequence  int64     `json:"lastSequence"`
	LineCount     int64     `json:"lineCount"`
	SizeBytes     int64     `json:"sizeBytes"`
	SHA256        string    `json:"sha256"`
	UploadState   string    `json:"uploadState"`
	QueryCode     string    `json:"queryCode"`
	CreatedAt     time.Time `json:"createdAt"`
	CompletedAt   time.Time `json:"completedAt"`
}
type HistoryPageDTO struct {
	Items []HistoryFileDTO `json:"items"`
	Total int64            `json:"total"`
}
type HistoryPreviewDTO struct {
	File      HistoryFileDTO `json:"file"`
	Lines     []string       `json:"lines"`
	Truncated bool           `json:"truncated"`
}
type DeviceStorageDTO struct {
	DeviceID      string     `json:"deviceId"`
	TotalBytes    int64      `json:"totalBytes"`
	FileCount     int64      `json:"fileCount"`
	PendingBytes  int64      `json:"pendingBytes"`
	UploadedBytes int64      `json:"uploadedBytes"`
	EarliestAt    *time.Time `json:"earliestAt,omitempty"`
	LatestAt      *time.Time `json:"latestAt,omitempty"`
}

func (s *Service) ListHistory(query HistoryQueryDTO) (HistoryPageDTO, error) {
	from, err := spool.ParseHistoryTime(query.From)
	if err != nil {
		return HistoryPageDTO{}, err
	}
	to, err := spool.ParseHistoryTime(query.To)
	if err != nil {
		return HistoryPageDTO{}, err
	}
	items, total, err := s.store.ListHistory(s.ctx, spool.HistoryFilter{DeviceSN: query.DeviceID, ProjectID: query.ProjectID, Version: query.Version, TestTaskID: query.TestTaskID, Search: query.Search, State: query.State, From: from, To: to, Offset: query.Offset, Limit: query.Limit})
	if err != nil {
		return HistoryPageDTO{}, err
	}
	result := HistoryPageDTO{Total: total, Items: make([]HistoryFileDTO, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, toHistoryDTO(item))
	}
	return result, nil
}

func (s *Service) ReadHistoryPreview(id string) (HistoryPreviewDTO, error) {
	record, err := s.store.GetLogFile(s.ctx, id)
	if err != nil {
		return HistoryPreviewDTO{}, err
	}
	file, err := os.Open(record.Path)
	if err != nil {
		return HistoryPreviewDTO{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 2*1024*1024))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make([]string, 0, 500)
	truncated := false
	for scanner.Scan() {
		if len(lines) >= 500 {
			truncated = true
			break
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return HistoryPreviewDTO{}, err
	}
	return HistoryPreviewDTO{File: toHistoryDTO(record), Lines: lines, Truncated: truncated}, nil
}

func (s *Service) SaveLogAs(deviceID, sessionID, scope, windowContent string) (string, error) {
	if scope != "window" && scope != "session" {
		return "", errors.New("保存范围必须是 window 或 session")
	}
	if scope == "session" {
		for _, state := range s.manager.GetDeviceStates() {
			if state.DeviceID == deviceID && state.SessionID == sessionID {
				_ = s.manager.RotateDevice(deviceID)
				break
			}
		}
	}
	cfg := s.configForDevice(deviceID)
	name := defaultExportName(cfg)
	target, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{Title: "另存日志", DefaultFilename: name, Filters: []runtime.FileFilter{{DisplayName: "日志文件", Pattern: "*.log;*.txt"}}})
	if err != nil {
		return "", err
	}
	if target == "" {
		return "", errors.New("已取消保存")
	}
	if scope == "window" {
		if err := os.WriteFile(target, []byte(windowContent), 0o644); err != nil {
			return "", err
		}
		return target, nil
	}
	files, err := s.store.SessionFiles(s.ctx, sessionID)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("本次采集尚未生成可另存的正式日志")
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	for _, record := range files {
		input, openErr := os.Open(record.Path)
		if openErr != nil {
			output.Close()
			return "", openErr
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		if copyErr != nil {
			output.Close()
			return "", copyErr
		}
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Service) ExportWindowLogs(deviceID, content string) (string, error) {
	return s.SaveLogAs(deviceID, "", "window", content)
}

func (s *Service) OpenLogFolder(path string) error {
	if strings.TrimSpace(path) == "" {
		path = s.spoolDirectory
	}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	} else if err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	command := exec.Command("explorer.exe", absolute)
	return command.Start()
}

func (s *Service) EnqueueHistoryFile(id string) error {
	_, err := s.store.EnqueueHistoryFile(s.ctx, id)
	return err
}

func (s *Service) DeleteHistoryFile(id string, deleteLocalFile bool) error {
	if !deleteLocalFile {
		_, err := s.store.DeleteLocalHistoryRecord(s.ctx, id)
		return err
	}
	record, err := s.store.GetLogFile(s.ctx, id)
	if err != nil {
		return err
	}
	if record.UploadState != "local" {
		return errors.New("只有未进入上传队列的本地文件可以删除")
	}
	temporaryPath := fmt.Sprintf("%s.deleting-%d", record.Path, time.Now().UnixNano())
	if err := os.Rename(record.Path, temporaryPath); err != nil {
		return fmt.Errorf("暂存待删除文件失败: %w", err)
	}
	if _, err := s.store.DeleteLocalHistoryRecord(s.ctx, id); err != nil {
		if restoreErr := os.Rename(temporaryPath, record.Path); restoreErr != nil {
			return fmt.Errorf("删除历史记录失败: %v; 恢复本地文件失败: %w", err, restoreErr)
		}
		return err
	}
	if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("历史记录已删除，但删除本地文件失败: %w", err)
	}
	return nil
}

func (s *Service) GetDeviceStorage() ([]DeviceStorageDTO, error) {
	byID := map[string]*DeviceStorageDTO{}
	offset := 0
	for {
		history, total, err := s.store.ListHistory(s.ctx, spool.HistoryFilter{Offset: offset, Limit: 200})
		if err != nil {
			return nil, err
		}
		for _, item := range history {
			entry := byID[item.DeviceSN]
			if entry == nil {
				entry = &DeviceStorageDTO{DeviceID: item.DeviceSN}
				byID[item.DeviceSN] = entry
			}
			entry.TotalBytes += item.SizeBytes
			entry.FileCount++
			if item.UploadState == "uploaded" {
				entry.UploadedBytes += item.SizeBytes
			} else {
				entry.PendingBytes += item.SizeBytes
			}
			if entry.EarliestAt == nil || item.CompletedAt.Before(*entry.EarliestAt) {
				value := item.CompletedAt
				entry.EarliestAt = &value
			}
			if entry.LatestAt == nil || item.CompletedAt.After(*entry.LatestAt) {
				value := item.CompletedAt
				entry.LatestAt = &value
			}
		}
		offset += len(history)
		if int64(offset) >= total || len(history) == 0 {
			break
		}
	}
	result := make([]DeviceStorageDTO, 0, len(byID))
	for _, entry := range byID {
		result = append(result, *entry)
	}
	return result, nil
}

func toHistoryDTO(item spool.LogFileRecord) HistoryFileDTO {
	return HistoryFileDTO{ID: item.ID, SessionID: item.SessionID, Path: item.Path, FileName: item.FileName, DeviceID: item.DeviceSN, PortName: item.PortName, ProjectID: item.ProjectID, ProjectName: item.ProjectName, Version: item.Version, TestTaskID: item.TestTaskID, TestTaskName: item.TestTaskName, FirstSequence: item.FirstSequence, LastSequence: item.LastSequence, LineCount: item.LineCount, SizeBytes: item.SizeBytes, SHA256: item.SHA256, UploadState: item.UploadState, QueryCode: item.QueryCode, CreatedAt: item.CreatedAt, CompletedAt: item.CompletedAt}
}

func (s *Service) configForDevice(id string) DeviceConfigDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configs[id]
}

var invalidExportName = regexp.MustCompile(`[<>:"/\\|?*]+`)

func defaultExportName(cfg DeviceConfigDTO) string {
	parts := []string{cfg.PortName, cfg.ProjectName, cfg.Version, cfg.TestTaskName, time.Now().Format("20060102_150405")}
	for i := range parts {
		parts[i] = strings.TrimSpace(invalidExportName.ReplaceAllString(parts[i], "_"))
		if parts[i] == "" {
			parts[i] = "未配置"
		}
	}
	return strings.Join(parts, "-") + ".log"
}
