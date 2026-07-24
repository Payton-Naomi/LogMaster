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
	Path          string
	SHA256        string
	SizeBytes     int64
	DeviceSN      string
	FirstSequence int64
	LastSequence  int64
}

type Batch struct {
	ID            string
	ProjectName   string
	Version       string
	State         State
	AttemptCount  int
	NextAttemptAt time.Time
	UploadID      string
	TaskID        string
	LastError     string
	CreatedAt     time.Time
	UploadedAt    *time.Time
	Files         []File
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
		`CREATE INDEX IF NOT EXISTS idx_upload_batches_ready ON upload_batches(state, next_attempt_at, created_at)`,
		`CREATE TABLE IF NOT EXISTS analysis_cache (
			cache_key TEXT PRIMARY KEY,
			response_json BLOB NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	return &Store{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
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
	now := s.now().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batches(local_batch_id,project_name,version,state,next_attempt_at,created_at) VALUES(?,?,?,'pending',?,?)`, id, projectName, version, now, now); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_files(local_batch_id,file_path,sha256,size_bytes,device_sn,first_sequence,last_sequence) VALUES(?,?,?,?,?,?,?)`, id, file.Path, strings.ToLower(file.SHA256), file.SizeBytes, file.DeviceSN, file.FirstSequence, file.LastSequence); err != nil {
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
	var parentID, project, version, lastError string
	err = tx.QueryRowContext(ctx, `SELECT local_batch_id,project_name,version,COALESCE(last_error,'') FROM upload_batches WHERE state='pending' AND next_attempt_at<=? ORDER BY created_at LIMIT 1`, now).Scan(&parentID, &project, &version, &lastError)
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
	if lastError != "split after HTTP 413" {
		rows, err := tx.QueryContext(ctx, `SELECT b.local_batch_id FROM upload_batches b WHERE b.state='pending' AND b.next_attempt_at<=? AND b.project_name=? AND b.version=? AND COALESCE(b.last_error,'')<>'split after HTTP 413' AND EXISTS (SELECT 1 FROM upload_files f WHERE f.local_batch_id=b.local_batch_id AND f.device_sn=?) ORDER BY b.created_at LIMIT ?`, now, project, version, deviceSN, maxFiles)
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
		if _, err := tx.ExecContext(ctx, `DELETE FROM upload_batches WHERE local_batch_id=?`, id); err != nil {
			return nil, err
		}
	}
	claimed, err := tx.ExecContext(ctx, `UPDATE upload_batches SET state='uploading', attempt_count=attempt_count+1, next_attempt_at=? WHERE local_batch_id=? AND state='pending'`, now, parentID)
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

func (s *Store) GetBatch(ctx context.Context, id string) (*Batch, error) {
	var b Batch
	var state, next, created, uploaded sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT local_batch_id,project_name,version,state,attempt_count,next_attempt_at,COALESCE(upload_id,''),COALESCE(task_id,''),COALESCE(last_error,''),created_at,uploaded_at FROM upload_batches WHERE local_batch_id=?`, id).
		Scan(&b.ID, &b.ProjectName, &b.Version, &state, &b.AttemptCount, &next, &b.UploadID, &b.TaskID, &b.LastError, &created, &uploaded)
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
	rows, err := s.db.QueryContext(ctx, `SELECT file_path,sha256,size_bytes,device_sn,first_sequence,last_sequence FROM upload_files WHERE local_batch_id=? ORDER BY file_path`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.Path, &f.SHA256, &f.SizeBytes, &f.DeviceSN, &f.FirstSequence, &f.LastSequence); err != nil {
			return nil, err
		}
		b.Files = append(b.Files, f)
	}
	return &b, rows.Err()
}

func (s *Store) MarkUploaded(ctx context.Context, id, uploadID, taskID string) error {
	now := s.now().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uploaded',upload_id=?,task_id=?,last_error=NULL,uploaded_at=? WHERE local_batch_id=? AND state='uploading'`, uploadID, taskID, now, id)
	return requireUpdated(result, err)
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
	for _, file := range batch.Files[1:] {
		childID, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO upload_batches(local_batch_id,project_name,version,state,next_attempt_at,created_at,last_error) VALUES(?,?,?,'pending',?,?, 'split after HTTP 413')`, childID, batch.ProjectName, batch.Version, now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE upload_files SET local_batch_id=? WHERE local_batch_id=? AND file_path=?`, childID, id, file.Path); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE upload_batches SET state='pending',next_attempt_at=?,last_error='split after HTTP 413' WHERE local_batch_id=? AND state='uploading'`, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Recover(ctx context.Context, staleAfter time.Duration) (int64, error) {
	_ = staleAfter
	result, err := s.db.ExecContext(ctx, `UPDATE upload_batches SET state='uncertain',last_error='agent restarted during upload' WHERE state='uploading'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
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
