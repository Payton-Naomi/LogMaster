package spool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Session struct {
	ID               string
	DeviceSN         string
	PortName         string
	ProjectID        string
	ProjectName      string
	Version          string
	TestTaskID       string
	TestTaskName     string
	UploaderName     string
	UploaderEmail    string
	Remark           string
	ScenarioIDs      []string
	CollectorVersion string
	Timezone         string
	SaveEnabled      bool
	UploadEnabled    bool
	StartedAt        time.Time
	EndedAt          *time.Time
}

type LogFileRecord struct {
	ID             string
	SessionID      string
	Path           string
	FileName       string
	DeviceSN       string
	PortName       string
	ProjectID      string
	ProjectName    string
	Version        string
	TestTaskID     string
	TestTaskName   string
	FirstSequence  int64
	LastSequence   int64
	LineCount      int64
	SizeBytes      int64
	SHA256         string
	UploadEligible bool
	CreatedAt      time.Time
	CompletedAt    time.Time
	UploadState    string
	QueryCode      string
}

type HistoryFilter struct {
	DeviceSN   string
	ProjectID  string
	Version    string
	TestTaskID string
	Search     string
	State      string
	From       *time.Time
	To         *time.Time
	Offset     int
	Limit      int
}

type KeywordHit struct {
	ID        string
	SessionID string
	DeviceSN  string
	RuleID    string
	RuleName  string
	MatchedAt time.Time
	Sequence  int64
	LineText  string
}

func (s *Store) StartSession(ctx context.Context, session Session) (string, error) {
	if strings.TrimSpace(session.ID) == "" {
		id, err := newID()
		if err != nil {
			return "", err
		}
		session.ID = id
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = s.now()
	}
	scenarioJSON, err := json.Marshal(session.ScenarioIDs)
	if err != nil {
		return "", fmt.Errorf("encode session scenario ids: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO collection_sessions(session_id,device_sn,port_name,project_id,project_name,version,test_task_id,test_task_name,save_enabled,upload_enabled,started_at,uploader_name,uploader_email,remark,collector_version,timezone,scenario_ids_json)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(session_id) DO NOTHING`, session.ID, session.DeviceSN, session.PortName, session.ProjectID, session.ProjectName, session.Version, session.TestTaskID, session.TestTaskName, boolInt(session.SaveEnabled), boolInt(session.UploadEnabled), session.StartedAt.UTC().Format(time.RFC3339Nano), session.UploaderName, strings.ToLower(strings.TrimSpace(session.UploaderEmail)), session.Remark, session.CollectorVersion, session.Timezone, string(scenarioJSON))
	return session.ID, err
}

func (s *Store) EndSession(ctx context.Context, id string, endedAt time.Time) error {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	if endedAt.IsZero() {
		endedAt = s.now()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE collection_sessions SET ended_at=COALESCE(ended_at,?) WHERE session_id=?`, endedAt.UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) RegisterLogFile(ctx context.Context, session Session, file LogFileRecord) (string, error) {
	if _, err := s.StartSession(ctx, session); err != nil {
		return "", err
	}
	if strings.TrimSpace(file.ID) == "" {
		id, err := newID()
		if err != nil {
			return "", err
		}
		file.ID = id
	}
	file.SessionID = session.ID
	if file.FileName == "" {
		file.FileName = filepath.Base(file.Path)
	}
	if file.LineCount == 0 && file.LastSequence >= file.FirstSequence {
		file.LineCount = file.LastSequence - file.FirstSequence + 1
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = s.now()
	}
	if file.CompletedAt.IsZero() {
		file.CompletedAt = s.now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO log_files(log_file_id,session_id,file_path,file_name,device_sn,port_name,project_id,project_name,version,test_task_id,test_task_name,first_sequence,last_sequence,line_count,size_bytes,sha256,upload_eligible,created_at,completed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(file_path) DO UPDATE SET size_bytes=excluded.size_bytes,sha256=excluded.sha256,line_count=excluded.line_count,completed_at=excluded.completed_at`, file.ID, file.SessionID, file.Path, file.FileName, file.DeviceSN, file.PortName, session.ProjectID, session.ProjectName, session.Version, session.TestTaskID, session.TestTaskName, file.FirstSequence, file.LastSequence, file.LineCount, file.SizeBytes, strings.ToLower(file.SHA256), boolInt(file.UploadEligible), file.CreatedAt.UTC().Format(time.RFC3339Nano), file.CompletedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return "", err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT log_file_id FROM log_files WHERE file_path=?`, file.Path).Scan(&file.ID); err != nil {
		return "", err
	}
	return file.ID, nil
}

func (s *Store) ListHistory(ctx context.Context, filter HistoryFilter) ([]LogFileRecord, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	if filter.DeviceSN != "" {
		where = append(where, "f.device_sn=?")
		args = append(args, filter.DeviceSN)
	}
	if filter.ProjectID != "" {
		where = append(where, "f.project_id=?")
		args = append(args, filter.ProjectID)
	}
	if filter.Version != "" {
		where = append(where, "f.version=?")
		args = append(args, filter.Version)
	}
	if filter.TestTaskID != "" {
		where = append(where, "f.test_task_id=?")
		args = append(args, filter.TestTaskID)
	}
	if filter.Search != "" {
		where = append(where, "(f.file_name LIKE ? OR f.sha256 LIKE ? OR COALESCE(b.query_code,'') LIKE ?)")
		term := "%" + filter.Search + "%"
		args = append(args, term, term, term)
	}
	if filter.State != "" {
		where = append(where, "COALESCE(b.state,'local')=?")
		args = append(args, filter.State)
	}
	if filter.From != nil {
		where = append(where, "f.completed_at>=?")
		args = append(args, filter.From.UTC().Format(time.RFC3339Nano))
	}
	if filter.To != nil {
		where = append(where, "f.completed_at<=?")
		args = append(args, filter.To.UTC().Format(time.RFC3339Nano))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM log_files f LEFT JOIN upload_files uf ON uf.log_file_id=f.log_file_id LEFT JOIN upload_batches b ON b.local_batch_id=uf.local_batch_id WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	query := `SELECT f.log_file_id,f.session_id,f.file_path,f.file_name,f.device_sn,f.port_name,f.project_id,f.project_name,f.version,f.test_task_id,f.test_task_name,f.first_sequence,f.last_sequence,f.line_count,f.size_bytes,f.sha256,f.upload_eligible,f.created_at,f.completed_at,COALESCE(b.state,'local'),COALESCE(b.query_code,'') FROM log_files f LEFT JOIN upload_files uf ON uf.log_file_id=f.log_file_id LEFT JOIN upload_batches b ON b.local_batch_id=uf.local_batch_id WHERE ` + clause + ` ORDER BY f.completed_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var result []LogFileRecord
	for rows.Next() {
		var item LogFileRecord
		var created, completed string
		var eligible int
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Path, &item.FileName, &item.DeviceSN, &item.PortName, &item.ProjectID, &item.ProjectName, &item.Version, &item.TestTaskID, &item.TestTaskName, &item.FirstSequence, &item.LastSequence, &item.LineCount, &item.SizeBytes, &item.SHA256, &eligible, &created, &completed, &item.UploadState, &item.QueryCode); err != nil {
			return nil, 0, err
		}
		item.UploadEligible = eligible != 0
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		item.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		result = append(result, item)
	}
	return result, total, rows.Err()
}

func (s *Store) SessionFiles(ctx context.Context, sessionID string) ([]LogFileRecord, error) {
	return s.listFiles(ctx, `f.session_id=?`, sessionID)
}

func (s *Store) GetLogFile(ctx context.Context, id string) (LogFileRecord, error) {
	items, err := s.listFiles(ctx, `f.log_file_id=?`, id)
	if err != nil {
		return LogFileRecord{}, err
	}
	if len(items) == 0 {
		return LogFileRecord{}, sql.ErrNoRows
	}
	return items[0], nil
}

func (s *Store) listFiles(ctx context.Context, clause string, arg any) ([]LogFileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.log_file_id,f.session_id,f.file_path,f.file_name,f.device_sn,f.port_name,f.project_id,f.project_name,f.version,f.test_task_id,f.test_task_name,f.first_sequence,f.last_sequence,f.line_count,f.size_bytes,f.sha256,f.upload_eligible,f.created_at,f.completed_at,COALESCE(b.state,'local'),COALESCE(b.query_code,'') FROM log_files f LEFT JOIN upload_files uf ON uf.log_file_id=f.log_file_id LEFT JOIN upload_batches b ON b.local_batch_id=uf.local_batch_id WHERE `+clause+` ORDER BY f.first_sequence`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []LogFileRecord
	for rows.Next() {
		var item LogFileRecord
		var created, completed string
		var eligible int
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Path, &item.FileName, &item.DeviceSN, &item.PortName, &item.ProjectID, &item.ProjectName, &item.Version, &item.TestTaskID, &item.TestTaskName, &item.FirstSequence, &item.LastSequence, &item.LineCount, &item.SizeBytes, &item.SHA256, &eligible, &created, &completed, &item.UploadState, &item.QueryCode); err != nil {
			return nil, err
		}
		item.UploadEligible = eligible != 0
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		item.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) RecordKeywordHit(ctx context.Context, hit KeywordHit) (string, error) {
	if hit.ID == "" {
		id, err := newID()
		if err != nil {
			return "", err
		}
		hit.ID = id
	}
	if hit.MatchedAt.IsZero() {
		hit.MatchedAt = s.now()
	}
	hit.LineText = truncateKeywordLine(hit.LineText)
	_, err := s.db.ExecContext(ctx, `INSERT INTO keyword_hits(hit_id,session_id,device_sn,rule_id,rule_name,matched_at,sequence,line_text) VALUES(?,?,?,?,?,?,?,?)`, hit.ID, hit.SessionID, hit.DeviceSN, hit.RuleID, hit.RuleName, hit.MatchedAt.UTC().Format(time.RFC3339Nano), hit.Sequence, hit.LineText)
	return hit.ID, err
}

const maxKeywordHitLineRunes = 256

func truncateKeywordLine(text string) string {
	runes := []rune(text)
	if len(runes) <= maxKeywordHitLineRunes {
		return text
	}
	return string(runes[:maxKeywordHitLineRunes]) + "…"
}

func (s *Store) ListKeywordHits(ctx context.Context, sessionID, ruleID string) ([]KeywordHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT hit_id,session_id,device_sn,rule_id,rule_name,matched_at,sequence,line_text FROM keyword_hits WHERE session_id=? AND (?='' OR rule_id=?) ORDER BY matched_at`, sessionID, ruleID, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []KeywordHit
	for rows.Next() {
		var h KeywordHit
		var at string
		if err := rows.Scan(&h.ID, &h.SessionID, &h.DeviceSN, &h.RuleID, &h.RuleName, &at, &h.Sequence, &h.LineText); err != nil {
			return nil, err
		}
		h.MatchedAt, _ = time.Parse(time.RFC3339Nano, at)
		result = append(result, h)
	}
	return result, rows.Err()
}

func (s *Store) ResetKeywordHits(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM keyword_hits WHERE session_id=?`, sessionID)
	return err
}

// PruneKeywordHits removes keyword-hit records older than before, bounding the
// growth of the hits table on long-running collection benches.
func (s *Store) PruneKeywordHits(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM keyword_hits WHERE matched_at<=?`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) StorageByDevice(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT device_sn,COALESCE(SUM(size_bytes),0) FROM log_files GROUP BY device_sn`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var id string
		var size int64
		if err := rows.Scan(&id, &size); err != nil {
			return nil, err
		}
		result[id] = size
	}
	return result, rows.Err()
}

func (s *Store) EnqueueHistoryFile(ctx context.Context, id string) (string, error) {
	record, err := s.GetLogFile(ctx, id)
	if err != nil {
		return "", err
	}
	if record.UploadState != "local" {
		return "", errors.New("history file already has an upload state")
	}
	return s.EnqueueFile(ctx, record.ProjectName, record.Version, File{LogFileID: record.ID, SessionID: record.SessionID, Path: record.Path, SHA256: record.SHA256, SizeBytes: record.SizeBytes, DeviceSN: record.DeviceSN, FirstSequence: record.FirstSequence, LastSequence: record.LastSequence})
}

func (s *Store) DeleteLocalHistoryRecord(ctx context.Context, id string) (LogFileRecord, error) {
	record, err := s.GetLogFile(ctx, id)
	if err != nil {
		return LogFileRecord{}, err
	}
	if record.UploadState != "local" {
		return LogFileRecord{}, errors.New("only local history files can be deleted")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM log_files
		WHERE log_file_id=? AND NOT EXISTS (SELECT 1 FROM upload_files WHERE log_file_id=?)`, id, id)
	if err != nil {
		return LogFileRecord{}, err
	}
	if err := requireUpdated(result, nil); err != nil {
		return LogFileRecord{}, errors.New("history file entered the upload queue and cannot be deleted")
	}
	return record, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ParseHistoryTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parse history time: %w", err)
	}
	return &parsed, nil
}
