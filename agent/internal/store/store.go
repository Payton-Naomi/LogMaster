package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"logmaster-agent/agent/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_sn TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			captured_at TEXT NOT NULL,
			content TEXT NOT NULL,
			batch_id TEXT,
			created_at TEXT NOT NULL,
			UNIQUE(device_sn, sequence)
		)`,
		`CREATE TABLE IF NOT EXISTS device_state (
			device_sn TEXT PRIMARY KEY,
			last_sequence INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(device_sn, batch_id, id)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Append(ctx context.Context, entry model.LogEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO device_state(device_sn, last_sequence) VALUES(?, ?) ON CONFLICT(device_sn) DO UPDATE SET last_sequence=MAX(last_sequence, excluded.last_sequence)`, entry.DeviceSN, entry.Sequence); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO outbox(device_sn, sequence, captured_at, content, created_at) VALUES(?,?,?,?,?)`,
		entry.DeviceSN, entry.Sequence, entry.CapturedAt.UTC().Format(time.RFC3339Nano), entry.Content, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MaxSequence(ctx context.Context, deviceSN string) (int64, error) {
	var sequence int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT last_sequence FROM device_state WHERE device_sn=?), 0)`, deviceSN).Scan(&sequence)
	return sequence, err
}

func (s *Store) PendingCount(ctx context.Context, deviceSN string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox WHERE device_sn=?`, deviceSN).Scan(&count)
	return count, err
}

func (s *Store) ClaimBatch(ctx context.Context, deviceSN string, limit int) (*model.UploadBatch, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var batchID string
	err = tx.QueryRowContext(ctx, `SELECT batch_id FROM outbox WHERE device_sn=? AND batch_id IS NOT NULL ORDER BY id LIMIT 1`, deviceSN).Scan(&batchID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == sql.ErrNoRows {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM outbox WHERE device_sn=? AND batch_id IS NULL ORDER BY id LIMIT ?`, deviceSN, limit)
		if err != nil {
			return nil, err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		batchID, err = newBatchID()
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE outbox SET batch_id=? WHERE id=? AND batch_id IS NULL`, batchID, id); err != nil {
				return nil, err
			}
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT sequence, captured_at, content FROM outbox WHERE device_sn=? AND batch_id=? ORDER BY id`, deviceSN, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	batch := &model.UploadBatch{BatchID: batchID, DeviceSN: deviceSN}
	for rows.Next() {
		var entry model.LogEntry
		var captured string
		if err := rows.Scan(&entry.Sequence, &captured, &entry.Content); err != nil {
			return nil, err
		}
		entry.DeviceSN = deviceSN
		entry.CapturedAt, err = time.Parse(time.RFC3339Nano, captured)
		if err != nil {
			return nil, fmt.Errorf("parse stored timestamp: %w", err)
		}
		batch.Logs = append(batch.Logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return batch, nil
}

func (s *Store) Acknowledge(ctx context.Context, batchID string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM outbox WHERE batch_id=?`, batchID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func newBatchID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
