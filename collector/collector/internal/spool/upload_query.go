package spool

import (
	"context"
	"strings"
)

type BatchFilter struct {
	DeviceSN        string
	States          []State
	Search          string
	IncludeUploaded bool
	Offset          int
	Limit           int
}

func (s *Store) ListBatches(ctx context.Context, filter BatchFilter) ([]Batch, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	if !filter.IncludeUploaded && len(filter.States) == 0 {
		where = append(where, "b.state<>'uploaded'")
	}
	if filter.DeviceSN != "" {
		where = append(where, "EXISTS (SELECT 1 FROM upload_files f WHERE f.local_batch_id=b.local_batch_id AND f.device_sn=?)")
		args = append(args, filter.DeviceSN)
	}
	if len(filter.States) > 0 {
		placeholders := make([]string, len(filter.States))
		for i, state := range filter.States {
			placeholders[i] = "?"
			args = append(args, state)
		}
		where = append(where, "b.state IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.Search != "" {
		term := "%" + filter.Search + "%"
		where = append(where, "(COALESCE(b.query_code,'') LIKE ? OR COALESCE(b.task_id,'') LIKE ? OR EXISTS (SELECT 1 FROM upload_files f WHERE f.local_batch_id=b.local_batch_id AND (f.file_path LIKE ? OR f.sha256 LIKE ?)))")
		args = append(args, term, term, term, term)
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_batches b WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	queryArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx, `SELECT b.local_batch_id FROM upload_batches b WHERE `+clause+` ORDER BY b.created_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	result := make([]Batch, 0, len(ids))
	for _, id := range ids {
		batch, err := s.GetBatch(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *batch)
	}
	return result, total, nil
}
