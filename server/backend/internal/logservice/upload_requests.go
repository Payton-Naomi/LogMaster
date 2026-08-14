package logservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"logmaster-agent/internal/response"
)

type UploadSessionRequest struct {
	ClientRequestID     string   `json:"client_request_id"`
	DeviceID            string   `json:"device_id"`
	Name                string   `json:"name"`
	PortName            string   `json:"port_name"`
	BaudRate            int      `json:"baud_rate"`
	DataBits            int      `json:"data_bits"`
	StopBits            int      `json:"stop_bits"`
	Parity              string   `json:"parity"`
	Handshake           string   `json:"handshake"`
	DTR                 bool     `json:"dtr"`
	RTS                 bool     `json:"rts"`
	ProjectID           string   `json:"project_id"`
	ProjectName         string   `json:"project_name"`
	Version             string   `json:"version"`
	TestTaskID          string   `json:"test_task_id"`
	TestTaskName        string   `json:"test_task_name"`
	UploaderName        string   `json:"uploader_name"`
	Remark              string   `json:"remark"`
	ScenarioIDs         []string `json:"scenario_ids"`
	KeywordProfileID    string   `json:"keyword_profile_id"`
	KeywordRuleIDs      []string `json:"keyword_rule_ids"`
	KeywordMatching     bool     `json:"keyword_matching_enabled"`
	SaveEnabled         bool     `json:"save_enabled"`
	UploadEnabled       bool     `json:"upload_enabled"`
	NoLogTimeoutSeconds int      `json:"no_log_timeout_seconds"`
	VID                 string   `json:"vid"`
	PID                 string   `json:"pid"`
	USBSerial           string   `json:"usb_serial"`
	Location            string   `json:"location"`
	CollectorVersion    string   `json:"collector_version"`
	Timezone            string   `json:"timezone"`
}

type UploadSession struct {
	ID              string    `json:"upload_session_id"`
	QueryCode       string    `json:"query_code"`
	ClientRequestID string    `json:"client_request_id"`
	ProjectID       string    `json:"project_id"`
	ProjectName     string    `json:"project_name"`
	Version         string    `json:"version"`
	TestTaskID      string    `json:"test_task_id"`
	TestTaskName    string    `json:"test_task_name"`
	UploaderName    string    `json:"uploader_name"`
	StorageRoot     string    `json:"-"`
	ConfigSnapshot  []byte    `json:"-"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s *Service) uploadRequestHandler(w http.ResponseWriter, r *http.Request) {
	s.uploadSessionHandler(w, r)
}

func (s *Service) uploadSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	owner, ok := s.uploadUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	var request UploadSessionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload session request")
		return
	}
	normalizeUploadSessionRequest(&request)
	request.ProjectID = numericProjectID(request.ProjectID)
	if err := validateUploadSessionRequest(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	snapshot, err := json.Marshal(request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode upload session snapshot failed")
		return
	}
	sessionID, queryCode := newID(), newQueryCode(request.ProjectName)
	root := filepath.Join(s.config.StorageDir, "sessions", queryCode)
	if err := os.MkdirAll(root, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "create upload session storage failed")
		return
	}
	session, created, err := s.repo.CreateOrGetUploadSession(r.Context(), sessionID, queryCode, owner, request, snapshot, root)
	if err != nil {
		os.RemoveAll(root)
		if errors.Is(err, ErrProjectNotFound) {
			writeError(w, http.StatusBadRequest, "project does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "create upload session failed")
		return
	}
	if !created && session.StorageRoot != root {
		os.RemoveAll(root)
	}
	response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "upload session created", Data: session})
}

func (s *Service) completeUploadSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/complete") {
		methodNotAllowed(w)
		return
	}
	owner, ok := s.uploadUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/upload-sessions/"), "/complete")
	if strings.TrimSpace(id) == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "invalid upload session id")
		return
	}
	if err := s.repo.CloseUploadSession(r.Context(), id, owner); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "upload session not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "close upload session failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "upload session closed", Data: map[string]any{"upload_session_id": id, "status": "closed"}})
}

func normalizeUploadSessionRequest(request *UploadSessionRequest) {
	request.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	request.Name = strings.TrimSpace(request.Name)
	request.PortName = strings.TrimSpace(request.PortName)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.ProjectName = strings.TrimSpace(request.ProjectName)
	request.Version = strings.TrimSpace(request.Version)
	request.TestTaskID = strings.TrimSpace(request.TestTaskID)
	request.TestTaskName = strings.TrimSpace(request.TestTaskName)
	request.UploaderName = strings.TrimSpace(request.UploaderName)
	request.Remark = strings.TrimSpace(request.Remark)
	request.CollectorVersion = strings.TrimSpace(request.CollectorVersion)
	request.Timezone = strings.TrimSpace(request.Timezone)
}

func validateUploadSessionRequest(request UploadSessionRequest) error {
	required := []struct{ name, value string }{
		{"client_request_id", request.ClientRequestID}, {"project_name", request.ProjectName},
		{"version", request.Version}, {"uploader_name", request.UploaderName},
	}
	for _, field := range required {
		if field.value == "" {
			return errors.New(field.name + " is required")
		}
	}
	if len(request.ClientRequestID) > 128 || len(request.ProjectName) > 128 || len(request.Version) > 64 || len(request.UploaderName) > 128 {
		return errors.New("upload session field is too long")
	}
	return nil
}

func numericProjectID(value string) string {
	for _, character := range strings.TrimSpace(value) {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return strings.TrimSpace(value)
}

func (r *Repository) CreateOrGetUploadSession(ctx context.Context, sessionID, queryCode, owner string, request UploadSessionRequest, snapshot []byte, root string) (UploadSession, bool, error) {
	var projectID int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM logmaster_api.projects WHERE name=$1 AND ($2='' OR id::text=$2) AND is_active=TRUE`, request.ProjectName, request.ProjectID).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadSession{}, false, ErrProjectNotFound
	}
	if err != nil {
		return UploadSession{}, false, err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.upload_sessions
		(id,query_code,created_by_open_id,client_request_id,project_id,project_name,version,test_task_id,test_task_name,uploader_name,config_snapshot,storage_root)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT(created_by_open_id,client_request_id) DO NOTHING`, sessionID, queryCode, owner, request.ClientRequestID,
		projectID, request.ProjectName, request.Version, request.TestTaskID, request.TestTaskName, request.UploaderName, snapshot, root)
	if err != nil {
		return UploadSession{}, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return UploadSession{}, false, err
	}
	var session UploadSession
	err = r.db.QueryRowContext(ctx, `SELECT id,query_code,client_request_id,project_id::text,project_name,version,test_task_id,test_task_name,uploader_name,storage_root,config_snapshot,status,created_at
		FROM logmaster_api.upload_sessions WHERE created_by_open_id=$1 AND client_request_id=$2`, owner, request.ClientRequestID).Scan(
		&session.ID, &session.QueryCode, &session.ClientRequestID, &session.ProjectID, &session.ProjectName, &session.Version,
		&session.TestTaskID, &session.TestTaskName, &session.UploaderName, &session.StorageRoot, &session.ConfigSnapshot, &session.Status, &session.CreatedAt)
	return session, rows == 1, err
}

func (r *Repository) GetUploadSessionForUpload(ctx context.Context, id, queryCode, owner string) (UploadSession, error) {
	var session UploadSession
	err := r.db.QueryRowContext(ctx, `SELECT id,query_code,client_request_id,project_id::text,project_name,version,test_task_id,test_task_name,uploader_name,storage_root,config_snapshot,status,created_at
		FROM logmaster_api.upload_sessions WHERE id=$1 AND query_code=$2 AND created_by_open_id=$3`, id, queryCode, owner).Scan(
		&session.ID, &session.QueryCode, &session.ClientRequestID, &session.ProjectID, &session.ProjectName, &session.Version,
		&session.TestTaskID, &session.TestTaskName, &session.UploaderName, &session.StorageRoot, &session.ConfigSnapshot, &session.Status, &session.CreatedAt)
	return session, err
}

func (r *Repository) CloseUploadSession(ctx context.Context, id, owner string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.upload_sessions SET status='closed',closed_at=COALESCE(closed_at,NOW()),updated_at=NOW() WHERE id=$1 AND created_by_open_id=$2`, id, owner)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return err
}
