package model

import "time"

type LogEntry struct {
	Sequence   int64     `json:"sequence"`
	CapturedAt time.Time `json:"captured_at"`
	Content    string    `json:"content"`
	DeviceSN   string    `json:"-"`
}

type UploadBatch struct {
	BatchID   string     `json:"batch_id"`
	AgentID   string     `json:"agent_id"`
	ProjectID string     `json:"project_id"`
	DeviceSN  string     `json:"device_sn"`
	SentAt    time.Time  `json:"sent_at"`
	Logs      []LogEntry `json:"logs"`
}

type UploadResponse struct {
	Accepted bool   `json:"accepted"`
	BatchID  string `json:"batch_id"`
	Received int    `json:"received"`
}
