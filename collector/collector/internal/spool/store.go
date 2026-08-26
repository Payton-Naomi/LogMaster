package spool

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type State string

const (
	Pending   State = "pending"
	Uploading State = "uploading"
	Uploaded  State = "uploaded"
	Uncertain State = "uncertain"
	Dead      State = "dead"
)

type File struct {
	LogFileID     string
	SessionID     string
	Path          string
	SHA256        string
	SizeBytes     int64
	DeviceSN      string
	FirstSequence int64
	LastSequence  int64
}

type Batch struct {
	ID               string
	ProjectID        string
	ProjectName      string
	Version          string
	TestTaskID       string
	TestTaskName     string
	UploaderName     string
	UploaderEmail    string
	Remark           string
	ClientRequestID  string
	CollectorVersion string
	Timezone         string
	SourceCreatedAt  *time.Time
	SourceStartedAt  *time.Time
	SourceEndedAt    *time.Time
	ScenarioIDs      []string
	State            State
	AttemptCount     int
	NextAttemptAt    time.Time
	UploadID         string
	TaskID           string
	QueryCode        string
	UploadSessionID  string
	UploadPosition   int
	ConfigSnapshot   string
	SessionID        string
	BytesTotal       int64
	StartedAt        *time.Time
	CompletedAt      *time.Time
	LastError        string
	CreatedAt        time.Time
	UploadedAt       *time.Time
	Files            []File
}

type UploadMetadata struct {
	ProjectID        string
	ProjectName      string
	Version          string
	TestTaskID       string
	TestTaskName     string
	UploaderName     string
	UploaderEmail    string
	Remark           string
	CollectorVersion string
	Timezone         string
	CreatedAt        *time.Time
	StartedAt        *time.Time
	EndedAt          *time.Time
	ScenarioIDs      []string
	UploadSessionID  string
	QueryCode        string
	ConfigSnapshot   string
}

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite permits one writer at a time. A single pooled connection keeps all
	// state transitions ordered and prevents healthy collector sessions from
	// being misclassified as serial failures when concurrent sequence writes race.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=FULL`,
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS sequences (
			device_sn TEXT PRIMARY KEY,
			next_value INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS upload_batches (
			local_batch_id TEXT PRIMARY KEY,
			project_name TEXT NOT NULL,
			version TEXT NOT NULL,
			state TEXT NOT NULL CHECK(state IN ('pending','uploading','uploaded','uncertain','dead')),
			attempt_count INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TEXT NOT NULL,
			upload_id TEXT,
			task_id TEXT,
			last_error TEXT,
			created_at TEXT NOT NULL,
			uploaded_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS upload_files (
			local_batch_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			sha256 TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			device_sn TEXT NOT NULL,
			first_sequence INTEGER NOT NULL,
			last_sequence INTEGER NOT NULL,
			PRIMARY KEY (local_batch_id, file_path),
			FOREIGN KEY (local_batch_id) REFERENCES upload_batches(local_batch_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS upload_batch_metadata (
			local_batch_id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL DEFAULT '',
			test_task_id TEXT NOT NULL DEFAULT '',
			test_task_name TEXT NOT NULL DEFAULT '',
			uploader_name TEXT NOT NULL DEFAULT '',
			uploader_email TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			collector_version TEXT NOT NULL DEFAULT '',
			timezone TEXT NOT NULL DEFAULT '',
			source_created_at TEXT,
			source_started_at TEXT,
			source_ended_at TEXT,
			scenario_ids_json TEXT NOT NULL DEFAULT '[]',
			FOREIGN KEY (local_batch_id) REFERENCES upload_batches(local_batch_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_upload_batches_ready ON upload_batches(state, next_attempt_at, created_at)`,
		`CREATE TABLE IF NOT EXISTS upload_device_policy (
			device_sn TEXT PRIMARY KEY,
			paused INTEGER NOT NULL DEFAULT 0 CHECK(paused IN (0,1)),
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_cache (
			cache_key TEXT PRIMARY KEY,
			response_json BLOB NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS collection_sessions (
			session_id TEXT PRIMARY KEY,
			device_sn TEXT NOT NULL,
			port_name TEXT NOT NULL,
			project_id TEXT NOT NULL DEFAULT '',
			project_name TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			test_task_id TEXT NOT NULL DEFAULT '',
			test_task_name TEXT NOT NULL DEFAULT '',
			save_enabled INTEGER NOT NULL DEFAULT 1,
			upload_enabled INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL,
			ended_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_collection_sessions_device ON collection_sessions(device_sn, started_at)`,
		`CREATE TABLE IF NOT EXISTS log_files (
			log_file_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			file_name TEXT NOT NULL,
			device_sn TEXT NOT NULL,
			port_name TEXT NOT NULL,
			project_id TEXT NOT NULL DEFAULT '',
			project_name TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			test_task_id TEXT NOT NULL DEFAULT '',
			test_task_name TEXT NOT NULL DEFAULT '',
			first_sequence INTEGER NOT NULL,
			last_sequence INTEGER NOT NULL,
			line_count INTEGER NOT NULL,
			size_bytes INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			upload_eligible INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			completed_at TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES collection_sessions(session_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_log_files_history ON log_files(device_sn, completed_at)`,
		`CREATE TABLE IF NOT EXISTS keyword_hits (
			hit_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			device_sn TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			rule_name TEXT NOT NULL,
			matched_at TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			line_text TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES collection_sessions(session_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword_hits_session ON keyword_hits(session_id, rule_id, matched_at)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	columns := []struct{ table, name, definition string }{
		{"upload_batches", "query_code", "TEXT"},
		{"upload_batches", "upload_session_id", "TEXT"},
		{"upload_batches", "session_id", "TEXT"},
		{"upload_batches", "bytes_total", "INTEGER NOT NULL DEFAULT 0"},
		{"upload_batches", "started_at", "TEXT"},
		{"upload_batches", "completed_at", "TEXT"},
		{"upload_files", "log_file_id", "TEXT"},
		{"collection_sessions", "uploader_name", "TEXT NOT NULL DEFAULT ''"},
		{"collection_sessions", "uploader_email", "TEXT NOT NULL DEFAULT ''"},
		{"collection_sessions", "remark", "TEXT NOT NULL DEFAULT ''"},
		{"collection_sessions", "collector_version", "TEXT NOT NULL DEFAULT ''"},
		{"collection_sessions", "timezone", "TEXT NOT NULL DEFAULT ''"},
		{"collection_sessions", "scenario_ids_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"upload_batch_metadata", "config_snapshot", "TEXT NOT NULL DEFAULT '{}'"},
		{"upload_batch_metadata", "uploader_email", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		if err := ensureColumn(db, column.table, column.name, column.definition); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate sqlite %s.%s: %w", column.table, column.name, err)
		}
	}
	return &Store{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) NextSequence(ctx context.Context, deviceSN string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := s.now().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO sequences(device_sn,next_value,updated_at) VALUES(?,1,?) ON CONFLICT(device_sn) DO NOTHING`, deviceSN, now); err != nil {
		return 0, err
	}
	var next int64
	if err := tx.QueryRowContext(ctx, `SELECT next_value FROM sequences WHERE device_sn=?`, deviceSN).Scan(&next); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sequences SET next_value=?, updated_at=? WHERE device_sn=?`, next+1, now, deviceSN); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

func (s *Store) EnqueueFile(ctx context.Context, projectName, version string, file File) (string, error) {
	return s.EnqueueFileWithMetadata(ctx, UploadMetadata{ProjectName: projectName, Version: version}, file)
}

func (s *Store) EnqueueFileWithMetadata(ctx context.Context, metadata UploadMetadata, file File) (string, error) {
	if err := VerifyFile(file); err != nil {
		return "", err
	}
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT local_batch_id FROM upload_files WHERE file_path=? LIMIT 1`, file.Path).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	scenarioJSON, err := json.Marshal(metadata.ScenarioIDs)
	if err != nil {
		return "", fmt.Errorf("encode scenario ids: %w", err)
	}
	now := s.now().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batches(local_batch_id,project_name,version,state,next_attempt_at,created_at,session_id,bytes_total,upload_session_id,query_code) VALUES(?,?,?,'pending',?,?,?,?,?,?)`, id, strings.TrimSpace(metadata.ProjectName), strings.TrimSpace(metadata.Version), now, now, file.SessionID, file.SizeBytes, nullable(metadata.UploadSessionID), nullable(metadata.QueryCode)); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batch_metadata(local_batch_id,project_id,test_task_id,test_task_name,uploader_name,uploader_email,remark,collector_version,timezone,source_created_at,source_started_at,source_ended_at,scenario_ids_json,config_snapshot) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, strings.TrimSpace(metadata.ProjectID), strings.TrimSpace(metadata.TestTaskID), strings.TrimSpace(metadata.TestTaskName), strings.TrimSpace(metadata.UploaderName), strings.ToLower(strings.TrimSpace(metadata.UploaderEmail)), strings.TrimSpace(metadata.Remark), strings.TrimSpace(metadata.CollectorVersion), strings.TrimSpace(metadata.Timezone), nullableTime(metadata.CreatedAt), nullableTime(metadata.StartedAt), nullableTime(metadata.EndedAt), string(scenarioJSON), firstNonBlank(metadata.ConfigSnapshot, "{}")); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_files(local_batch_id,file_path,sha256,size_bytes,device_sn,first_sequence,last_sequence,log_file_id) VALUES(?,?,?,?,?,?,?,?)`, id, file.Path, strings.ToLower(file.SHA256), file.SizeBytes, file.DeviceSN, file.FirstSequence, file.LastSequence, nullable(file.LogFileID)); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func VerifyFile(file File) error {
	info, err := os.Stat(file.Path)
	if err != nil {
		return fmt.Errorf("stat spool file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != file.SizeBytes {
		return fmt.Errorf("spool file size mismatch: expected %d, got %d", file.SizeBytes, info.Size())
	}
	f, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, file.SHA256) {
		return fmt.Errorf("spool file sha256 mismatch: expected %s, got %s", file.SHA256, actual)
	}
	return nil
}

func (s *Store) ClaimReady(ctx context.Context, maxFiles int) (*Batch, error) {
	if maxFiles < 1 {
		maxFiles = 1
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := s.now().Format(time.RFC3339Nano)
	var parentID, project, version, lastError, sessionID string
	var parentAttempts int
	err = tx.QueryRowContext(ctx, `SELECT b.local_batch_id,b.project_name,b.version,COALESCE(b.last_error,''),COALESCE(b.session_id,''),b.attempt_count
		FROM upload_batches b
		WHERE b.state='pending' AND b.next_attempt_at<=?
		AND NOT EXISTS (
			SELECT 1 FROM upload_files f JOIN upload_device_policy p ON p.device_sn=f.device_sn AND p.paused=1
			WHERE f.local_batch_id=b.local_batch_id
		)
		ORDER BY b.created_at LIMIT 1`, now).Scan(&parentID, &project, &version, &lastError, &sessionID, &parentAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var deviceSN string
	if err := tx.QueryRowContext(ctx, `SELECT device_sn FROM upload_files WHERE local_batch_id=? LIMIT 1`, parentID).Scan(&deviceSN); err != nil {
		return nil, err
	}
	ids := []string{parentID}
	if parentAttempts == 0 && lastError != "split after HTTP 413" {
		rows, err := tx.QueryContext(ctx, `SELECT b.local_batch_id FROM upload_batches b WHERE b.state='pending' AND b.next_attempt_at<=? AND b.attempt_count=0 AND b.project_name=? AND b.version=? AND COALESCE(b.session_id,'')=? AND COALESCE(b.last_error,'')<>'split after HTTP 413' AND EXISTS (SELECT 1 FROM upload_files f WHERE f.local_batch_id=b.local_batch_id AND f.device_sn=?) AND NOT EXISTS (SELECT 1 FROM upload_files f JOIN upload_device_policy p ON p.device_sn=f.device_sn AND p.paused=1 WHERE f.local_batch_id=b.local_batch_id) ORDER BY b.created_at LIMIT ?`, now, project, version, sessionID, deviceSN, maxFiles)
		if err != nil {
			return nil, err
		}
		ids = ids[:0]
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	for _, id := range ids {
		if id == parentID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE upload_files SET local_batch_id=? WHERE local_batch_id=?`, parentID, id); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batch_metadata WHERE local_batch_id=?`, id); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batches WHERE local_batch_id=?`, id); err != nil {
			return nil, err
		}
	}
	claimed, err := tx.ExecContext(ctx, `UPDATE upload_batches SET state='uploading', attempt_count=attempt_count+1, next_attempt_at=?, started_at=?, bytes_total=(SELECT COALESCE(SUM(size_bytes),0) FROM upload_files WHERE local_batch_id=?) WHERE local_batch_id=? AND state='pending'`, now, now, parentID, parentID)
	if err != nil {
		return nil, err
	}
	if count, err := claimed.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetBatch(ctx, parentID)
}

func (s *Store) SetDeviceUploadPaused(ctx context.Context, deviceSN string, paused bool) error {
	deviceSN = strings.TrimSpace(deviceSN)
	if deviceSN == "" {
		return errors.New("device serial is required")
	}
	value := 0
	if paused {
		value = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO upload_device_policy(device_sn,paused,updated_at) VALUES(?,?,?)
		ON CONFLICT(device_sn) DO UPDATE SET paused=excluded.paused,updated_at=excluded.updated_at`,
		deviceSN, value, s.now().Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetBatch(ctx context.Context, id string) (*Batch, error) {
	var b Batch
	var state, next, created, uploaded, started, completed, sourceCreated, sourceStarted, sourceEnded sql.NullString
	var scenarioJSON string
	err := s.db.QueryRowContext(ctx, `SELECT b.local_batch_id,b.project_name,b.version,COALESCE(m.project_id,''),COALESCE(m.test_task_id,''),COALESCE(m.test_task_name,''),COALESCE(m.uploader_name,''),COALESCE(m.uploader_email,''),COALESCE(m.remark,''),b.local_batch_id,COALESCE(m.collector_version,''),COALESCE(m.timezone,''),m.source_created_at,m.source_started_at,m.source_ended_at,COALESCE(m.scenario_ids_json,'[]'),b.state,b.attempt_count,b.next_attempt_at,COALESCE(b.upload_id,''),COALESCE(b.task_id,''),COALESCE(b.query_code,''),COALESCE(b.upload_session_id,''),CASE WHEN COALESCE(b.session_id,'')='' THEN 1 ELSE (SELECT COUNT(*) FROM upload_batches position WHERE position.session_id=b.session_id AND (position.created_at<b.created_at OR (position.created_at=b.created_at AND position.local_batch_id<=b.local_batch_id))) END,COALESCE(m.config_snapshot,'{}'),COALESCE(b.session_id,''),b.bytes_total,COALESCE(b.last_error,''),b.created_at,b.uploaded_at,b.started_at,b.completed_at FROM upload_batches b LEFT JOIN upload_batch_metadata m ON m.local_batch_id=b.local_batch_id WHERE b.local_batch_id=?`, id).
		Scan(&b.ID, &b.ProjectName, &b.Version, &b.ProjectID, &b.TestTaskID, &b.TestTaskName, &b.UploaderName, &b.UploaderEmail, &b.Remark, &b.ClientRequestID, &b.CollectorVersion, &b.Timezone, &sourceCreated, &sourceStarted, &sourceEnded, &scenarioJSON, &state, &b.AttemptCount, &next, &b.UploadID, &b.TaskID, &b.QueryCode, &b.UploadSessionID, &b.UploadPosition, &b.ConfigSnapshot, &b.SessionID, &b.BytesTotal, &b.LastError, &created, &uploaded, &started, &completed)
	if err != nil {
		return nil, err
	}
	b.State = State(state.String)
	b.NextAttemptAt, _ = time.Parse(time.RFC3339Nano, next.String)
	b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created.String)
	if uploaded.Valid {
		t, _ := time.Parse(time.RFC3339Nano, uploaded.String)
		b.UploadedAt = &t
	}
	if started.Valid {
		t, _ := time.Parse(time.RFC3339Nano, started.String)
		b.StartedAt = &t
	}
	if completed.Valid {
		t, _ := time.Parse(time.RFC3339Nano, completed.String)
		b.CompletedAt = &t
	}
	b.SourceCreatedAt = parseNullableTime(sourceCreated)
	b.SourceStartedAt = parseNullableTime(sourceStarted)
	b.SourceEndedAt = parseNullableTime(sourceEnded)
	if err := json.Unmarshal([]byte(scenarioJSON), &b.ScenarioIDs); err != nil {
		return nil, fmt.Errorf("decode scenario ids for batch %s: %w", id, err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(log_file_id,''),file_path,sha256,size_bytes,device_sn,first_sequence,last_sequence FROM upload_files WHERE local_batch_id=? ORDER BY file_path`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.LogFileID, &f.Path, &f.SHA256, &f.SizeBytes, &f.DeviceSN, &f.FirstSequence, &f.LastSequence); err != nil {
			return nil, err
		}
		b.Files = append(b.Files, f)
	}
	return &b, rows.Err()
}

func (s *Store) MarkUploaded(ctx context.Context, id, uploadID, taskID string) error {
	now := s.now().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uploaded',upload_id=?,task_id=?,last_error=NULL,uploaded_at=?,completed_at=? WHERE local_batch_id=? AND state='uploading'`, uploadID, taskID, now, now, id)
	return requireUpdated(result, err)
}

func (s *Store) SetQueryCode(ctx context.Context, id, queryCode string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET query_code=? WHERE local_batch_id=?`, nullable(queryCode), id)
	return err
}

func (s *Store) BindPendingUploads(ctx context.Context, deviceSN, uploadSessionID, queryCode, uploaderName, uploaderEmail, configSnapshot string) error {
	if strings.TrimSpace(deviceSN) == "" || strings.TrimSpace(uploadSessionID) == "" || strings.TrimSpace(queryCode) == "" {
		return errors.New("device, upload session, and query code are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE upload_batches SET upload_session_id=?,query_code=?
		WHERE state='pending' AND COALESCE(upload_session_id,'')='' AND EXISTS
		(SELECT 1 FROM upload_files f WHERE f.local_batch_id=upload_batches.local_batch_id AND f.device_sn=?)`, uploadSessionID, queryCode, deviceSN); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE upload_batch_metadata SET uploader_name=?,uploader_email=?,config_snapshot=?
		WHERE local_batch_id IN (SELECT b.local_batch_id FROM upload_batches b JOIN upload_files f ON f.local_batch_id=b.local_batch_id
		WHERE b.state='pending' AND b.upload_session_id=? AND f.device_sn=?)`, strings.TrimSpace(uploaderName), strings.ToLower(strings.TrimSpace(uploaderEmail)), firstNonBlank(configSnapshot, "{}"), uploadSessionID, deviceSN); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkPending(ctx context.Context, id, message string, retryAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='pending',last_error=?,next_attempt_at=? WHERE local_batch_id=? AND state='uploading'`, message, retryAt.UTC().Format(time.RFC3339Nano), id)
	return requireUpdated(result, err)
}

func (s *Store) MarkUncertain(ctx context.Context, id, message string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uncertain',last_error=? WHERE local_batch_id=? AND state='uploading'`, message, id)
	return requireUpdated(result, err)
}

func (s *Store) MarkDead(ctx context.Context, id, message string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='dead',last_error=? WHERE local_batch_id=? AND state IN ('pending','uploading')`, message, id)
	return requireUpdated(result, err)
}

func (s *Store) SplitUploading(ctx context.Context, id string) error {
	batch, err := s.GetBatch(ctx, id)
	if err != nil {
		return err
	}
	if batch.State != Uploading {
		return errors.New("only an uploading batch can be split")
	}
	if len(batch.Files) < 2 {
		return errors.New("batch has only one file")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now().Format(time.RFC3339Nano)
	for _, file := range batch.Files {
		childID, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batches(local_batch_id,project_name,version,state,next_attempt_at,created_at,last_error,session_id,bytes_total,upload_session_id,query_code) VALUES(?,?,?,'pending',?,?, 'split after HTTP 413',?,?,?,?)`, childID, batch.ProjectName, batch.Version, now, now, nullable(batch.SessionID), file.SizeBytes, nullable(batch.UploadSessionID), nullable(batch.QueryCode)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batch_metadata(local_batch_id,project_id,test_task_id,test_task_name,uploader_name,uploader_email,remark,collector_version,timezone,source_created_at,source_started_at,source_ended_at,scenario_ids_json,config_snapshot) SELECT ?,project_id,test_task_id,test_task_name,uploader_name,uploader_email,remark,collector_version,timezone,source_created_at,source_started_at,source_ended_at,scenario_ids_json,config_snapshot FROM upload_batch_metadata WHERE local_batch_id=?`, childID, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE upload_files SET local_batch_id=? WHERE local_batch_id=? AND file_path=?`, childID, id, file.Path); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batch_metadata WHERE local_batch_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batches WHERE local_batch_id=? AND state='uploading'`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Recover(ctx context.Context, staleAfter time.Duration) (int64, error) {
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}
	cutoff := s.now().Add(-staleAfter).Format(time.RFC3339Nano)
	// 只有“本次重启前就已滞留超过 staleAfter 的上传中批次”才需要人工核对；
	// 更近的批次回退为 pending 自动重试，避免每次重启都制造 uncertain。
	stale, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uncertain',last_error='agent restarted during upload' WHERE state='uploading' AND next_attempt_at<=?`, cutoff)
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='pending',next_attempt_at=?,last_error='agent restarted, retrying recent upload' WHERE state='uploading'`, s.now().Format(time.RFC3339Nano)); err != nil {
		return 0, err
	}
	return stale.RowsAffected()
}

func (s *Store) Counts(ctx context.Context) (map[State]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT state,COUNT(*) FROM upload_batches GROUP BY state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[State]int64{}
	for rows.Next() {
		var state State
		var count int64
		if err := rows.Scan(&state, &count); err != nil {
			return nil, err
		}
		counts[state] = count
	}
	return counts, rows.Err()
}

func (s *Store) ListByState(ctx context.Context, state State) ([]Batch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT local_batch_id FROM upload_batches WHERE state=? ORDER BY created_at`, state)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	var batches []Batch
	for _, id := range ids {
		batch, err := s.GetBatch(ctx, id)
		if err != nil {
			return nil, err
		}
		batches = append(batches, *batch)
	}
	return batches, nil
}

func (s *Store) RetryUncertain(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='pending',next_attempt_at=?,last_error='operator requested retry' WHERE local_batch_id=? AND state='uncertain'`, s.now().Format(time.RFC3339Nano), id)
	return requireUpdated(result, err)
}

func (s *Store) RetryDead(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='pending',next_attempt_at=?,last_error='operator requested retry' WHERE local_batch_id=? AND state='dead'`, s.now().Format(time.RFC3339Nano), id)
	return requireUpdated(result, err)
}

func (s *Store) ConfirmUncertain(ctx context.Context, id, uploadID, taskID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uploaded',upload_id=?,task_id=?,last_error=NULL,uploaded_at=? WHERE local_batch_id=? AND state='uncertain'`, uploadID, taskID, s.now().Format(time.RFC3339Nano), id)
	return requireUpdated(result, err)
}

func (s *Store) DeleteExpiredUploaded(ctx context.Context, before time.Time) (int, error) {
	batches, err := s.ListByState(ctx, Uploaded)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, batch := range batches {
		if batch.UploadedAt == nil || !batch.UploadedAt.Before(before) {
			continue
		}
		allRemoved := true
		for _, file := range batch.Files {
			if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				allRemoved = false
			}
		}
		if allRemoved {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM upload_batches WHERE local_batch_id=? AND state='uploaded'`, batch.ID); err != nil {
				return deleted, err
			}
			deleted++
		}
	}
	return deleted, nil
}

// ClearUploadHistory removes only local upload queue records. The original log
// files and their local history entries remain available to the user.
func (s *Store) ClearUploadHistory(ctx context.Context) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_batches`).Scan(&count); err != nil {
		return 0, err
	}
	var uploading int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_batches WHERE state='uploading'`).Scan(&uploading); err != nil {
		return 0, err
	}
	if uploading > 0 {
		return 0, errors.New("正在上传，不能清空上传历史记录")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batch_metadata`); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_files`); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batches`); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	var raw []byte
	var expires string
	err := s.db.QueryRowContext(ctx, `SELECT response_json,expires_at FROM analysis_cache WHERE cache_key=?`, key).Scan(&raw, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || !expiresAt.After(s.now()) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM analysis_cache WHERE cache_key=?`, key)
		return nil, false, nil
	}
	return raw, true, nil
}

func (s *Store) Put(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !json.Valid(value) {
		return errors.New("analysis cache value must be valid JSON")
	}
	now := s.now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO analysis_cache(cache_key,response_json,expires_at,created_at) VALUES(?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET response_json=excluded.response_json,expires_at=excluded.expires_at,created_at=excluded.created_at`, key, value, now.Add(ttl).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return err
}

func requireUpdated(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("batch state transition rejected")
	}
	return nil
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func firstNonBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.Format(time.RFC3339Nano)
}

func parseNullableTime(value sql.NullString) *time.Time {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}
