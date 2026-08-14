package logs

import "context"

func (r *Repository) RecordRuntimeLog(ctx context.Context, ownerOpenID, module, event, status, message, taskID, queryCode string) {
	_, _ = r.db.ExecContext(ctx, `INSERT INTO logmaster_api.runtime_logs
		(owner_open_id, module, event, status, message, task_id, query_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, ownerOpenID, module, event, status, message, taskID, queryCode)
}

func (r *Repository) UploadRuntimeContext(ctx context.Context, uploadID string) (ownerOpenID, taskID, queryCode string) {
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(u.created_by_open_id, ''), COALESCE(t.id::text, ''), COALESCE(u.query_code, '')
		FROM logmaster_api.log_uploads u
		LEFT JOIN logmaster_api.parse_tasks t ON t.upload_id = u.id
		WHERE u.id = $1`, uploadID).Scan(&ownerOpenID, &taskID, &queryCode)
	return
}

func (r *Repository) RecordUploadRuntimeLog(ctx context.Context, uploadID, module, event, status, message string) {
	ownerOpenID, taskID, queryCode := r.UploadRuntimeContext(ctx, uploadID)
	r.RecordRuntimeLog(ctx, ownerOpenID, module, event, status, message, taskID, queryCode)
}
