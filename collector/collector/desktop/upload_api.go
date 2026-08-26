package main

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/spool"
)

type UploadQueueQueryDTO struct {
	DeviceID        string   `json:"deviceId"`
	States          []string `json:"states"`
	Search          string   `json:"search"`
	IncludeUploaded bool     `json:"includeUploaded"`
	Offset          int      `json:"offset"`
	Limit           int      `json:"limit"`
}
type UploadQueuePageDTO struct {
	Items []UploadTaskDTO `json:"items"`
	Total int64           `json:"total"`
}
type UploadProgressDTO struct {
	BatchID      string    `json:"batchId"`
	SentBytes    int64     `json:"sentBytes"`
	TotalBytes   int64     `json:"totalBytes"`
	SpeedBytes   int64     `json:"speedBytes"`
	StartedAt    time.Time `json:"startedAt"`
	AttemptCount int       `json:"attemptCount"`
}

func (s *Service) GetUploadQueue(query UploadQueueQueryDTO) (UploadQueuePageDTO, error) {
	states := make([]spool.State, 0, len(query.States))
	for _, state := range query.States {
		states = append(states, spool.State(state))
	}
	batches, total, err := s.store.ListBatches(s.ctx, spool.BatchFilter{DeviceSN: query.DeviceID, States: states, Search: query.Search, IncludeUploaded: query.IncludeUploaded, Offset: query.Offset, Limit: query.Limit})
	if err != nil {
		return UploadQueuePageDTO{}, err
	}
	_ = total // Task count is calculated after grouping instead of raw batch count.
	result := UploadQueuePageDTO{Items: s.toUploadTasks(batches)}
	result.Total = int64(len(result.Items))
	return result, nil
}

func uploadTaskKey(batch spool.Batch) string {
	if id := strings.TrimSpace(batch.UploadSessionID); id != "" {
		return "upload-session:" + id
	}
	if id := strings.TrimSpace(batch.SessionID); id != "" {
		return "capture-session:" + id
	}
	device := ""
	if len(batch.Files) > 0 {
		device = batch.Files[0].DeviceSN
	}
	return "legacy:" + strings.Join([]string{batch.ProjectID, batch.UploaderEmail, batch.TestTaskID, device, batch.QueryCode}, "|")
}

func taskStatePriority(state spool.State) int {
	switch state {
	case spool.Dead:
		return 5
	case spool.Uncertain:
		return 4
	case spool.Uploading:
		return 3
	case spool.Pending:
		return 2
	default:
		return 1
	}
}

func (s *Service) toUploadTasks(batches []spool.Batch) []UploadTaskDTO {
	grouped := map[string]*UploadTaskDTO{}
	priorities := map[string]int{}
	for _, batch := range batches {
		key := uploadTaskKey(batch)
		item := grouped[key]
		if item == nil {
			deviceID, portName := "", ""
			if len(batch.Files) > 0 {
				deviceID = batch.Files[0].DeviceSN
				portName = deviceID
			}
			item = &UploadTaskDTO{ID: key, State: string(batch.State), ProjectName: batch.ProjectName, UploaderName: batch.UploaderName, UploaderEmail: batch.UploaderEmail, TestTaskName: batch.TestTaskName, PortName: portName, DeviceID: deviceID, QueryCode: batch.QueryCode, CreatedAt: batch.CreatedAt}
			grouped[key], priorities[key] = item, taskStatePriority(batch.State)
		}
		item.BatchIDs = append(item.BatchIDs, batch.ID)
		item.FileCount += len(batch.Files)
		item.BytesTotal += batch.BytesTotal
		if item.ProjectName == "" {
			item.ProjectName = batch.ProjectName
		}
		if item.UploaderName == "" {
			item.UploaderName = batch.UploaderName
		}
		if item.UploaderEmail == "" {
			item.UploaderEmail = batch.UploaderEmail
		}
		if item.TestTaskName == "" {
			item.TestTaskName = batch.TestTaskName
		}
		if item.QueryCode == "" {
			item.QueryCode = batch.QueryCode
		}
		if batch.CreatedAt.Before(item.CreatedAt) {
			item.CreatedAt = batch.CreatedAt
		}
		if taskStatePriority(batch.State) > priorities[key] {
			item.State, item.LastError, priorities[key] = string(batch.State), batch.LastError, taskStatePriority(batch.State)
		}
		if batch.State == spool.Dead || batch.State == spool.Uncertain {
			if item.LastError == "" {
				item.LastError = batch.LastError
			}
		}
		dto := s.toUploadBatchDTO(batch)
		item.BytesSent += dto.BytesSent
		item.SpeedBytes += dto.SpeedBytes
		if batch.CompletedAt != nil && (item.CompletedAt == nil || batch.CompletedAt.After(*item.CompletedAt)) {
			value := *batch.CompletedAt
			item.CompletedAt = &value
		}
	}
	items := make([]UploadTaskDTO, 0, len(grouped))
	for _, item := range grouped {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

type UploadTaskActionResult struct {
	Succeeded int `json:"succeeded"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
}

func (s *Service) RetryUploadTask(task UploadTaskDTO) (UploadTaskActionResult, error) {
	result := UploadTaskActionResult{}
	for _, id := range task.BatchIDs {
		batch, err := s.store.GetBatch(s.ctx, id)
		if err != nil {
			result.Failed++
			continue
		}
		if batch.State == spool.Dead {
			err = s.RetryDeadBatch(id)
		} else if batch.State == spool.Uncertain {
			err = s.RetryUncertain(id)
		} else {
			result.Skipped++
			continue
		}
		if err != nil {
			result.Failed++
		} else {
			result.Succeeded++
		}
	}
	return result, nil
}

// ClearUploadHistory removes local upload tasks only. It deliberately leaves
// source log files and local collection history untouched.
func (s *Service) ClearUploadHistory() (int, error) {
	deleted, err := s.store.ClearUploadHistory(s.ctx)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.uploadProgress = map[string]UploadProgressDTO{}
	s.mu.Unlock()
	return deleted, nil
}

func (s *Service) toUploadBatchDTO(batch spool.Batch) UploadBatchDTO {
	item := UploadBatchDTO{ID: batch.ID, State: string(batch.State), AttemptCount: batch.AttemptCount, LastError: batch.LastError, CreatedAt: batch.CreatedAt, ProjectName: batch.ProjectName, Version: batch.Version, SessionID: batch.SessionID, QueryCode: batch.QueryCode, UploadPosition: batch.UploadPosition, BytesTotal: batch.BytesTotal, StartedAt: batch.StartedAt, CompletedAt: batch.CompletedAt}
	if len(batch.Files) > 0 {
		item.DeviceID = batch.Files[0].DeviceSN
		item.FileName = filepath.Base(batch.Files[0].Path)
		item.SizeBytes = batch.Files[0].SizeBytes
		item.SHA256 = batch.Files[0].SHA256
	}
	s.mu.RLock()
	progress, ok := s.uploadProgress[batch.ID]
	s.mu.RUnlock()
	if ok {
		item.BytesSent = progress.SentBytes
		item.BytesTotal = progress.TotalBytes
		item.SpeedBytes = progress.SpeedBytes
		if item.StartedAt == nil {
			value := progress.StartedAt
			item.StartedAt = &value
		}
	} else if batch.State == spool.Uploaded {
		item.BytesSent = item.BytesTotal
	}
	return item
}

func (s *Service) handleUploadProgress(progress backend.UploadProgress) {
	dto := UploadProgressDTO{BatchID: progress.BatchID, SentBytes: progress.SentBytes, TotalBytes: progress.TotalBytes, SpeedBytes: progress.SpeedBytes, StartedAt: progress.StartedAt, AttemptCount: progress.AttemptCount}
	s.mu.Lock()
	s.uploadProgress[progress.BatchID] = dto
	s.mu.Unlock()
	runtime.EventsEmit(s.ctx, "upload:progress", dto)
}
