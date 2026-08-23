package logservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	agentJobLeaseDuration = 90 * time.Second
	maxAgentJobAttempts   = 3
)

func (r *Repository) EnqueueAgentJob(ctx context.Context, job agentJob) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taskExists bool
	if err = tx.QueryRowContext(ctx, `SELECT TRUE FROM logmaster_api.parse_tasks
		WHERE id=$1 AND attempt_no=$2 AND ai_cancel_requested=FALSE FOR UPDATE`, job.taskID, job.attemptNo).Scan(&taskExists); err != nil {
		return err
	}
	jobType := "file"
	var fileID any = job.file.ID
	if job.overview {
		jobType = "overview"
		fileID = nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO logmaster_api.ai_jobs
		(task_id, attempt_no, job_type, log_file_id, owner_open_id, status)
		SELECT $1, $2, $3, $4, $5, 'queued'
		FROM logmaster_api.parse_tasks task
		WHERE task.id=$1 AND task.attempt_no=$2 AND task.ai_cancel_requested=FALSE
		ON CONFLICT (task_id, attempt_no, job_type, (COALESCE(log_file_id, 0))) DO UPDATE SET
		owner_open_id=EXCLUDED.owner_open_id,
		status=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN 'queued' ELSE ai_jobs.status END,
		attempt_count=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN 0 ELSE ai_jobs.attempt_count END,
		worker_id=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN '' ELSE ai_jobs.worker_id END,
		run_token=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN NULL ELSE ai_jobs.run_token END,
		lease_expires_at=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN NULL ELSE ai_jobs.lease_expires_at END,
		heartbeat_at=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN NULL ELSE ai_jobs.heartbeat_at END,
		error_code=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN '' ELSE ai_jobs.error_code END,
		error_message=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN '' ELSE ai_jobs.error_message END,
		started_at=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN NULL ELSE ai_jobs.started_at END,
		completed_at=CASE WHEN ai_jobs.status IN ('completed','failed','cancelled') THEN NULL ELSE ai_jobs.completed_at END,
		updated_at=NOW()`, job.taskID, job.attemptNo, jobType, fileID, job.ownerOpenID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return ErrParseTaskLeaseLost
	}
	if err := recomputeAIStatusTx(ctx, tx, job.taskID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ReconcileAIQueue(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs job
		SET status='cancelled', completed_at=NOW(), lease_expires_at=NULL, run_token=NULL,
			worker_id='', error_code='cancelled', error_message='AI 分析已取消', updated_at=NOW()
		FROM logmaster_api.parse_tasks task
		WHERE task.id=job.task_id AND job.status IN ('queued','running')
		AND (task.attempt_no<>job.attempt_no OR task.ai_cancel_requested)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs
		SET status='queued', worker_id='', run_token=NULL, lease_expires_at=NULL,
			heartbeat_at=NULL, error_code='', error_message='', updated_at=NOW()
		WHERE status='running' AND lease_expires_at<NOW() AND attempt_count<$1`, maxAgentJobAttempts); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs
		SET status='failed', completed_at=NOW(), lease_expires_at=NULL, run_token=NULL,
			worker_id='', error_code='internal_error', error_message='AI Worker 多次中断，已停止自动恢复', updated_at=NOW()
		WHERE status='running' AND lease_expires_at<NOW() AND attempt_count>=$1`, maxAgentJobAttempts); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `WITH states AS (
		SELECT task.id,
			CASE
				WHEN task.ai_cancel_requested THEN 'cancelled'
				WHEN BOOL_OR(job.status='running') THEN 'running'
				WHEN BOOL_OR(job.status='queued') THEN 'queued'
				WHEN BOOL_OR(job.status='completed') AND (BOOL_OR(job.status='failed') OR BOOL_OR(job.status='cancelled')) THEN 'partial_failed'
				WHEN BOOL_OR(job.status='failed') THEN 'failed'
				WHEN BOOL_OR(job.status='completed') THEN 'completed'
				WHEN BOOL_OR(job.status='cancelled') THEN 'cancelled'
				ELSE 'disabled' END AS ai_status,
			COALESCE((ARRAY_AGG(job.error_message ORDER BY job.updated_at DESC)
				FILTER (WHERE job.status='failed'))[1],'') AS ai_error_message
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.ai_jobs job ON job.task_id=task.id AND job.attempt_no=task.attempt_no
		GROUP BY task.id,task.ai_cancel_requested
	)
	UPDATE logmaster_api.parse_tasks task SET ai_status=states.ai_status,
		ai_error_message=states.ai_error_message,updated_at=NOW()
	FROM states WHERE task.id=states.id
	AND (task.ai_status IS DISTINCT FROM states.ai_status OR task.ai_error_message IS DISTINCT FROM states.ai_error_message)`); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ClaimAgentJob(ctx context.Context, workerID string) (agentJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return agentJob{}, err
	}
	defer tx.Rollback()
	var job agentJob
	var jobType string
	err = tx.QueryRowContext(ctx, `SELECT job.id, job.task_id, task.upload_id, job.owner_open_id,
		job.attempt_no, job.job_type, COALESCE(file.id,0), COALESCE(file.relative_path,''),
		COALESCE(file.size_bytes,0), COALESCE(file.sha256,''), COALESCE(file.line_count,0)
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.ai_jobs job ON job.task_id=task.id AND task.attempt_no=job.attempt_no
		JOIN logmaster_api.log_uploads upload ON upload.id=task.upload_id
		JOIN logmaster_api.projects project ON project.id=upload.project_id
		LEFT JOIN logmaster_api.log_files file ON file.id=job.log_file_id
		WHERE job.status='queued' AND task.ai_cancel_requested=FALSE
		AND (job.job_type='file' OR NOT EXISTS (
			SELECT 1 FROM logmaster_api.ai_jobs file_job
			WHERE file_job.task_id=job.task_id AND file_job.attempt_no=job.attempt_no
			AND file_job.job_type='file' AND file_job.status IN ('queued','running')
		))
		ORDER BY (project.scheduling_priority+task.priority) DESC, job.created_at, job.id
		LIMIT 1 FOR UPDATE OF task,job SKIP LOCKED`).Scan(
		&job.queueID, &job.taskID, &job.uploadID, &job.ownerOpenID, &job.attemptNo, &jobType,
		&job.file.ID, &job.file.RelativePath, &job.file.SizeBytes, &job.file.SHA256, &job.file.LineCount)
	if err != nil {
		return agentJob{}, err
	}
	job.overview = jobType == "overview"
	job.runToken = newID()
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs
		SET status='running', attempt_count=attempt_count+1, worker_id=$2, run_token=$3::uuid,
			lease_expires_at=NOW()+($4*INTERVAL '1 second'), heartbeat_at=NOW(),
			started_at=COALESCE(started_at,NOW()), completed_at=NULL, error_code='', error_message='', updated_at=NOW()
		WHERE id=$1 AND status='queued'`, job.queueID, workerID, job.runToken, int64(agentJobLeaseDuration/time.Second))
	if err != nil {
		return agentJob{}, err
	}
	if err = requireAffectedRow(result); err != nil {
		return agentJob{}, err
	}
	if err = recomputeAIStatusTx(ctx, tx, job.taskID); err != nil {
		return agentJob{}, err
	}
	if err = tx.Commit(); err != nil {
		return agentJob{}, err
	}
	if !job.overview {
		results, loadErr := r.Results(ctx, job.taskID, job.ownerOpenID, 100000, 0)
		if loadErr != nil {
			_ = r.FailClaimedAgentJob(context.Background(), job.queueID, job.runToken, "internal_error", loadErr.Error())
			return agentJob{}, loadErr
		}
		for _, result := range results {
			if result.FilePath == job.file.RelativePath {
				job.matches = append(job.matches, result)
			}
		}
		job.totalLines = job.file.LineCount
	}
	return job, nil
}

func (r *Repository) RenewAgentJobLease(ctx context.Context, jobID int64, runToken string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs
		SET lease_expires_at=NOW()+($3*INTERVAL '1 second'), heartbeat_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND run_token=$2::uuid AND status='running'`, jobID, runToken, int64(agentJobLeaseDuration/time.Second))
	if err != nil {
		return err
	}
	return requireAffectedRow(result)
}

func (r *Repository) FinalizeAgentJob(ctx context.Context, jobID int64, runToken string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taskID, jobType string
	var attemptNo int
	var fileID sql.NullInt64
	var cancelled bool
	err = tx.QueryRowContext(ctx, `SELECT job.task_id,job.attempt_no,job.job_type,job.log_file_id,task.ai_cancel_requested
		FROM logmaster_api.parse_tasks task JOIN logmaster_api.ai_jobs job ON job.task_id=task.id
		WHERE job.id=$1 AND job.run_token=$2::uuid AND job.status='running' FOR UPDATE OF task,job`, jobID, runToken).
		Scan(&taskID, &attemptNo, &jobType, &fileID, &cancelled)
	if err != nil {
		return err
	}
	status, errorCode, errorMessage := "failed", "internal_error", "AI 作业未生成结果"
	if cancelled {
		status, errorCode, errorMessage = "cancelled", "cancelled", "AI 分析已取消"
	} else if jobType == "overview" {
		err = tx.QueryRowContext(ctx, `SELECT status,error_code,error_message FROM logmaster_api.task_ai_overviews
			WHERE task_id=$1 AND attempt_no=$2`, taskID, attemptNo).
			Scan(&status, &errorCode, &errorMessage)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT status,error_code,error_message FROM logmaster_api.agent_analyses
			WHERE task_id=$1 AND attempt_no=$2 AND log_file_id=$3 ORDER BY updated_at DESC LIMIT 1`, taskID, attemptNo, fileID.Int64).
			Scan(&status, &errorCode, &errorMessage)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if status != "completed" && status != "cancelled" {
		status = "failed"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status=$3,error_code=$4,error_message=$5,
		completed_at=NOW(),lease_expires_at=NULL,heartbeat_at=NOW(),worker_id='',run_token=NULL,updated_at=NOW()
		WHERE id=$1 AND run_token=$2::uuid`, jobID, runToken, status, errorCode, errorMessage); err != nil {
		return err
	}
	if err = recomputeAIStatusTx(ctx, tx, taskID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) FailClaimedAgentJob(ctx context.Context, jobID int64, runToken, errorCode, errorMessage string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taskID string
	err = tx.QueryRowContext(ctx, `SELECT task.id FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.ai_jobs job ON job.task_id=task.id
		WHERE job.id=$1 AND job.run_token=$2::uuid AND job.status='running' FOR UPDATE OF task,job`, jobID, runToken).Scan(&taskID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='failed',error_code=$3,error_message=$4,
		completed_at=NOW(),lease_expires_at=NULL,heartbeat_at=NOW(),worker_id='',run_token=NULL,updated_at=NOW()
		WHERE id=$1 AND run_token=$2::uuid AND status='running'`, jobID, runToken, errorCode, errorMessage)
	if err != nil {
		return err
	}
	if err = requireAffectedRow(result); err != nil {
		return err
	}
	if err = recomputeAIStatusTx(ctx, tx, taskID); err != nil {
		return err
	}
	return tx.Commit()
}

func recomputeAIStatusTx(ctx context.Context, tx *sql.Tx, taskID string) error {
	result, err := tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks task SET
		ai_status=CASE
			WHEN task.ai_cancel_requested THEN 'cancelled'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='running') THEN 'running'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='queued') THEN 'queued'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='completed')
			 AND EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status IN ('failed','cancelled')) THEN 'partial_failed'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='failed') THEN 'failed'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='completed') THEN 'completed'
			WHEN EXISTS (SELECT 1 FROM logmaster_api.ai_jobs job WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='cancelled') THEN 'cancelled'
			ELSE 'disabled' END,
		ai_error_message=COALESCE((SELECT job.error_message FROM logmaster_api.ai_jobs job
			WHERE job.task_id=task.id AND job.attempt_no=task.attempt_no AND job.status='failed'
			ORDER BY job.updated_at DESC LIMIT 1),''), updated_at=NOW()
		WHERE task.id=$1`, taskID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return fmt.Errorf("AI task not found")
	}
	return nil
}
