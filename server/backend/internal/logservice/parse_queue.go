package logservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const parseTaskLeaseDuration = 90 * time.Second

var (
	ErrParseTaskLeaseLost      = errors.New("parse task lease lost")
	ErrTaskNotRetryable        = errors.New("task can only be retried after failure")
	ErrAgentRetryNotReady      = errors.New("AI retry requires a completed parse task")
	ErrAgentRetryQueued        = errors.New("AI retry is already queued")
	ErrAgentNotCancellable     = errors.New("AI analysis cannot be cancelled in its current state")
	ErrTaskNotCancellable      = errors.New("task cannot be cancelled in its current state")
	ErrTaskNotPausable         = errors.New("task cannot be paused in its current state")
	ErrTaskNotResumable        = errors.New("task cannot be resumed in its current state")
	ErrTaskPriorityNotEditable = errors.New("task priority cannot be changed in its current state")
)

type ClaimedParseTask struct {
	TaskID       string
	UploadID     string
	Phase        string
	RunToken     string
	AttemptNo    int
	OriginalName string
	OriginalSize int64
	StoragePath  string
}

func (r *Repository) ReconcileParseQueue(ctx context.Context, maxAttempts int) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	const message = "parse worker lease expired and maximum recovery attempts were reached"
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts attempt
		SET status = 'interrupted', error_message = $2, completed_at = NOW(), heartbeat_at = NOW()
		FROM logmaster_api.parse_tasks task
		WHERE attempt.task_id = task.id AND attempt.run_token = task.run_token
		AND attempt.status = 'running' AND task.status = 'running'
		AND (task.lease_expires_at IS NULL OR task.lease_expires_at < NOW())
		AND task.attempt_no >= $1`, maxAttempts, message); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads upload
		SET status = 'failed', error_message = $2, updated_at = NOW()
		FROM logmaster_api.parse_tasks task
		WHERE task.upload_id = upload.id AND task.status = 'running'
		AND (task.lease_expires_at IS NULL OR task.lease_expires_at < NOW())
		AND task.attempt_no >= $1`, maxAttempts, message); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'failed', error_message = $2, completed_at = NOW(), worker_id = '',
			run_token = NULL, lease_expires_at = NULL, updated_at = NOW()
		WHERE status = 'running' AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
		AND attempt_no >= $1`, maxAttempts, message); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ClaimParseTask(ctx context.Context, workerID string, maxAttempts, maxPerUser, maxPerProject int) (ClaimedParseTask, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if maxPerUser < 1 {
		maxPerUser = 1
	}
	if maxPerProject < 1 {
		maxPerProject = 1
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimedParseTask{}, err
	}
	defer tx.Rollback()

	var task ClaimedParseTask
	var previousStatus string
	var previousToken sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT task.id, task.upload_id, task.phase, task.attempt_no,
		upload.original_name, upload.original_size, upload.storage_path, task.status, task.run_token::text
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		JOIN logmaster_api.projects project ON project.id = upload.project_id
		WHERE (task.attempt_no < $1 OR task.manual_retry_requested) AND (
			(task.status = 'queued' AND upload.status = 'queued') OR
			(task.status = 'running' AND (task.lease_expires_at IS NULL OR task.lease_expires_at < NOW()))
		)
		AND (SELECT COUNT(*) FROM logmaster_api.parse_tasks active
			JOIN logmaster_api.log_uploads active_upload ON active_upload.id=active.upload_id
			WHERE active.status='running' AND active.id<>task.id AND active.lease_expires_at>NOW()
			AND COALESCE(NULLIF(active_upload.uploader_id,''),active_upload.created_by_open_id)=COALESCE(NULLIF(upload.uploader_id,''),upload.created_by_open_id)) < $2
		AND (SELECT COUNT(*) FROM logmaster_api.parse_tasks active
			JOIN logmaster_api.log_uploads active_upload ON active_upload.id=active.upload_id
			WHERE active.status='running' AND active.id<>task.id AND active.lease_expires_at>NOW()
			AND active_upload.project_id=upload.project_id) < $3
		ORDER BY (project.scheduling_priority + task.priority) DESC, task.created_at, task.id
		LIMIT 1 FOR UPDATE OF task SKIP LOCKED`, maxAttempts, maxPerUser, maxPerProject).Scan(
		&task.TaskID, &task.UploadID, &task.Phase, &task.AttemptNo,
		&task.OriginalName, &task.OriginalSize, &task.StoragePath, &previousStatus, &previousToken)
	if err != nil {
		return ClaimedParseTask{}, err
	}

	if previousStatus == "running" && previousToken.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts
			SET status = 'interrupted', error_message = 'parse worker lease expired', completed_at = NOW(), heartbeat_at = NOW()
			WHERE task_id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, previousToken.String); err != nil {
			return ClaimedParseTask{}, err
		}
	}

	task.AttemptNo++
	task.RunToken = newID()
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'running', attempt_no = $2, worker_id = $3, run_token = $4::uuid,
			manual_retry_requested = FALSE,
			lease_expires_at = NOW() + ($5 * INTERVAL '1 second'), heartbeat_at = NOW(),
			started_at = COALESCE(started_at, NOW()), completed_at = NULL, error_message = '', updated_at = NOW()
		WHERE id = $1`, task.TaskID, task.AttemptNo, workerID, task.RunToken, int64(parseTaskLeaseDuration/time.Second)); err != nil {
		return ClaimedParseTask{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.parse_task_attempts
		(task_id, attempt_no, run_token, worker_id, phase, status)
		VALUES ($1, $2, $3::uuid, $4, $5, 'running')`, task.TaskID, task.AttemptNo, task.RunToken, workerID, task.Phase); err != nil {
		return ClaimedParseTask{}, err
	}
	if task.Phase == "parse" {
		if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
			SET status = 'parsing', error_message = '', updated_at = NOW() WHERE id = $1`, task.UploadID); err != nil {
			return ClaimedParseTask{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return ClaimedParseTask{}, err
	}
	return task, nil
}

func (r *Repository) RenewParseTaskLease(ctx context.Context, taskID, runToken string) error {
	result, err := r.db.ExecContext(ctx, `WITH renewed AS (
		UPDATE logmaster_api.parse_tasks
		SET lease_expires_at = NOW() + ($3 * INTERVAL '1 second'), heartbeat_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'
		RETURNING id
	)
	UPDATE logmaster_api.parse_task_attempts attempt
	SET heartbeat_at = NOW()
	FROM renewed WHERE attempt.task_id = renewed.id AND attempt.run_token = $2::uuid AND attempt.status = 'running'`,
		taskID, runToken, int64(parseTaskLeaseDuration/time.Second))
	if err != nil {
		return err
	}
	return requireAffectedRow(result)
}

func (r *Repository) QueuePreparedUpload(ctx context.Context, task ClaimedParseTask, files []LogFile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockClaimedParseTask(ctx, tx, task.TaskID, task.RunToken); err != nil {
		return err
	}
	for i := range files {
		err = tx.QueryRowContext(ctx, `INSERT INTO logmaster_api.log_files (upload_id, relative_path, size_bytes, sha256)
			VALUES ($1, $2, $3, $4) RETURNING id`, task.UploadID, files[i].RelativePath, files[i].SizeBytes, files[i].SHA256).Scan(&files[i].ID)
		if err != nil {
			return fmt.Errorf("create log file: %w", err)
		}
	}
	var totalBytes int64
	for _, file := range files {
		totalBytes += file.SizeBytes
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET phase = 'parse', total_files = $3, total_bytes = $4, updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`,
		task.TaskID, task.RunToken, len(files), totalBytes); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts
		SET phase = 'parse', heartbeat_at = NOW()
		WHERE task_id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'parsing', original_name = $2, original_size = $3, error_message = '', updated_at = NOW()
		WHERE id = $1`, task.UploadID, task.OriginalName, task.OriginalSize); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) BeginClaimedParsing(ctx context.Context, task ClaimedParseTask) (string, []LogFile, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback()
	if err := lockClaimedParseTask(ctx, tx, task.TaskID, task.RunToken); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.task_ai_overviews WHERE task_id = $1`, task.TaskID); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.agent_analyses WHERE task_id = $1`, task.TaskID); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.parse_results WHERE task_id = $1`, task.TaskID); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_files SET line_count = 0 WHERE upload_id = $1`, task.UploadID); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET phase = 'parse', processed_files = 0, processed_bytes = 0, total_lines = 0,
			error_count = 0, warning_count = 0, error_message = '', updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken); err != nil {
		return "", nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'parsing', error_message = '', updated_at = NOW() WHERE id = $1`, task.UploadID); err != nil {
		return "", nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, relative_path, size_bytes, sha256, line_count
		FROM logmaster_api.log_files WHERE upload_id = $1 ORDER BY id`, task.UploadID)
	if err != nil {
		return "", nil, err
	}
	files := make([]LogFile, 0)
	for rows.Next() {
		var file LogFile
		if err := rows.Scan(&file.ID, &file.RelativePath, &file.SizeBytes, &file.SHA256, &file.LineCount); err != nil {
			rows.Close()
			return "", nil, err
		}
		files = append(files, file)
	}
	if err := rows.Close(); err != nil {
		return "", nil, err
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return task.TaskID, files, nil
}

func (r *Repository) UpdateClaimedParsingProgress(ctx context.Context, taskID, runToken string, processedBytes int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET processed_bytes = LEAST(total_bytes, GREATEST(processed_bytes, $3)), updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`, taskID, runToken, processedBytes)
	if err != nil {
		return err
	}
	return requireAffectedRow(result)
}

func (r *Repository) SaveClaimedFileResults(ctx context.Context, task ClaimedParseTask, fileID, lineCount, errorCount, warningCount int64, results []ParseResult) error {
	encodedResults, err := encodeParseResults(results)
	if err != nil {
		return fmt.Errorf("encode parse results: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockClaimedParseTask(ctx, tx, task.TaskID, task.RunToken); err != nil {
		return err
	}
	if err := insertParseResults(ctx, tx, task.TaskID, fileID, encodedResults); err != nil {
		return fmt.Errorf("save parse results: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_files SET line_count = $2 WHERE id = $1`, fileID, lineCount); err != nil {
		return fmt.Errorf("update log file: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET processed_files = processed_files + 1, total_lines = total_lines + $3,
			error_count = error_count + $4, warning_count = warning_count + $5, updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`,
		task.TaskID, task.RunToken, lineCount, errorCount, warningCount)
	if err != nil {
		return fmt.Errorf("update parse task: %w", err)
	}
	if err := requireAffectedRow(result); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CompleteClaimedParsing(ctx context.Context, task ClaimedParseTask) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'completed', completed_at = NOW(), worker_id = '', run_token = NULL,
			lease_expires_at = NULL, error_message = '', updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken)
	if err != nil {
		return err
	}
	if err := requireAffectedRow(result); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts
		SET status = 'completed', completed_at = NOW(), heartbeat_at = NOW()
		WHERE task_id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'completed', error_message = '', updated_at = NOW() WHERE id = $1`, task.UploadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) FailClaimedParseTask(ctx context.Context, task ClaimedParseTask, message string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'failed', error_message = $3, completed_at = NOW(), worker_id = '', run_token = NULL,
			lease_expires_at = NULL, ai_status=CASE WHEN ai_status='disabled' THEN 'disabled' ELSE 'cancelled' END,
			ai_error_message=CASE WHEN ai_status='disabled' THEN '' ELSE '规则解析失败，AI 作业已停止' END, updated_at = NOW()
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken, message)
	if err != nil {
		return err
	}
	if err := requireAffectedRow(result); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts
		SET status = 'failed', error_message = $3, completed_at = NOW(), heartbeat_at = NOW()
		WHERE task_id = $1 AND run_token = $2::uuid AND status = 'running'`, task.TaskID, task.RunToken, message); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'failed', error_message = $2, updated_at = NOW() WHERE id = $1`, task.UploadID, message); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='cancelled',completed_at=NOW(),
		worker_id='',run_token=NULL,lease_expires_at=NULL,error_code='cancelled',error_message='规则解析失败，AI 作业已停止',updated_at=NOW()
		WHERE task_id=$1 AND status IN ('queued','running')`, task.TaskID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockClaimedParseTask(ctx context.Context, tx *sql.Tx, taskID, runToken string) error {
	var exists bool
	err := tx.QueryRowContext(ctx, `SELECT TRUE FROM logmaster_api.parse_tasks
		WHERE id = $1 AND run_token = $2::uuid AND status = 'running' FOR UPDATE`, taskID, runToken).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrParseTaskLeaseLost
	}
	return err
}

func requireAffectedRow(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrParseTaskLeaseLost
	}
	return nil
}
