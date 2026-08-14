package main

import (
	"path/filepath"
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
	Items []UploadBatchDTO `json:"items"`
	Total int64            `json:"total"`
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
	result := UploadQueuePageDTO{Total: total, Items: make([]UploadBatchDTO, 0, len(batches))}
	for _, batch := range batches {
		result.Items = append(result.Items, s.toUploadBatchDTO(batch))
	}
	return result, nil
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
