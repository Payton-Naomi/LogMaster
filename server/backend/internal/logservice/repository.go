package logservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"logmaster-agent/internal/rolepolicy"
)

type Repository struct{ db *sql.DB }

type Notification struct {
	ID        int64      `json:"id"`
	TaskID    string     `json:"task_id,omitempty"`
	UploadID  string     `json:"upload_id,omitempty"`
	ResultID  int64      `json:"result_id,omitempty"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	IsRead    bool       `json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (r *Repository) CreateTaskNotifications(ctx context.Context, taskID, notificationType, title, message string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.notifications
		(recipient_open_id, task_id, upload_id, type, title, message, dedupe_key)
		SELECT DISTINCT recipient.open_id, task.id, upload.id, $2, $3, $4,
			$2||':'||task.id::text||':'||task.attempt_no::text||':'||recipient.open_id
		FROM logmaster_api.parse_tasks task JOIN logmaster_api.log_uploads upload ON upload.id=task.upload_id
		CROSS JOIN LATERAL (
			SELECT upload.created_by_open_id AS open_id
			UNION SELECT access.user_open_id FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.upload_session_id=upload.upload_session_id
		) recipient
		LEFT JOIN logmaster_api.notification_settings settings ON settings.user_open_id=recipient.open_id
		WHERE task.id=$1 AND recipient.open_id<>'' AND EXISTS (
			SELECT 1 FROM logmaster_api.users user_record WHERE user_record.feishu_open_id=recipient.open_id)
		AND COALESCE(CASE $2
			WHEN 'task_completed' THEN settings.task_completed
			WHEN 'task_failed' THEN settings.task_failed
			WHEN 'task_cancelled' THEN settings.task_cancelled
			ELSE TRUE END,TRUE)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`, taskID, notificationType, title, message)
	return err
}

func (r *Repository) ListNotifications(ctx context.Context, recipient string, unreadOnly bool, limit, offset int) ([]Notification, int, int, error) {
	filter := "recipient_open_id=$1"
	if unreadOnly {
		filter += " AND is_read=FALSE"
	}
	var total, unread int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE `+filter+`), COUNT(*) FILTER (WHERE recipient_open_id=$1 AND is_read=FALSE) FROM logmaster_api.notifications`, recipient).Scan(&total, &unread); err != nil {
		return nil, 0, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, COALESCE(task_id::text,''), COALESCE(upload_id::text,''), COALESCE(result_id,0), type, title, message, is_read, read_at, created_at
		FROM logmaster_api.notifications WHERE `+filter+` ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, recipient, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		var readAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TaskID, &item.UploadID, &item.ResultID, &item.Type, &item.Title, &item.Message, &item.IsRead, &readAt, &item.CreatedAt); err != nil {
			return nil, 0, 0, err
		}
		if readAt.Valid {
			item.ReadAt = &readAt.Time
		}
		items = append(items, item)
	}
	return items, total, unread, rows.Err()
}

func (r *Repository) MarkNotificationRead(ctx context.Context, id int64, recipient string) (Notification, error) {
	var item Notification
	var readAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.notifications SET is_read=TRUE, read_at=COALESCE(read_at,NOW())
		WHERE id=$1 AND recipient_open_id=$2 RETURNING id, COALESCE(task_id::text,''), COALESCE(upload_id::text,''), COALESCE(result_id,0), type, title, message, is_read, read_at, created_at`, id, recipient).
		Scan(&item.ID, &item.TaskID, &item.UploadID, &item.ResultID, &item.Type, &item.Title, &item.Message, &item.IsRead, &readAt, &item.CreatedAt)
	if readAt.Valid {
		item.ReadAt = &readAt.Time
	}
	return item, err
}

func (r *Repository) MarkAllNotificationsRead(ctx context.Context, recipient string) (int64, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.notifications
		SET is_read=TRUE,read_at=COALESCE(read_at,NOW()) WHERE recipient_open_id=$1 AND is_read=FALSE`, recipient)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Repository) NotificationsAfter(ctx context.Context, recipient string, afterID int64, limit int) ([]Notification, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,COALESCE(task_id::text,''),COALESCE(upload_id::text,''),COALESCE(result_id,0),
		type,title,message,is_read,read_at,created_at FROM logmaster_api.notifications
		WHERE recipient_open_id=$1 AND id>$2 ORDER BY id LIMIT $3`, recipient, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		var readAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TaskID, &item.UploadID, &item.ResultID, &item.Type, &item.Title,
			&item.Message, &item.IsRead, &readAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		if readAt.Valid {
			item.ReadAt = &readAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) LatestNotificationID(ctx context.Context, recipient string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM logmaster_api.notifications WHERE recipient_open_id=$1`, recipient).Scan(&id)
	return id, err
}

type NotificationSettings struct {
	TaskCompleted   bool `json:"task_completed"`
	TaskFailed      bool `json:"task_failed"`
	TaskCancelled   bool `json:"task_cancelled"`
	AICompleted     bool `json:"ai_completed"`
	AIFailed        bool `json:"ai_failed"`
	ResultAssigned  bool `json:"result_assigned"`
	ResultCommented bool `json:"result_commented"`
}

func (r *Repository) GetNotificationSettings(ctx context.Context, userOpenID string) (NotificationSettings, error) {
	settings := NotificationSettings{
		TaskCompleted: true, TaskFailed: true, TaskCancelled: true,
		AICompleted: true, AIFailed: true, ResultAssigned: true, ResultCommented: true,
	}
	err := r.db.QueryRowContext(ctx, `SELECT task_completed,task_failed,task_cancelled,ai_completed,ai_failed,result_assigned,result_commented
		FROM logmaster_api.notification_settings WHERE user_open_id=$1`, userOpenID).Scan(
		&settings.TaskCompleted, &settings.TaskFailed, &settings.TaskCancelled, &settings.AICompleted,
		&settings.AIFailed, &settings.ResultAssigned, &settings.ResultCommented)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

func (r *Repository) SaveNotificationSettings(ctx context.Context, userOpenID string, settings NotificationSettings) (NotificationSettings, error) {
	err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.notification_settings
		(user_open_id,task_completed,task_failed,task_cancelled,ai_completed,ai_failed,result_assigned,result_commented)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (user_open_id) DO UPDATE SET task_completed=EXCLUDED.task_completed,task_failed=EXCLUDED.task_failed,
		task_cancelled=EXCLUDED.task_cancelled,ai_completed=EXCLUDED.ai_completed,ai_failed=EXCLUDED.ai_failed,
		result_assigned=EXCLUDED.result_assigned,result_commented=EXCLUDED.result_commented,updated_at=NOW()
		RETURNING task_completed,task_failed,task_cancelled,ai_completed,ai_failed,result_assigned,result_commented`,
		userOpenID, settings.TaskCompleted, settings.TaskFailed, settings.TaskCancelled, settings.AICompleted,
		settings.AIFailed, settings.ResultAssigned, settings.ResultCommented).Scan(
		&settings.TaskCompleted, &settings.TaskFailed, &settings.TaskCancelled, &settings.AICompleted,
		&settings.AIFailed, &settings.ResultAssigned, &settings.ResultCommented)
	return settings, err
}

func (r *Repository) CreatePendingAINotifications(ctx context.Context, taskID string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.notifications
		(recipient_open_id,task_id,upload_id,type,title,message,dedupe_key)
		SELECT DISTINCT recipient.open_id,task.id,upload.id,
			CASE WHEN task.ai_status='completed' THEN 'ai_completed' ELSE 'ai_failed' END,
			CASE WHEN task.ai_status='completed' THEN 'AI 分析完成' ELSE 'AI 分析存在失败' END,
			CASE WHEN task.ai_status='completed' THEN '文件分析和任务总览已完成'
				ELSE COALESCE(NULLIF(task.ai_error_message,''),'部分或全部 AI 作业执行失败') END,
			'ai:'||task.id::text||':'||task.attempt_no::text||':'||task.ai_status||':'||recipient.open_id
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id=task.upload_id
		CROSS JOIN LATERAL (
			SELECT upload.created_by_open_id AS open_id
			UNION SELECT access.user_open_id FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.upload_session_id=upload.upload_session_id
		) recipient
		LEFT JOIN logmaster_api.notification_settings settings ON settings.user_open_id=recipient.open_id
		WHERE ($1='' OR task.id=NULLIF($1,'')::uuid) AND task.ai_status IN ('completed','partial_failed','failed') AND recipient.open_id<>''
		AND EXISTS (SELECT 1 FROM logmaster_api.users user_record WHERE user_record.feishu_open_id=recipient.open_id AND user_record.identity_type='feishu')
		AND CASE WHEN task.ai_status='completed' THEN COALESCE(settings.ai_completed,TRUE) ELSE COALESCE(settings.ai_failed,TRUE) END
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`, taskID)
	return err
}

func (r *Repository) CreateResultAssignmentNotification(ctx context.Context, resultID int64, actorOpenID string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.notifications
		(recipient_open_id,task_id,upload_id,result_id,type,title,message)
		SELECT result.assigned_to_open_id,result.task_id,upload.id,result.id,'result_assigned','收到新的异常任务',
			'异常结果已分配给你：'||file.relative_path||':'||result.line_number
		FROM logmaster_api.parse_results result
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id
		JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		LEFT JOIN logmaster_api.notification_settings settings ON settings.user_open_id=result.assigned_to_open_id
		WHERE result.id=$1 AND result.assigned_to_open_id IS NOT NULL AND result.assigned_to_open_id<>$2
		AND COALESCE(settings.result_assigned,TRUE)`, resultID, actorOpenID)
	return err
}

func (r *Repository) CreateResultCommentNotifications(ctx context.Context, resultID int64, actorOpenID string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.notifications
		(recipient_open_id,task_id,upload_id,result_id,type,title,message)
		SELECT DISTINCT recipient.open_id,result.task_id,upload.id,result.id,'result_commented','异常结果有新备注',
			file.relative_path||':'||result.line_number||' 添加了新备注'
		FROM logmaster_api.parse_results result
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id
		JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		CROSS JOIN LATERAL (
			SELECT upload.created_by_open_id AS open_id
			UNION SELECT result.assigned_to_open_id
			UNION SELECT access.user_open_id FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.upload_session_id=upload.upload_session_id
		) recipient
		LEFT JOIN logmaster_api.notification_settings settings ON settings.user_open_id=recipient.open_id
		WHERE result.id=$1 AND recipient.open_id IS NOT NULL AND recipient.open_id<>'' AND recipient.open_id<>$2
		AND EXISTS (SELECT 1 FROM logmaster_api.users user_record WHERE user_record.feishu_open_id=recipient.open_id)
		AND COALESCE(settings.result_commented,TRUE)`, resultID, actorOpenID)
	return err
}

type AgentRetryInput struct {
	TaskID    string
	UploadID  string
	AttemptNo int
	Files     []LogFile
	Matches   map[string][]ParseResult
}

var (
	ErrProjectNotFound          = errors.New("project not found")
	ErrScenarioNotApplicable    = errors.New("scenario does not exist, is not published, or does not apply to the project")
	ErrScenarioRuleUnavailable  = errors.New("scenario contains a rule unavailable to the current user")
	ErrUploaderEmailNotFound    = errors.New("uploader email is not registered")
	ErrUploaderEmailAmbiguous   = errors.New("uploader email matches multiple users")
	ErrUploaderEmailMismatch    = errors.New("uploader name does not match uploader email")
	ErrUploaderEmailNotInternal = errors.New("uploader email is not an active enterprise member")
	ErrAssignedUserNotFound     = errors.New("assigned user not found")
)

func (r *Repository) UpsertCollectorIdentity(ctx context.Context, identity collectorIdentity) error {
	role := rolepolicy.ForJobTitle(identity.JobTitle)
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.users (feishu_open_id, name, email, job_title, role, role_source)
		VALUES ($1,$2,$3,$4,$5,'feishu')
		ON CONFLICT (feishu_open_id) DO UPDATE SET name=EXCLUDED.name,email=EXCLUDED.email,
			job_title=EXCLUDED.job_title,
			role=CASE WHEN logmaster_api.users.role_source='feishu' THEN EXCLUDED.role ELSE logmaster_api.users.role END,
			updated_at=NOW()`,
		identity.OpenID, identity.Name, identity.Email, identity.JobTitle, role)
	return err
}

// GrantCollectorSessionAccess makes a Feishu-verified uploader the owner-facing
// viewer of a collector session without changing the collector's upload owner.
func (r *Repository) GrantCollectorSessionAccess(ctx context.Context, sessionID, userOpenID string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.user_collected_upload_sessions
		(user_open_id, upload_session_id) VALUES ($1, $2)
		ON CONFLICT (user_open_id, upload_session_id) DO UPDATE SET accessed_at = NOW()`, userOpenID, sessionID)
	return err
}

// PrepareAgentRetry clears only AI output and advances the result generation.
// Rule parse results and source files are intentionally left untouched.
func (r *Repository) PrepareAgentRetry(ctx context.Context, taskID, ownerOpenID string) (AgentRetryInput, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AgentRetryInput{}, err
	}
	defer tx.Rollback()
	var input AgentRetryInput
	var status string
	var retryRequested bool
	err = tx.QueryRowContext(ctx, `SELECT task.id, task.upload_id, task.status, task.attempt_no, task.ai_retry_requested
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		WHERE task.id = $1 AND (upload.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = upload.upload_session_id
		)) FOR UPDATE OF task`, taskID, ownerOpenID).Scan(&input.TaskID, &input.UploadID, &status, &input.AttemptNo, &retryRequested)
	if err != nil {
		return AgentRetryInput{}, err
	}
	if status != "completed" {
		return AgentRetryInput{}, ErrAgentRetryNotReady
	}
	if retryRequested {
		return AgentRetryInput{}, ErrAgentRetryQueued
	}
	input.AttemptNo++
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='cancelled', completed_at=NOW(),
		worker_id='', run_token=NULL, lease_expires_at=NULL, error_code='cancelled', error_message='已由新的 AI 重试代次替代', updated_at=NOW()
		WHERE task_id=$1 AND status IN ('queued','running')`, taskID); err != nil {
		return AgentRetryInput{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET attempt_no = $2, ai_retry_requested = TRUE, ai_cancel_requested = FALSE,
			ai_status='queued', ai_error_message='', updated_at = NOW() WHERE id = $1`, taskID, input.AttemptNo); err != nil {
		return AgentRetryInput{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.task_ai_overviews WHERE task_id = $1`, taskID); err != nil {
		return AgentRetryInput{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.agent_analyses WHERE task_id = $1`, taskID); err != nil {
		return AgentRetryInput{}, err
	}
	if err = tx.Commit(); err != nil {
		return AgentRetryInput{}, err
	}
	_, input.Files, err = r.GetUploadByTask(ctx, taskID, ownerOpenID)
	if err != nil {
		_ = r.ClearAgentRetryRequested(context.Background(), taskID, input.AttemptNo)
		return AgentRetryInput{}, err
	}
	results, err := r.Results(ctx, taskID, ownerOpenID, 100000, 0)
	if err != nil {
		_ = r.ClearAgentRetryRequested(context.Background(), taskID, input.AttemptNo)
		return AgentRetryInput{}, err
	}
	input.Matches = make(map[string][]ParseResult)
	for _, result := range results {
		input.Matches[result.FilePath] = append(input.Matches[result.FilePath], result)
	}
	return input, nil
}

// PrepareAgentFileRetry replaces one file's AI output without advancing the
// parse attempt, so all other current file results remain visible.
func (r *Repository) PrepareAgentFileRetry(ctx context.Context, taskID, ownerOpenID string, fileID int64) (AgentRetryInput, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AgentRetryInput{}, err
	}
	defer tx.Rollback()
	var input AgentRetryInput
	var status string
	var retryRequested bool
	err = tx.QueryRowContext(ctx, `SELECT task.id, task.upload_id, task.status, task.attempt_no, task.ai_retry_requested
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		WHERE task.id = $1 AND (upload.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = upload.upload_session_id
		)) FOR UPDATE OF task`, taskID, ownerOpenID).Scan(&input.TaskID, &input.UploadID, &status, &input.AttemptNo, &retryRequested)
	if err != nil {
		return AgentRetryInput{}, err
	}
	if status != "completed" {
		return AgentRetryInput{}, ErrAgentRetryNotReady
	}
	if retryRequested {
		return AgentRetryInput{}, ErrAgentRetryQueued
	}
	var file LogFile
	err = tx.QueryRowContext(ctx, `SELECT id, relative_path, size_bytes, sha256, line_count
		FROM logmaster_api.log_files WHERE id = $1 AND upload_id = $2`, fileID, input.UploadID).
		Scan(&file.ID, &file.RelativePath, &file.SizeBytes, &file.SHA256, &file.LineCount)
	if err != nil {
		return AgentRetryInput{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET ai_retry_requested = TRUE, ai_cancel_requested = FALSE,
			ai_status='queued', ai_error_message='', updated_at = NOW() WHERE id = $1`, taskID); err != nil {
		return AgentRetryInput{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM logmaster_api.agent_analyses
		WHERE task_id = $1 AND log_file_id = $2`, taskID, fileID); err != nil {
		return AgentRetryInput{}, err
	}
	if err = tx.Commit(); err != nil {
		return AgentRetryInput{}, err
	}
	input.Files = []LogFile{file}
	input.Matches = map[string][]ParseResult{file.RelativePath: {}}
	results, err := r.Results(ctx, taskID, ownerOpenID, 100000, 0)
	if err != nil {
		_ = r.ClearAgentRetryRequested(context.Background(), taskID, input.AttemptNo)
		return AgentRetryInput{}, err
	}
	for _, result := range results {
		if result.FilePath == file.RelativePath {
			input.Matches[file.RelativePath] = append(input.Matches[file.RelativePath], result)
		}
	}
	return input, nil
}

func (r *Repository) CancelAgentAnalysis(ctx context.Context, taskID, ownerOpenID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var status string
	var cancelled bool
	err = tx.QueryRowContext(ctx, `SELECT task.status, task.ai_cancel_requested
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		WHERE task.id = $1 AND (upload.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = upload.upload_session_id
		)) FOR UPDATE OF task`, taskID, ownerOpenID).Scan(&status, &cancelled)
	if err != nil {
		return false, err
	}
	if status != "completed" {
		return false, ErrAgentNotCancellable
	}
	if cancelled {
		return true, nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET ai_cancel_requested=TRUE,ai_retry_requested=FALSE,ai_status='cancelled',ai_error_message='',updated_at=NOW()
		WHERE id=$1`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='cancelled',completed_at=NOW(),
		worker_id='',run_token=NULL,lease_expires_at=NULL,error_code='cancelled',error_message='AI 分析已由用户取消',updated_at=NOW()
		WHERE task_id=$1 AND status IN ('queued','running')`, taskID); err != nil {
		return false, err
	}
	return false, tx.Commit()
}

func (r *Repository) ClearAgentRetryRequested(ctx context.Context, taskID string, attemptNo int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET ai_retry_requested = FALSE, updated_at = NOW()
		WHERE id = $1 AND attempt_no = $2`, taskID, attemptNo)
	return err
}

func (r *Repository) CancelTask(ctx context.Context, taskID, ownerOpenID string) (alreadyCancelled bool, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var uploadID, taskStatus, uploadStatus, runToken string
	err = tx.QueryRowContext(ctx, `SELECT task.upload_id, task.status, upload.status, COALESCE(task.run_token::text, '')
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		WHERE task.id = $1 AND (upload.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = upload.upload_session_id
		)) FOR UPDATE OF task, upload`, taskID, ownerOpenID).Scan(&uploadID, &taskStatus, &uploadStatus, &runToken)
	if err != nil {
		return false, err
	}
	if taskStatus == "cancelled" {
		return true, nil
	}
	if (taskStatus != "queued" && taskStatus != "running" && taskStatus != "paused") || (uploadStatus != "queued" && uploadStatus != "parsing" && uploadStatus != "paused") {
		return false, ErrTaskNotCancellable
	}
	if runToken != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts
			SET status = 'interrupted', error_message = 'task cancelled by user', completed_at = NOW(), heartbeat_at = NOW()
			WHERE task_id = $1 AND run_token = $2::uuid AND status = 'running'`, taskID, runToken); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'cancelled', error_message = 'task cancelled by user', completed_at = NOW(),
			worker_id = '', run_token = NULL, lease_expires_at = NULL, ai_retry_requested = FALSE,
			ai_cancel_requested=TRUE, ai_status='cancelled', ai_error_message='', updated_at = NOW()
		WHERE id = $1`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='cancelled',completed_at=NOW(),
		worker_id='',run_token=NULL,lease_expires_at=NULL,error_code='cancelled',error_message='规则解析任务已取消',updated_at=NOW()
		WHERE task_id=$1 AND status IN ('queued','running')`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'cancelled', error_message = 'task cancelled by user', updated_at = NOW() WHERE id = $1`, uploadID); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	_ = r.CreateTaskNotifications(ctx, taskID, "task_cancelled", "日志任务已取消", "日志解析任务已由用户取消")
	return false, nil
}

func (r *Repository) PauseTask(ctx context.Context, taskID, ownerOpenID string) (alreadyPaused bool, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var uploadID, taskStatus, runToken string
	err = tx.QueryRowContext(ctx, `SELECT task.upload_id, task.status, COALESCE(task.run_token::text,'')
		FROM logmaster_api.parse_tasks task JOIN logmaster_api.log_uploads upload ON upload.id=task.upload_id
		WHERE task.id=$1 AND (upload.created_by_open_id=$2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id))
		FOR UPDATE OF task, upload`, taskID, ownerOpenID).Scan(&uploadID, &taskStatus, &runToken)
	if err != nil {
		return false, err
	}
	if taskStatus == "paused" {
		return true, nil
	}
	if taskStatus != "queued" && taskStatus != "running" {
		return false, ErrTaskNotPausable
	}
	if runToken != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_task_attempts SET status='interrupted', error_message='任务已暂停', completed_at=NOW(), heartbeat_at=NOW() WHERE task_id=$1 AND run_token=$2::uuid AND status='running'`, taskID, runToken); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status='paused', error_message='任务已暂停', worker_id='', run_token=NULL, lease_expires_at=NULL, completed_at=NULL, updated_at=NOW() WHERE id=$1`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status='paused', error_message='任务已暂停', updated_at=NOW() WHERE id=$1`, uploadID); err != nil {
		return false, err
	}
	return false, tx.Commit()
}

func (r *Repository) ResumeTask(ctx context.Context, taskID, ownerOpenID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var uploadID, status string
	err = tx.QueryRowContext(ctx, `SELECT task.upload_id,task.status FROM logmaster_api.parse_tasks task JOIN logmaster_api.log_uploads upload ON upload.id=task.upload_id
		WHERE task.id=$1 AND (upload.created_by_open_id=$2 OR EXISTS (SELECT 1 FROM logmaster_api.user_collected_upload_sessions access WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id)) FOR UPDATE OF task,upload`, taskID, ownerOpenID).Scan(&uploadID, &status)
	if err != nil {
		return err
	}
	if status != "paused" {
		return ErrTaskNotResumable
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status='queued',error_message='',manual_retry_requested=TRUE,worker_id='',run_token=NULL,lease_expires_at=NULL,completed_at=NULL,updated_at=NOW() WHERE id=$1`, taskID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status='queued',error_message='',updated_at=NOW() WHERE id=$1`, uploadID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) UpdateTaskPriority(ctx context.Context, taskID, ownerOpenID string, priority int) error {
	result, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks task SET priority=$3,updated_at=NOW()
		FROM logmaster_api.log_uploads upload WHERE task.id=$1 AND upload.id=task.upload_id
		AND task.status IN ('queued','paused') AND (upload.created_by_open_id=$2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id))`, taskID, ownerOpenID, priority)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrTaskPriorityNotEditable
	}
	return nil
}

func (r *Repository) IsParseTaskStopped(ctx context.Context, taskID string) (bool, error) {
	var stopped bool
	err := r.db.QueryRowContext(ctx, `SELECT status IN ('cancelled','paused') FROM logmaster_api.parse_tasks WHERE id = $1`, taskID).Scan(&stopped)
	return stopped, err
}

type Upload struct {
	ID               string     `json:"id"`
	TaskID           string     `json:"task_id"`
	ProjectID        string     `json:"project_id"`
	ProjectName      string     `json:"project_name"`
	Version          string     `json:"version"`
	TestTaskID       string     `json:"test_task_id,omitempty"`
	TestTaskName     string     `json:"test_task_name,omitempty"`
	UploaderName     string     `json:"uploader_name,omitempty"`
	UploaderID       string     `json:"uploader_id,omitempty"`
	UploaderEmail    string     `json:"uploader_email,omitempty"`
	Remark           string     `json:"remark,omitempty"`
	ClientRequestID  string     `json:"client_request_id,omitempty"`
	QueryCode        string     `json:"query_code,omitempty"`
	UploadSessionID  string     `json:"upload_session_id,omitempty"`
	UploadPosition   int        `json:"upload_position,omitempty"`
	CollectorVersion string     `json:"collector_version,omitempty"`
	Timezone         string     `json:"timezone,omitempty"`
	ClientCreatedAt  *time.Time `json:"client_created_at,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	ScenarioID       string     `json:"scenario_id,omitempty"`
	ScenarioName     string     `json:"scenario_name,omitempty"`
	SourceType       string     `json:"source_type"`
	Status           string     `json:"status"`
	Priority         int        `json:"priority"`
	AIStatus         string     `json:"ai_status"`
	AIErrorMessage   string     `json:"ai_error_message,omitempty"`
	OriginalName     string     `json:"original_name"`
	OriginalSize     int64      `json:"original_size"`
	FileCount        int        `json:"file_count"`
	TotalFiles       int        `json:"total_files"`
	ProcessedFiles   int        `json:"processed_files"`
	TotalBytes       int64      `json:"total_bytes"`
	ProcessedBytes   int64      `json:"processed_bytes"`
	Progress         int        `json:"progress"`
	TotalLines       int64      `json:"total_lines"`
	ErrorCount       int64      `json:"error_count"`
	WarningCount     int64      `json:"warning_count"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type UploadMetadata struct {
	UploadSessionID     string
	ProjectID           string
	ProjectName         string
	Version             string
	TestTaskID          string
	TestTaskName        string
	UploaderName        string
	UploaderID          string
	Remark              string
	ClientRequestID     string
	QueryCode           string
	CollectorVersion    string
	Timezone            string
	DisableParsingRules bool
	AIAnalysisEnabled   bool
	CreatedAt           *time.Time
	StartedAt           *time.Time
	EndedAt             *time.Time
}

type PublicUploadStatus struct {
	UploadID       string    `json:"upload_id"`
	TaskID         string    `json:"task_id"`
	QueryCode      string    `json:"query_code"`
	ProjectName    string    `json:"project_name"`
	Version        string    `json:"version"`
	Status         string    `json:"status"`
	TotalFiles     int       `json:"total_files"`
	ProcessedFiles int       `json:"processed_files"`
	TotalLines     int64     `json:"total_lines"`
	ErrorCount     int64     `json:"error_count"`
	WarningCount   int64     `json:"warning_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PublicUploadBatch struct {
	UploadID        string    `json:"upload_id"`
	TaskID          string    `json:"task_id"`
	ClientRequestID string    `json:"client_request_id"`
	QueryCode       string    `json:"query_code"`
	Status          string    `json:"status"`
	TotalFiles      int       `json:"total_files"`
	ProcessedFiles  int       `json:"processed_files"`
	TotalLines      int64     `json:"total_lines"`
	ErrorCount      int64     `json:"error_count"`
	WarningCount    int64     `json:"warning_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PublicUploadSessionStatus struct {
	UploadSessionID string              `json:"upload_session_id"`
	QueryCode       string              `json:"query_code"`
	ProjectName     string              `json:"project_name"`
	Version         string              `json:"version"`
	TestTaskName    string              `json:"test_task_name"`
	UploaderName    string              `json:"uploader_name"`
	Status          string              `json:"status"`
	BatchCount      int                 `json:"batch_count"`
	TotalFiles      int                 `json:"total_files"`
	ProcessedFiles  int                 `json:"processed_files"`
	TotalLines      int64               `json:"total_lines"`
	ErrorCount      int64               `json:"error_count"`
	WarningCount    int64               `json:"warning_count"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Batches         []PublicUploadBatch `json:"batches"`
}

type LogFile struct {
	ID           int64  `json:"id"`
	RelativePath string `json:"relative_path"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	LineCount    int64  `json:"line_count"`
}

type ContextLine struct {
	LineNumber int64      `json:"line_number"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
	Level      string     `json:"level,omitempty"`
	Content    string     `json:"content"`
	IsHit      bool       `json:"is_hit"`
}

type RelatedCause struct {
	Kind       string     `json:"kind"`
	Label      string     `json:"label"`
	Reason     string     `json:"reason"`
	Confidence float64    `json:"confidence"`
	LineNumber int64      `json:"line_number"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
	Content    string     `json:"content"`
}

type ParseResult struct {
	ID               int64          `json:"id,omitempty"`
	Status           string         `json:"status"`
	AssignedTo       string         `json:"assigned_to,omitempty"`
	AssignedAt       *time.Time     `json:"assigned_at,omitempty"`
	Level            string         `json:"level"`
	MatchedText      string         `json:"matched_text"`
	LineNumber       int64          `json:"line_number"`
	Content          string         `json:"content"`
	FilePath         string         `json:"file_path"`
	RuleID           int64          `json:"rule_id,omitempty"`
	RuleName         string         `json:"rule_name,omitempty"`
	Category         string         `json:"category,omitempty"`
	EventTime        *time.Time     `json:"event_time,omitempty"`
	ContextStartTime *time.Time     `json:"context_start_time,omitempty"`
	ContextEndTime   *time.Time     `json:"context_end_time,omitempty"`
	ContextLines     []ContextLine  `json:"context_lines,omitempty"`
	RelatedCauses    []RelatedCause `json:"related_causes,omitempty"`
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ArchivePasswords(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT password FROM logmaster_api.archive_passwords ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	passwords := make([]string, 0)
	for rows.Next() {
		var password string
		if err := rows.Scan(&password); err != nil {
			return nil, err
		}
		passwords = append(passwords, password)
	}
	return passwords, rows.Err()
}

type scenarioSnapshot struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Checks []ScenarioCheck `json:"checks"`
}

type uploadScenarioSnapshot struct {
	Name                string             `json:"name"`
	Scenarios           []scenarioSnapshot `json:"scenarios"`
	DisableParsingRules *bool              `json:"disable_parsing_rules,omitempty"`
}

func (r *Repository) CreateUpload(ctx context.Context, uploadID, taskID, projectName, version string, scenarioIDs []string, storagePath, creatorOpenID string) error {
	return r.CreateUploadWithMetadata(ctx, uploadID, taskID, UploadMetadata{
		ProjectName:         projectName,
		Version:             version,
		UploaderID:          creatorOpenID,
		DisableParsingRules: true,
		AIAnalysisEnabled:   true,
	}, scenarioIDs, storagePath, creatorOpenID)
}

func (r *Repository) CreateUploadWithMetadata(ctx context.Context, uploadID, taskID string, metadata UploadMetadata, scenarioIDs []string, storagePath, creatorOpenID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(scenarioIDs) == 0 {
		matchedScenarioIDs, err := resolveTaskScenarioIDs(ctx, tx, metadata.TestTaskID, metadata.TestTaskName)
		if err != nil {
			return err
		}
		scenarioIDs = matchedScenarioIDs
	}

	var projectID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM logmaster_api.projects
		WHERE name = $1 AND ($2 = '' OR id::text = $2) AND is_active = TRUE`, metadata.ProjectName, metadata.ProjectID).Scan(&projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("find project: %w", err)
	}
	var snapshots []scenarioSnapshot
	for _, scenarioID := range scenarioIDs {
		var name string
		var metadataJSON, checksJSON []byte
		err = tx.QueryRowContext(ctx, `SELECT name, metadata, checks
			FROM logmaster_api.test_scenarios WHERE id = $1`, scenarioID).Scan(&name, &metadataJSON, &checksJSON)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrScenarioNotApplicable
		}
		if err != nil {
			return fmt.Errorf("find scenario: %w", err)
		}
		var scenarioMetadata ScenarioMetadata
		if json.Unmarshal(metadataJSON, &scenarioMetadata) != nil {
			return ErrScenarioNotApplicable
		}
		if scenarioMetadata.Status == "" {
			scenarioMetadata.Status = "published"
		}
		if scenarioMetadata.ProjectScope == "" {
			scenarioMetadata.ProjectScope = "all"
		}
		if scenarioMetadata.Status != "published" ||
			(scenarioMetadata.ProjectScope == "selected" && !containsString(scenarioMetadata.Projects, metadata.ProjectName)) {
			return ErrScenarioNotApplicable
		}
		var checks []ScenarioCheck
		if err := json.Unmarshal(checksJSON, &checks); err != nil {
			return fmt.Errorf("decode scenario checks: %w", err)
		}
		for index := range checks {
			check := &checks[index]
			if check.Source == "" {
				check.Source = "custom"
			}
			if !check.Enabled || check.Source != "rule" || check.RuleID == nil {
				continue
			}
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM logmaster_api.parse_rules
				WHERE id = $1 AND (created_by_open_id IS NULL OR created_by_open_id = $2))`,
				*check.RuleID, creatorOpenID).Scan(&exists); err != nil {
				return fmt.Errorf("validate scenario rule: %w", err)
			}
			if !exists {
				return ErrScenarioRuleUnavailable
			}
		}
		snapshots = append(snapshots, scenarioSnapshot{ID: scenarioID, Name: name, Checks: checks})
	}
	var snapshotJSON []byte
	primaryScenarioID := ""
	if len(snapshots) > 0 {
		primaryScenarioID = snapshots[0].ID
		names := make([]string, 0, len(snapshots))
		for _, snapshot := range snapshots {
			names = append(names, snapshot.Name)
		}
		disableParsingRules := metadata.DisableParsingRules
		snapshotJSON, err = json.Marshal(uploadScenarioSnapshot{
			Name: strings.Join(names, "、"), Scenarios: snapshots, DisableParsingRules: &disableParsingRules,
		})
		if err != nil {
			return fmt.Errorf("encode scenario snapshot: %w", err)
		}
	} else {
		snapshotJSON = []byte("{}")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.log_uploads
		(id, project_id, version, scenario_id, scenario_snapshot, status, storage_path, created_by_open_id,
		 ai_analysis_enabled, test_task_id, test_task_name, uploader_name, uploader_id, remark, client_request_id,
		 collector_version, timezone, client_created_at, started_at, ended_at, query_code, upload_session_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'uploading', $6, NULLIF($7, ''), $8,
			 NULLIF($9, ''), $10, $11, $12, $13, NULLIF($14, ''), $15, $16, $17, $18, $19, $20, NULLIF($21, '')::uuid )`,
		uploadID, projectID, metadata.Version, primaryScenarioID, snapshotJSON, storagePath, creatorOpenID,
		metadata.AIAnalysisEnabled, metadata.TestTaskID, metadata.TestTaskName, metadata.UploaderName, metadata.UploaderID, metadata.Remark,
		metadata.ClientRequestID, metadata.CollectorVersion, metadata.Timezone, metadata.CreatedAt, metadata.StartedAt, metadata.EndedAt, metadata.QueryCode, metadata.UploadSessionID)
	if err != nil {
		return fmt.Errorf("create upload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.parse_tasks (id, upload_id, status) VALUES ($1, $2, 'queued')`, taskID, uploadID)
	if err != nil {
		return fmt.Errorf("create parse task: %w", err)
	}
	return tx.Commit()
}

func (r *Repository) FindUploadByClientRequestID(ctx context.Context, creatorOpenID, clientRequestID string) (uploadID, taskID, queryCode, status string, fileCount int, err error) {
	err = r.db.QueryRowContext(ctx, `SELECT u.id, t.id, COALESCE(u.query_code, ''), u.status, COUNT(f.id)
		FROM logmaster_api.log_uploads u
		JOIN logmaster_api.parse_tasks t ON t.upload_id = u.id
		LEFT JOIN logmaster_api.log_files f ON f.upload_id = u.id
		WHERE u.created_by_open_id = $1 AND u.client_request_id = $2
		GROUP BY u.id, t.id`, creatorOpenID, clientRequestID).Scan(&uploadID, &taskID, &queryCode, &status, &fileCount)
	return
}

func (r *Repository) GetPublicUploadByQueryCode(ctx context.Context, queryCode string) (PublicUploadSessionStatus, error) {
	var result PublicUploadSessionStatus
	err := r.db.QueryRowContext(ctx, `SELECT s.id,s.query_code,s.project_name,s.version,s.test_task_name,s.uploader_name,s.created_at,
		COALESCE(MAX(u.updated_at),s.updated_at),COUNT(u.id),COALESCE(SUM(t.total_files),0),COALESCE(SUM(t.processed_files),0),
		COALESCE(SUM(t.total_lines),0),COALESCE(SUM(t.error_count),0),COALESCE(SUM(t.warning_count),0),
		CASE WHEN COUNT(u.id)=0 THEN 'uploading' WHEN BOOL_OR(u.status='cancelled') THEN 'cancelled' WHEN BOOL_OR(u.status='failed') THEN 'failed'
		     WHEN BOOL_AND(u.status='completed') THEN 'completed' WHEN BOOL_OR(u.status='parsing') THEN 'parsing' ELSE 'queued' END
		FROM logmaster_api.upload_sessions s
		LEFT JOIN logmaster_api.log_uploads u ON u.upload_session_id=s.id
		LEFT JOIN logmaster_api.parse_tasks t ON t.upload_id=u.id
		WHERE s.query_code = $1 OR split_part(s.query_code, '-', 2) = $1 GROUP BY s.id`, queryCode).Scan(
		&result.UploadSessionID, &result.QueryCode, &result.ProjectName, &result.Version, &result.TestTaskName, &result.UploaderName,
		&result.CreatedAt, &result.UpdatedAt, &result.BatchCount, &result.TotalFiles, &result.ProcessedFiles, &result.TotalLines,
		&result.ErrorCount, &result.WarningCount, &result.Status)
	if err != nil {
		return result, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT u.id,t.id,COALESCE(u.client_request_id,''),COALESCE(u.query_code,''),u.status,COALESCE(t.total_files,0),COALESCE(t.processed_files,0),
		COALESCE(t.total_lines,0),COALESCE(t.error_count,0),COALESCE(t.warning_count,0),u.created_at,u.updated_at
		FROM logmaster_api.log_uploads u JOIN logmaster_api.parse_tasks t ON t.upload_id=u.id
		WHERE u.upload_session_id=$1 ORDER BY u.created_at`, result.UploadSessionID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Batches = []PublicUploadBatch{}
	for rows.Next() {
		var batch PublicUploadBatch
		if err := rows.Scan(&batch.UploadID, &batch.TaskID, &batch.ClientRequestID, &batch.QueryCode, &batch.Status, &batch.TotalFiles, &batch.ProcessedFiles, &batch.TotalLines, &batch.ErrorCount, &batch.WarningCount, &batch.CreatedAt, &batch.UpdatedAt); err != nil {
			return result, err
		}
		result.Batches = append(result.Batches, batch)
	}
	return result, rows.Err()
}

func (r *Repository) UploadStoragePath(ctx context.Context, uploadID string) (string, error) {
	var storagePath string
	err := r.db.QueryRowContext(ctx, `SELECT storage_path FROM logmaster_api.log_uploads WHERE id = $1`, uploadID).Scan(&storagePath)
	return storagePath, err
}

func (r *Repository) UserStorageName(ctx context.Context, creatorOpenID string) (string, error) {
	var name string
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(name, ''), feishu_open_id)
		FROM logmaster_api.users WHERE feishu_open_id = $1`, creatorOpenID).Scan(&name)
	return name, err
}

// StandardKeyword 是面向采集端同步的标准关键字条目。
type StandardKeyword struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Keyword     string    `json:"keyword"`
	Scope       string    `json:"scope"`
	Level       string    `json:"level"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListStandardKeywords 返回管理员维护的标准关键字库，供采集端云端同步。
func (r *Repository) ListStandardKeywords(ctx context.Context) ([]StandardKeyword, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, category, keyword, scope, level, COALESCE(description, ''), updated_at
		FROM logmaster_api.parse_rules WHERE created_by_open_id IS NULL AND source = 'admin_keyword_upload'
		AND enabled = TRUE ORDER BY updated_at DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keywords := make([]StandardKeyword, 0)
	for rows.Next() {
		var item StandardKeyword
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Keyword, &item.Scope, &item.Level, &item.Description, &item.UpdatedAt); err != nil {
			return nil, err
		}
		keywords = append(keywords, item)
	}
	return keywords, rows.Err()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (r *Repository) QueueUpload(ctx context.Context, uploadID, originalName string, originalSize int64, files []LogFile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := range files {
		err = tx.QueryRowContext(ctx, `INSERT INTO logmaster_api.log_files (upload_id, relative_path, size_bytes, sha256)
			VALUES ($1, $2, $3, $4) RETURNING id`, uploadID, files[i].RelativePath, files[i].SizeBytes, files[i].SHA256).Scan(&files[i].ID)
		if err != nil {
			return fmt.Errorf("create log file: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status = 'queued', original_name = $2,
		original_size = $3, updated_at = NOW() WHERE id = $1`, uploadID, originalName, originalSize)
	if err != nil {
		return fmt.Errorf("queue upload: %w", err)
	}
	var totalBytes int64
	for _, file := range files {
		totalBytes += file.SizeBytes
	}
	_, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET total_files = $2, total_bytes = $3, updated_at = NOW() WHERE upload_id = $1`,
		uploadID, len(files), totalBytes)
	if err != nil {
		return fmt.Errorf("update parse task: %w", err)
	}
	return tx.Commit()
}

func (r *Repository) StoreUploadMetadata(ctx context.Context, uploadID, originalName string, originalSize int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'queued', original_name = $2, original_size = $3, updated_at = NOW()
		WHERE id = $1`, uploadID, originalName, originalSize)
	return err
}

func (r *Repository) MarkFailed(ctx context.Context, uploadID, message string) {
	_, _ = r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status = 'failed', error_message = $2, updated_at = NOW() WHERE id = $1`, uploadID, message)
	_, _ = r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status = 'failed', error_message = $2,
		completed_at = NOW(), updated_at = NOW() WHERE upload_id = $1`, uploadID, message)
}

func (r *Repository) FailStaleTasks(ctx context.Context, message string, staleBefore time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads u
		SET status = 'completed', updated_at = NOW()
		FROM logmaster_api.parse_tasks t
		WHERE t.upload_id = u.id
		AND u.status = 'parsing'
		AND t.status = 'completed'`)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads u
		SET status = 'failed', error_message = $1, updated_at = NOW()
		FROM logmaster_api.parse_tasks t
		WHERE t.upload_id = u.id
		AND t.status IN ('queued', 'running')
		AND t.updated_at < $2`, message, staleBefore)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks t
		SET status = 'failed', error_message = $1, completed_at = NOW(), updated_at = NOW()
		FROM logmaster_api.log_uploads u
		WHERE u.id = t.upload_id
		AND t.status IN ('queued', 'running')
		AND t.updated_at < $2`, message, staleBefore)
	return err
}

func (r *Repository) TouchTask(ctx context.Context, uploadID string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET updated_at = NOW() WHERE id = $1 AND status IN ('queued', 'parsing')`, uploadID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET updated_at = NOW() WHERE upload_id = $1 AND status IN ('queued', 'running')`, uploadID)
	return err
}

func (r *Repository) StartParsing(ctx context.Context, uploadID string) (string, []LogFile, error) {
	var taskID string
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'running', processed_bytes = 0, started_at = NOW(), updated_at = NOW()
		WHERE upload_id = $1 RETURNING id`, uploadID).Scan(&taskID)
	if err != nil {
		return "", nil, err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status = 'parsing', updated_at = NOW() WHERE id = $1`, uploadID); err != nil {
		return "", nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, relative_path, size_bytes, sha256, line_count FROM logmaster_api.log_files WHERE upload_id = $1 ORDER BY id`, uploadID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	files := make([]LogFile, 0)
	for rows.Next() {
		var file LogFile
		if err := rows.Scan(&file.ID, &file.RelativePath, &file.SizeBytes, &file.SHA256, &file.LineCount); err != nil {
			return "", nil, err
		}
		files = append(files, file)
	}
	return taskID, files, rows.Err()
}

func (r *Repository) UploadOwner(ctx context.Context, uploadID string) (string, error) {
	var ownerOpenID string
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(created_by_open_id, '')
		FROM logmaster_api.log_uploads WHERE id = $1`, uploadID).Scan(&ownerOpenID)
	return ownerOpenID, err
}

func (r *Repository) UploadAIAnalysisEnabled(ctx context.Context, uploadID string) (bool, error) {
	var enabled bool
	err := r.db.QueryRowContext(ctx, `SELECT ai_analysis_enabled FROM logmaster_api.log_uploads WHERE id = $1`, uploadID).Scan(&enabled)
	return enabled, err
}

func (r *Repository) RulesForUpload(ctx context.Context, uploadID, ownerOpenID string) ([]ParseRule, error) {
	rules, err := r.ListRules(ctx, ownerOpenID)
	if err != nil {
		return nil, err
	}
	var encodedSnapshot []byte
	if err := r.db.QueryRowContext(ctx, `SELECT scenario_snapshot FROM logmaster_api.log_uploads
		WHERE id = $1`, uploadID).Scan(&encodedSnapshot); err != nil {
		return nil, err
	}
	if len(encodedSnapshot) == 0 || string(encodedSnapshot) == "{}" {
		return allConfiguredRules(rules), nil
	}
	var uploadSnapshot uploadScenarioSnapshot
	if err := json.Unmarshal(encodedSnapshot, &uploadSnapshot); err != nil {
		return nil, fmt.Errorf("decode upload scenario snapshot: %w", err)
	}
	if len(uploadSnapshot.Scenarios) == 0 {
		var legacy scenarioSnapshot
		if err := json.Unmarshal(encodedSnapshot, &legacy); err != nil {
			return nil, fmt.Errorf("decode legacy upload scenario snapshot: %w", err)
		}
		if legacy.ID != "" || len(legacy.Checks) > 0 {
			uploadSnapshot.Scenarios = []scenarioSnapshot{legacy}
		}
	}
	disableParsingRules := true
	if uploadSnapshot.DisableParsingRules != nil {
		disableParsingRules = *uploadSnapshot.DisableParsingRules
	}
	if len(uploadSnapshot.Scenarios) == 0 {
		return allConfiguredRules(rules), nil
	}
	return rulesFromScenarios(rules, uploadSnapshot.Scenarios, disableParsingRules)
}

func resolveTaskScenarioIDs(ctx context.Context, tx *sql.Tx, testTaskID, testTaskName string) ([]string, error) {
	testTaskID = strings.TrimSpace(testTaskID)
	testTaskName = strings.TrimSpace(testTaskName)
	if testTaskID == "" && testTaskName == "" {
		return nil, nil
	}
	if testTaskID != "" {
		var scenarioID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM logmaster_api.test_scenarios WHERE id = $1`, testTaskID).Scan(&scenarioID)
		if err == nil {
			return []string{scenarioID}, nil
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrScenarioNotApplicable
		}
		return nil, fmt.Errorf("match test task scenario by id: %w", err)
	}
	if testTaskName == "" {
		return nil, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM logmaster_api.test_scenarios WHERE name = $1 ORDER BY id LIMIT 2`, testTaskName)
	if err != nil {
		return nil, fmt.Errorf("match test task scenario by name: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 1 {
		return nil, fmt.Errorf("test task name matches multiple test scenarios")
	}
	if len(ids) == 0 {
		return nil, ErrScenarioNotApplicable
	}
	return ids, nil
}

func allConfiguredRules(rules []ParseRule) []ParseRule {
	selected := make([]ParseRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled || isGenericLogLevelRule(rule) {
			continue
		}
		selected = append(selected, rule)
	}
	return selected
}

func isGenericLogLevelRule(rule ParseRule) bool {
	if rule.Source != "system" {
		return false
	}
	keyword := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(rule.Keyword), " ", ""))
	return keyword == "FATAL|ERROR" || keyword == "WARNING|WARN"
}

func rulesFromScenarios(available []ParseRule, scenarios []scenarioSnapshot, disableParsingRules bool) ([]ParseRule, error) {
	availableByID := make(map[int64]ParseRule, len(available))
	for _, rule := range available {
		availableByID[rule.ID] = rule
	}
	selected := make([]ParseRule, 0)
	seenRuleIDs := make(map[int64]struct{})
	priority := 100
	for _, scenario := range scenarios {
		for _, check := range scenario.Checks {
			if !check.Enabled {
				continue
			}
			keyword := strings.Join(check.Keywords, "|")
			if check.Source == "rule" {
				if check.RuleID == nil {
					return nil, ErrScenarioRuleUnavailable
				}
				rule, exists := availableByID[*check.RuleID]
				if !exists {
					return nil, ErrScenarioRuleUnavailable
				}
				if _, duplicate := seenRuleIDs[rule.ID]; duplicate {
					continue
				}
				seenRuleIDs[rule.ID] = struct{}{}
				rule.Enabled = true
				if keyword != "" {
					rule.Keyword = keyword
				}
				if check.Severity != "" {
					rule.Level = check.Severity
				}
				rule.Priority = priority
				selected = append(selected, rule)
				priority++
				continue
			}
			if (check.Source == "custom" || check.Source == "") && keyword != "" {
				level := check.Severity
				if level == "" {
					level = "warning"
				}
				selected = append(selected, ParseRule{
					Name:        check.Name,
					Category:    "scenario",
					Keyword:     keyword,
					Scope:       scenario.Name,
					Level:       level,
					Enabled:     true,
					Description: check.Description,
					Priority:    priority,
					Source:      "scenario",
				})
				priority++
			}
		}
	}
	if !disableParsingRules {
		for _, rule := range available {
			if !rule.Enabled {
				continue
			}
			if _, duplicate := seenRuleIDs[rule.ID]; duplicate {
				continue
			}
			seenRuleIDs[rule.ID] = struct{}{}
			selected = append(selected, rule)
		}
	}
	return selected, nil
}

func (r *Repository) UpdateParsingProgress(ctx context.Context, taskID string, processedBytes int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET processed_bytes = LEAST(total_bytes, GREATEST(processed_bytes, $2)), updated_at = NOW()
		WHERE id = $1 AND status = 'running'`, taskID, processedBytes)
	return err
}

func (r *Repository) SaveFileResults(ctx context.Context, taskID string, fileID, lineCount, errorCount, warningCount int64, results []ParseResult) error {
	encodedResults, err := encodeParseResults(results)
	if err != nil {
		return fmt.Errorf("encode parse results: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertParseResults(ctx, tx, taskID, fileID, encodedResults); err != nil {
		return fmt.Errorf("save parse results: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_files SET line_count = $2 WHERE id = $1`, fileID, lineCount); err != nil {
		return fmt.Errorf("update log file: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET processed_files = processed_files + 1,
		total_lines = total_lines + $2, error_count = error_count + $3, warning_count = warning_count + $4,
		updated_at = NOW() WHERE id = $1`, taskID, lineCount, errorCount, warningCount)
	if err != nil {
		return fmt.Errorf("update parse task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit parse results: %w", err)
	}
	return nil
}

const parseResultBatchSize = 250

type encodedParseResult struct {
	result        ParseResult
	contextLines  []byte
	relatedCauses []byte
}

func encodeParseResults(results []ParseResult) ([]encodedParseResult, error) {
	encoded := make([]encodedParseResult, len(results))
	for index, result := range results {
		contextLines, err := json.Marshal(result.ContextLines)
		if err != nil {
			return nil, err
		}
		relatedCauses, err := json.Marshal(result.RelatedCauses)
		if err != nil {
			return nil, err
		}
		encoded[index] = encodedParseResult{result: result, contextLines: contextLines, relatedCauses: relatedCauses}
	}
	return encoded, nil
}

func insertParseResults(ctx context.Context, tx *sql.Tx, taskID string, fileID int64, results []encodedParseResult) error {
	for start := 0; start < len(results); start += parseResultBatchSize {
		end := min(start+parseResultBatchSize, len(results))
		var query strings.Builder
		query.WriteString(`INSERT INTO logmaster_api.parse_results
			(task_id, log_file_id, level, matched_text, line_number, content, rule_id, rule_name,
			 category, event_time, context_start_time, context_end_time, context_lines, related_causes) VALUES `)
		args := make([]any, 0, (end-start)*14)
		for index, encoded := range results[start:end] {
			result := encoded.result
			if index > 0 {
				query.WriteByte(',')
			}
			base := len(args)
			fmt.Fprintf(&query,
				"($%d,$%d,$%d,$%d,$%d,$%d,NULLIF($%d,0),$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7,
				base+8, base+9, base+10, base+11, base+12, base+13, base+14)
			args = append(args, taskID, fileID, result.Level, result.MatchedText, result.LineNumber, result.Content,
				result.RuleID, result.RuleName, result.Category, result.EventTime, result.ContextStartTime,
				result.ContextEndTime, encoded.contextLines, encoded.relatedCauses)
		}
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CompleteParsing(ctx context.Context, uploadID string) error {
	_, err := r.db.ExecContext(ctx, `WITH completed_task AS (
		UPDATE logmaster_api.parse_tasks
		SET status = 'completed', completed_at = NOW(), updated_at = NOW()
		WHERE upload_id = $1
		RETURNING upload_id
	)
	UPDATE logmaster_api.log_uploads
	SET status = 'completed', error_message = '', updated_at = NOW()
	WHERE id IN (SELECT upload_id FROM completed_task)`, uploadID)
	return err
}

func (r *Repository) AnalysisNotification(ctx context.Context, uploadID string) (AnalysisNotification, error) {
	var notification AnalysisNotification
	err := r.db.QueryRowContext(ctx, `SELECT t.id, COALESCE(u.created_by_open_id, ''), p.name,
		u.version, u.original_name, t.total_lines, t.error_count, t.warning_count
		FROM logmaster_api.log_uploads u
		JOIN logmaster_api.projects p ON p.id = u.project_id
		JOIN logmaster_api.parse_tasks t ON t.upload_id = u.id
		WHERE u.id = $1 AND t.status = 'completed'
		  AND EXISTS (SELECT 1 FROM logmaster_api.users recipient WHERE recipient.feishu_open_id = u.created_by_open_id AND recipient.identity_type = 'feishu')`, uploadID).Scan(
		&notification.TaskID, &notification.RecipientOpenID, &notification.ProjectName,
		&notification.Version, &notification.OriginalName, &notification.TotalLines,
		&notification.ErrorCount, &notification.WarningCount)
	if err != nil {
		return AnalysisNotification{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(rule_name, ''), matched_text), COUNT(*)
		FROM logmaster_api.parse_results
		WHERE task_id = $1
		GROUP BY COALESCE(NULLIF(rule_name, ''), matched_text)
		ORDER BY COUNT(*) DESC, COALESCE(NULLIF(rule_name, ''), matched_text)
		LIMIT 5`, notification.TaskID)
	if err != nil {
		return AnalysisNotification{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var match AnalysisNotificationMatch
		if err := rows.Scan(&match.Keyword, &match.Count); err != nil {
			return AnalysisNotification{}, err
		}
		notification.TopMatches = append(notification.TopMatches, match)
	}
	return notification, rows.Err()
}

const uploadSelect = `SELECT u.id, t.id, u.project_id::text, p.name, u.version,
	COALESCE(u.test_task_id, ''), u.test_task_name, u.uploader_name, u.uploader_id, u.remark,
	COALESCE(u.client_request_id, ''), COALESCE(u.query_code, ''), COALESCE(u.upload_session_id::text, ''),
	CASE WHEN u.upload_session_id IS NULL THEN 1 ELSE (SELECT COUNT(*) FROM logmaster_api.log_uploads position
		WHERE position.upload_session_id = u.upload_session_id AND (position.created_at < u.created_at OR (position.created_at = u.created_at AND position.id <= u.id))) END,
	u.collector_version, u.timezone,
	u.client_created_at, u.started_at, u.ended_at, COALESCE(u.scenario_id, ''),
	COALESCE(u.scenario_snapshot->>'name', ''),
	CASE WHEN u.created_by_open_id = 'logmaster-internal-collector' THEN 'collector' ELSE 'uploaded' END,
	u.status, u.original_name, u.original_size,
	t.priority, t.ai_status, t.ai_error_message,
	COUNT(DISTINCT f.id), COALESCE(t.total_files, 0), COALESCE(t.processed_files, 0),
	COALESCE(t.total_bytes, 0), COALESCE(t.processed_bytes, 0),
	COALESCE(t.total_lines, 0), COALESCE(t.error_count, 0), COALESCE(t.warning_count, 0),
	u.error_message, u.created_at, u.updated_at,
	COALESCE((SELECT email FROM logmaster_api.users WHERE feishu_open_id = u.uploader_id LIMIT 1), '')
	FROM logmaster_api.log_uploads u JOIN logmaster_api.projects p ON p.id = u.project_id
	JOIN logmaster_api.parse_tasks t ON t.upload_id = u.id LEFT JOIN logmaster_api.log_files f ON f.upload_id = u.id`

func scanUpload(row interface{ Scan(...any) error }) (Upload, error) {
	var u Upload
	err := row.Scan(&u.ID, &u.TaskID, &u.ProjectID, &u.ProjectName, &u.Version,
		&u.TestTaskID, &u.TestTaskName, &u.UploaderName, &u.UploaderID, &u.Remark,
		&u.ClientRequestID, &u.QueryCode, &u.UploadSessionID, &u.UploadPosition, &u.CollectorVersion, &u.Timezone, &u.ClientCreatedAt, &u.StartedAt, &u.EndedAt,
		&u.ScenarioID, &u.ScenarioName, &u.SourceType,
		&u.Status, &u.OriginalName, &u.OriginalSize, &u.Priority, &u.AIStatus, &u.AIErrorMessage,
		&u.FileCount, &u.TotalFiles, &u.ProcessedFiles, &u.TotalBytes, &u.ProcessedBytes,
		&u.TotalLines, &u.ErrorCount, &u.WarningCount,
		&u.ErrorMessage, &u.CreatedAt, &u.UpdatedAt, &u.UploaderEmail)
	u.Progress = uploadProgress(u)
	return u, err
}

func uploadProgress(u Upload) int {
	switch u.Status {
	case "completed", "failed", "cancelled":
		return 100
	case "paused":
		if u.TotalBytes > 0 {
			return 30 + int((u.ProcessedBytes*65)/u.TotalBytes)
		}
		return 25
	case "uploading":
		return 10
	case "queued":
		return 25
	case "parsing":
		if u.TotalBytes > 0 {
			progress := 30 + int((u.ProcessedBytes*65)/u.TotalBytes)
			if progress >= 100 {
				return 95
			}
			return progress
		}
		if u.TotalFiles <= 0 {
			return 30
		}
		progress := 30 + (u.ProcessedFiles*65)/u.TotalFiles
		if progress >= 100 {
			return 95
		}
		return progress
	default:
		return 0
	}
}

func (r *Repository) ListTasks(ctx context.Context, ownerOpenID string, limit, offset int, status, aiStatus, project, version, sort string) ([]Upload, int, error) {
	where := []string{"(u.created_by_open_id = $1 OR EXISTS (SELECT 1 FROM logmaster_api.user_collected_upload_sessions access WHERE access.user_open_id = $1 AND access.upload_session_id = u.upload_session_id))"}
	args := []any{ownerOpenID}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if status != "" {
		add("u.status = $%d", status)
	}
	if aiStatus != "" {
		add("t.ai_status = $%d", aiStatus)
	}
	if project != "" {
		add("p.name = $%d", project)
	}
	if version != "" {
		add("u.version = $%d", version)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	countQuery := `SELECT COUNT(*) FROM logmaster_api.log_uploads u JOIN logmaster_api.projects p ON p.id=u.project_id
		JOIN logmaster_api.parse_tasks t ON t.upload_id=u.id WHERE ` + whereSQL
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	sortSQL := "u.created_at DESC"
	switch sort {
	case "updated_at":
		sortSQL = "u.updated_at DESC"
	case "errors":
		sortSQL = "t.error_count DESC, u.updated_at DESC"
	case "oldest":
		sortSQL = "u.created_at ASC"
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := r.db.QueryContext(ctx, uploadSelect+` WHERE `+whereSQL+`
		GROUP BY u.id, t.id, p.name ORDER BY `+sortSQL+` LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2), queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	tasks := make([]Upload, 0)
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, u)
	}
	return tasks, total, rows.Err()
}

func (r *Repository) GetUpload(ctx context.Context, id, ownerOpenID string) (Upload, []LogFile, error) {
	u, err := scanUpload(r.db.QueryRowContext(ctx, uploadSelect+` WHERE u.id = $1 AND (u.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = u.upload_session_id
		))
		GROUP BY u.id, t.id, p.name`, id, ownerOpenID))
	if err != nil {
		return Upload{}, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, relative_path, size_bytes, sha256, line_count FROM logmaster_api.log_files WHERE upload_id = $1 ORDER BY id`, id)
	if err != nil {
		return Upload{}, nil, err
	}
	defer rows.Close()
	files := make([]LogFile, 0)
	for rows.Next() {
		var f LogFile
		if err := rows.Scan(&f.ID, &f.RelativePath, &f.SizeBytes, &f.SHA256, &f.LineCount); err != nil {
			return Upload{}, nil, err
		}
		files = append(files, f)
	}
	return u, files, rows.Err()
}

func (r *Repository) GetUploadByTask(ctx context.Context, taskID, ownerOpenID string) (Upload, []LogFile, error) {
	u, err := scanUpload(r.db.QueryRowContext(ctx, uploadSelect+` WHERE t.id = $1 AND (u.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = u.upload_session_id
		))
		GROUP BY u.id, t.id, p.name`, taskID, ownerOpenID))
	if err != nil {
		return Upload{}, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, relative_path, size_bytes, sha256, line_count FROM logmaster_api.log_files WHERE upload_id = $1 ORDER BY id`, u.ID)
	if err != nil {
		return Upload{}, nil, err
	}
	defer rows.Close()
	files := make([]LogFile, 0)
	for rows.Next() {
		var f LogFile
		if err := rows.Scan(&f.ID, &f.RelativePath, &f.SizeBytes, &f.SHA256, &f.LineCount); err != nil {
			return Upload{}, nil, err
		}
		files = append(files, f)
	}
	return u, files, rows.Err()
}

// RequestTaskRetry atomically moves a failed task back to the durable queue.
// The manual retry flag intentionally bypasses the automatic lease-recovery limit,
// while preserving the monotonically increasing attempt history.
func (r *Repository) RequestTaskRetry(ctx context.Context, taskID, ownerOpenID string) (alreadyQueued bool, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var uploadID, status string
	var manualRetryRequested bool
	err = tx.QueryRowContext(ctx, `SELECT u.id, u.status, task.manual_retry_requested
		FROM logmaster_api.parse_tasks task
		JOIN logmaster_api.log_uploads u ON u.id = task.upload_id
		WHERE task.id = $1 AND (u.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = u.upload_session_id
		)) FOR UPDATE OF task, u`, taskID, ownerOpenID).Scan(&uploadID, &status, &manualRetryRequested)
	if err != nil {
		return false, err
	}
	alreadyQueued, err = taskRetryDisposition(status, manualRetryRequested)
	if err != nil || alreadyQueued {
		return alreadyQueued, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET status = 'queued', manual_retry_requested = TRUE, worker_id = '', run_token = NULL,
			lease_expires_at = NULL, heartbeat_at = NOW(), completed_at = NULL,
			error_message = '', ai_status='disabled', ai_error_message='', ai_cancel_requested=FALSE, updated_at = NOW()
		WHERE id = $1`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='cancelled',completed_at=NOW(),
		worker_id='',run_token=NULL,lease_expires_at=NULL,error_code='cancelled',error_message='规则解析任务已重新开始',updated_at=NOW()
		WHERE task_id=$1 AND status IN ('queued','running')`, taskID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_uploads
		SET status = 'queued', error_message = '', updated_at = NOW() WHERE id = $1`, uploadID); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return false, nil
}

func taskRetryDisposition(status string, manualRetryRequested bool) (bool, error) {
	if status == "queued" && manualRetryRequested {
		return true, nil
	}
	if status != "failed" {
		return false, ErrTaskNotRetryable
	}
	return false, nil
}

func (r *Repository) ListUploads(ctx context.Context, ownerOpenID, sourceType string, limit, offset int) ([]Upload, int, error) {
	sourcePredicate := uploadSourcePredicate(sourceType)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM logmaster_api.log_uploads u
		WHERE (u.created_by_open_id = $1 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $1 AND access.upload_session_id = u.upload_session_id
		))`+sourcePredicate, ownerOpenID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, uploadSelect+` WHERE (u.created_by_open_id = $1 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $1 AND access.upload_session_id = u.upload_session_id
		))`+sourcePredicate+`
		GROUP BY u.id, t.id, p.name ORDER BY u.created_at DESC LIMIT $2 OFFSET $3`, ownerOpenID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	uploads := make([]Upload, 0)
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, 0, err
		}
		uploads = append(uploads, u)
	}
	return uploads, total, rows.Err()
}

func uploadSourcePredicate(sourceType string) string {
	switch sourceType {
	case "collector":
		return ` AND u.created_by_open_id = 'logmaster-internal-collector'`
	case "uploaded":
		return ` AND u.created_by_open_id <> 'logmaster-internal-collector'`
	default:
		return ""
	}
}

const collectorSessionByQueryCodeSQL = `SELECT id FROM logmaster_api.upload_sessions
		WHERE (query_code = $1 OR split_part(query_code, '-', 2) = $1) AND created_by_open_id = $2`

func (r *Repository) LinkCollectedUploadSession(ctx context.Context, ownerOpenID, queryCode string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var sessionID string
	if err := tx.QueryRowContext(ctx, collectorSessionByQueryCodeSQL, queryCode, builtinUploadOwnerOpenID).Scan(&sessionID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO logmaster_api.user_collected_upload_sessions
		(user_open_id, upload_session_id) VALUES ($1, $2)
		ON CONFLICT (user_open_id, upload_session_id) DO UPDATE SET accessed_at = NOW()`, ownerOpenID, sessionID); err != nil {
		return 0, err
	}
	var batchCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM logmaster_api.log_uploads WHERE upload_session_id = $1`, sessionID).Scan(&batchCount); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return batchCount, nil
}

func (r *Repository) Results(ctx context.Context, taskID, ownerOpenID string, limit, offset int) ([]ParseResult, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.status, COALESCE(r.assigned_to_open_id,''), r.assigned_at, r.level, r.matched_text, r.line_number, r.content, f.relative_path,
		r.rule_id, r.rule_name, r.category, r.event_time, r.context_start_time, r.context_end_time,
		r.context_lines, r.related_causes
		FROM logmaster_api.parse_results r
		JOIN logmaster_api.log_files f ON f.id = r.log_file_id
		JOIN logmaster_api.log_uploads u ON u.id = f.upload_id
		WHERE r.task_id = $1 AND (u.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = u.upload_session_id
		))
		ORDER BY r.id LIMIT $3 OFFSET $4`, taskID, ownerOpenID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]ParseResult, 0)
	for rows.Next() {
		var result ParseResult
		var ruleID sql.NullInt64
		var assignedAt sql.NullTime
		var eventTime, contextStart, contextEnd sql.NullTime
		var contextLines, relatedCauses []byte
		if err := rows.Scan(&result.ID, &result.Status, &result.AssignedTo, &assignedAt, &result.Level, &result.MatchedText, &result.LineNumber, &result.Content, &result.FilePath,
			&ruleID, &result.RuleName, &result.Category, &eventTime, &contextStart, &contextEnd,
			&contextLines, &relatedCauses); err != nil {
			return nil, err
		}
		if ruleID.Valid {
			result.RuleID = ruleID.Int64
		}
		if assignedAt.Valid {
			result.AssignedAt = &assignedAt.Time
		}
		if eventTime.Valid {
			result.EventTime = &eventTime.Time
		}
		if contextStart.Valid {
			result.ContextStartTime = &contextStart.Time
		}
		if contextEnd.Valid {
			result.ContextEndTime = &contextEnd.Time
		}
		if len(contextLines) > 0 {
			if err := json.Unmarshal(contextLines, &result.ContextLines); err != nil {
				return nil, err
			}
		}
		if len(relatedCauses) > 0 {
			if err := json.Unmarshal(relatedCauses, &result.RelatedCauses); err != nil {
				return nil, err
			}
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

type ComparisonItem struct {
	Key         string `json:"key"`
	FilePath    string `json:"file_path"`
	RuleName    string `json:"rule_name,omitempty"`
	MatchedText string `json:"matched_text"`
	Baseline    int    `json:"baseline"`
	Current     int    `json:"current"`
}

type AnalysisComparison struct {
	New        []ComparisonItem `json:"new"`
	Resolved   []ComparisonItem `json:"resolved"`
	Persistent []ComparisonItem `json:"persistent"`
	Increased  []ComparisonItem `json:"increased"`
	Decreased  []ComparisonItem `json:"decreased"`
}

func (r *Repository) CompareTasks(ctx context.Context, baselineTaskID, currentTaskID, ownerOpenID string) (AnalysisComparison, error) {
	load := func(taskID string) (map[string]ComparisonItem, error) {
		rows, err := r.db.QueryContext(ctx, `SELECT f.relative_path, COALESCE(r.rule_name,''), r.matched_text, COUNT(*)
			FROM logmaster_api.parse_results r JOIN logmaster_api.log_files f ON f.id=r.log_file_id
			JOIN logmaster_api.log_uploads u ON u.id=f.upload_id
			WHERE r.task_id=$1 AND (u.created_by_open_id=$2 OR EXISTS (
				SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
				WHERE access.user_open_id=$2 AND access.upload_session_id=u.upload_session_id))
			GROUP BY f.relative_path, r.rule_name, r.matched_text`, taskID, ownerOpenID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make(map[string]ComparisonItem)
		for rows.Next() {
			var item ComparisonItem
			if err := rows.Scan(&item.FilePath, &item.RuleName, &item.MatchedText, &item.Current); err != nil {
				return nil, err
			}
			item.Key = item.FilePath + "\x00" + item.RuleName + "\x00" + item.MatchedText
			items[item.Key] = item
		}
		return items, rows.Err()
	}
	baseline, err := load(baselineTaskID)
	if err != nil {
		return AnalysisComparison{}, err
	}
	current, err := load(currentTaskID)
	if err != nil {
		return AnalysisComparison{}, err
	}
	comparison := AnalysisComparison{}
	for key, item := range current {
		item.Baseline = baseline[key].Current
		if item.Baseline == 0 {
			comparison.New = append(comparison.New, item)
			continue
		}
		if item.Current > item.Baseline {
			comparison.Increased = append(comparison.Increased, item)
		} else if item.Current < item.Baseline {
			comparison.Decreased = append(comparison.Decreased, item)
		} else {
			comparison.Persistent = append(comparison.Persistent, item)
		}
	}
	for key, item := range baseline {
		if _, exists := current[key]; !exists {
			item.Baseline = item.Current
			item.Current = 0
			comparison.Resolved = append(comparison.Resolved, item)
		}
	}
	return comparison, nil
}

func (r *Repository) UpdateResultAssignment(ctx context.Context, resultID int64, ownerOpenID, assigneeOpenID string) (ParseResult, error) {
	if assigneeOpenID != "" {
		var exists bool
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM logmaster_api.users WHERE feishu_open_id=$1)`, assigneeOpenID).Scan(&exists); err != nil {
			return ParseResult{}, err
		}
		if !exists {
			return ParseResult{}, ErrAssignedUserNotFound
		}
	}
	var result ParseResult
	var assignedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.parse_results result
		SET assigned_to_open_id=NULLIF($2,''), assigned_at=CASE WHEN $2='' THEN NULL ELSE NOW() END, assignment_updated_by=$3, updated_at=NOW()
		FROM logmaster_api.log_files file JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		WHERE result.id=$1 AND result.log_file_id=file.id AND (upload.created_by_open_id=$3 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$3 AND access.upload_session_id=upload.upload_session_id))
		RETURNING result.id, result.status, COALESCE(result.assigned_to_open_id,''), result.assigned_at, result.level, result.matched_text, result.line_number, result.content, file.relative_path`, resultID, assigneeOpenID, ownerOpenID).
		Scan(&result.ID, &result.Status, &result.AssignedTo, &assignedAt, &result.Level, &result.MatchedText, &result.LineNumber, &result.Content, &result.FilePath)
	if assignedAt.Valid {
		result.AssignedAt = &assignedAt.Time
	}
	return result, err
}

var validResultStatuses = map[string]bool{"pending": true, "confirmed": true, "false_positive": true, "fixed": true, "closed": true}

func (r *Repository) UpdateResultStatus(ctx context.Context, resultID int64, ownerOpenID, status string) (ParseResult, error) {
	if !validResultStatuses[status] {
		return ParseResult{}, fmt.Errorf("invalid result status")
	}
	var result ParseResult
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.parse_results r
		SET status=$2, status_updated_by=$3, status_updated_at=NOW(), updated_at=NOW()
		FROM logmaster_api.log_files f JOIN logmaster_api.log_uploads u ON u.id=f.upload_id
		WHERE r.id=$1 AND r.log_file_id=f.id AND (u.created_by_open_id=$3 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$3 AND access.upload_session_id=u.upload_session_id))
		RETURNING r.id, r.status, r.level, r.matched_text, r.line_number, r.content, f.relative_path`, resultID, status, ownerOpenID).
		Scan(&result.ID, &result.Status, &result.Level, &result.MatchedText, &result.LineNumber, &result.Content, &result.FilePath)
	return result, err
}

type ResultComment struct {
	ID        int64     `json:"id"`
	ResultID  int64     `json:"result_id"`
	Comment   string    `json:"comment"`
	DefectID  string    `json:"defect_id,omitempty"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) AddResultComment(ctx context.Context, resultID int64, ownerOpenID, comment, defectID string) (ResultComment, error) {
	var result ResultComment
	err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.parse_result_comments
		(result_id, comment, defect_id, created_by_open_id)
		SELECT $1,$2,$3,$4::text FROM logmaster_api.parse_results result
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id
		JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		WHERE result.id=$1 AND (upload.created_by_open_id=$4::text OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$4::text AND access.upload_session_id=upload.upload_session_id))
		RETURNING id, result_id, comment, defect_id, created_by_open_id, created_at`, resultID, comment, defectID, ownerOpenID).
		Scan(&result.ID, &result.ResultID, &result.Comment, &result.DefectID, &result.AuthorID, &result.CreatedAt)
	return result, err
}

func (r *Repository) ResultComments(ctx context.Context, resultID int64, ownerOpenID string) ([]ResultComment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT comment.id, comment.result_id, comment.comment, comment.defect_id,
		comment.created_by_open_id, comment.created_at
		FROM logmaster_api.parse_result_comments comment
		JOIN logmaster_api.parse_results result ON result.id=comment.result_id
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id
		JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		WHERE comment.result_id=$1 AND (upload.created_by_open_id=$2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id))
		ORDER BY comment.created_at DESC, comment.id DESC`, resultID, ownerOpenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ResultComment, 0)
	for rows.Next() {
		var item ResultComment
		if err := rows.Scan(&item.ID, &item.ResultID, &item.Comment, &item.DefectID, &item.AuthorID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type ResultAuditLog struct {
	ID        int64           `json:"id"`
	ResultID  int64           `json:"result_id"`
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id"`
	OldValue  json.RawMessage `json:"old_value"`
	NewValue  json.RawMessage `json:"new_value"`
	CreatedAt time.Time       `json:"created_at"`
}

func (r *Repository) ResultHistory(ctx context.Context, resultID int64, ownerOpenID string) ([]ResultAuditLog, error) {
	var allowed bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM logmaster_api.parse_results result
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		WHERE result.id=$1 AND (upload.created_by_open_id=$2 OR EXISTS (SELECT 1 FROM logmaster_api.user_collected_upload_sessions access WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id)))`, resultID, ownerOpenID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, sql.ErrNoRows
	}
	rows, err := r.db.QueryContext(ctx, `SELECT audit.id,audit.result_id,audit.action,audit.actor_open_id,audit.old_value,audit.new_value,audit.created_at
		FROM logmaster_api.parse_result_audit_logs audit JOIN logmaster_api.parse_results result ON result.id=audit.result_id
		JOIN logmaster_api.log_files file ON file.id=result.log_file_id JOIN logmaster_api.log_uploads upload ON upload.id=file.upload_id
		WHERE audit.result_id=$1 AND (upload.created_by_open_id=$2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access WHERE access.user_open_id=$2 AND access.upload_session_id=upload.upload_session_id))
		ORDER BY audit.created_at DESC,audit.id DESC`, resultID, ownerOpenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ResultAuditLog, 0)
	for rows.Next() {
		var item ResultAuditLog
		if err := rows.Scan(&item.ID, &item.ResultID, &item.Action, &item.ActorID, &item.OldValue, &item.NewValue, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) IsCurrentParseAttempt(ctx context.Context, taskID string, attemptNo int) (bool, error) {
	var current bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM logmaster_api.parse_tasks WHERE id = $1 AND attempt_no = $2
	)`, taskID, attemptNo).Scan(&current)
	return current, err
}

func (r *Repository) IsAgentExecutionAllowed(ctx context.Context, taskID string, attemptNo int) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM logmaster_api.parse_tasks
		WHERE id = $1 AND attempt_no = $2 AND ai_cancel_requested = FALSE
	)`, taskID, attemptNo).Scan(&allowed)
	return allowed, err
}

func (r *Repository) SaveAgentAnalysis(ctx context.Context, taskID string, attemptNo int, fileID int64, provider string, result AgentAnalysisResponse, analysisErr error) error {
	status, errorMessage := "completed", ""
	if analysisErr != nil {
		status, errorMessage = "failed", chineseErrorMessage("AI 分析失败："+analysisErr.Error())
	}
	errorCode := classifyAIError(analysisErr)
	findings, err := json.Marshal(result.Findings)
	if err != nil {
		return fmt.Errorf("marshal agent findings: %w", err)
	}
	resultSet, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.agent_analyses
		(task_id, attempt_no, log_file_id, provider, status, summary, findings, error_message, error_code)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
		FROM logmaster_api.parse_tasks task WHERE task.id = $1 AND task.attempt_no = $2 AND task.ai_cancel_requested = FALSE
		FOR KEY SHARE
		ON CONFLICT (task_id, log_file_id, provider) DO UPDATE SET
		attempt_no = EXCLUDED.attempt_no, status = EXCLUDED.status, summary = EXCLUDED.summary, findings = EXCLUDED.findings,
		error_message = EXCLUDED.error_message, error_code = EXCLUDED.error_code, updated_at = NOW()`,
		taskID, attemptNo, fileID, provider, status, result.Summary, findings, errorMessage, errorCode)
	if err != nil {
		return err
	}
	rows, err := resultSet.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrParseTaskLeaseLost
	}
	return nil
}

func (r *Repository) AgentResults(ctx context.Context, taskID, ownerOpenID string) ([]AgentAnalysisRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.task_id, a.log_file_id, f.relative_path,
		a.provider, a.status, a.summary, a.findings, a.error_message, a.error_code, a.created_at, a.updated_at
		FROM logmaster_api.agent_analyses a JOIN logmaster_api.log_files f ON f.id = a.log_file_id
		JOIN logmaster_api.parse_tasks task ON task.id = a.task_id AND task.attempt_no = a.attempt_no
		JOIN logmaster_api.log_uploads u ON u.id = f.upload_id
		WHERE a.task_id = $1 AND (u.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = u.upload_session_id
		)) ORDER BY a.id`, taskID, ownerOpenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]AgentAnalysisRecord, 0)
	for rows.Next() {
		var record AgentAnalysisRecord
		var findings []byte
		if err := rows.Scan(&record.ID, &record.TaskID, &record.LogFileID, &record.FilePath,
			&record.Provider, &record.Status, &record.Summary, &findings, &record.ErrorMessage, &record.ErrorCode,
			&record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(findings, &record.Findings); err != nil {
			return nil, err
		}
		if record.Findings == nil {
			record.Findings = []AgentFinding{}
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

type AIAnalysisSettings struct {
	LLMAPIBaseURL      string
	LLMAPIKeyEncrypted string
	LLMModel           string
	LLMTimeoutSeconds  int
	LLMMaxMatches      int
	LLMMaxInputBytes   int
	MaxTokensPerFile   int
	DailyTokenQuota    int64
}

type TaskOverviewRecord struct {
	TaskID       string
	Provider     string
	Status       string
	Summary      string
	RiskLevel    string
	Risks        []TaskOverviewRisk
	Actions      []string
	ErrorMessage string
	ErrorCode    string
	GeneratedAt  time.Time
	UpdatedAt    time.Time
}

func (r *Repository) AIAnalysisSettings(ctx context.Context, fallback AIAnalysisSettings) (AIAnalysisSettings, error) {
	settings := fallback
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(llm_timeout_seconds,0),$1),
		COALESCE(NULLIF(llm_max_matches,0),$2), COALESCE(NULLIF(llm_max_input_bytes,0),$3),
		max_tokens_per_file, daily_token_quota FROM logmaster_api.ai_analysis_config WHERE singleton = TRUE`,
		fallback.LLMTimeoutSeconds, fallback.LLMMaxMatches, fallback.LLMMaxInputBytes).
		Scan(&settings.LLMTimeoutSeconds, &settings.LLMMaxMatches, &settings.LLMMaxInputBytes,
			&settings.MaxTokensPerFile, &settings.DailyTokenQuota)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

func (r *Repository) UserDailyTokenUsage(ctx context.Context, userOpenID string) (int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		FROM logmaster_api.ai_usage WHERE user_open_id = $1 AND usage_date = CURRENT_DATE`, userOpenID).Scan(&total)
	return total, err
}

func (r *Repository) RecordAIUsage(ctx context.Context, userOpenID, taskID string, fileID int64, promptTokens, completionTokens int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.ai_usage
		(user_open_id, usage_date, prompt_tokens, completion_tokens, task_id, log_file_id)
		VALUES ($1, CURRENT_DATE, $2, $3, $4, $5)`, userOpenID, promptTokens, completionTokens, taskID, fileID)
	return err
}

func (r *Repository) RecordAITaskUsage(ctx context.Context, userOpenID, taskID string, promptTokens, completionTokens int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.ai_usage
		(user_open_id, usage_date, prompt_tokens, completion_tokens, task_id)
		VALUES ($1, CURRENT_DATE, $2, $3, $4)`, userOpenID, promptTokens, completionTokens, taskID)
	return err
}

func (r *Repository) SaveTaskOverview(ctx context.Context, taskID string, attemptNo int, provider string, overview TaskOverview, analysisErr error) error {
	status, errorMessage := "completed", ""
	if analysisErr != nil {
		status, errorMessage = "failed", chineseErrorMessage("AI 分析失败："+analysisErr.Error())
	}
	errorCode := classifyAIError(analysisErr)
	risks, err := json.Marshal(overview.Risks)
	if err != nil {
		return fmt.Errorf("marshal task overview risks: %w", err)
	}
	actions, err := json.Marshal(overview.Actions)
	if err != nil {
		return fmt.Errorf("marshal task overview actions: %w", err)
	}
	resultSet, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.task_ai_overviews
		(task_id, attempt_no, provider, status, summary, risk_level, risks, actions, error_message, error_code, generated_at)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()
		FROM logmaster_api.parse_tasks task WHERE task.id = $1 AND task.attempt_no = $2 AND task.ai_cancel_requested = FALSE
		FOR KEY SHARE
		ON CONFLICT (task_id) DO UPDATE SET provider = EXCLUDED.provider, status = EXCLUDED.status,
		attempt_no = EXCLUDED.attempt_no, summary = EXCLUDED.summary, risk_level = EXCLUDED.risk_level, risks = EXCLUDED.risks,
		actions = EXCLUDED.actions, error_message = EXCLUDED.error_message, error_code = EXCLUDED.error_code, generated_at = NOW(), updated_at = NOW()`,
		taskID, attemptNo, provider, status, overview.Summary, overview.RiskLevel, risks, actions, errorMessage, errorCode)
	if err != nil {
		return err
	}
	rows, err := resultSet.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrParseTaskLeaseLost
	}
	if _, err = r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks
		SET ai_retry_requested = FALSE, updated_at = NOW()
		WHERE id = $1 AND attempt_no = $2`, taskID, attemptNo); err != nil {
		return err
	}
	return nil
}

func (r *Repository) TaskOverview(ctx context.Context, taskID, ownerOpenID string) (TaskOverviewRecord, error) {
	var record TaskOverviewRecord
	var risks, actions []byte
	err := r.db.QueryRowContext(ctx, `SELECT overview.task_id, overview.provider, overview.status,
		overview.summary, overview.risk_level, overview.risks, overview.actions, overview.error_message, overview.error_code,
		overview.generated_at, overview.updated_at
		FROM logmaster_api.task_ai_overviews overview
		JOIN logmaster_api.parse_tasks task ON task.id = overview.task_id
		JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
		WHERE overview.task_id = $1 AND overview.attempt_no = task.attempt_no
		AND (upload.created_by_open_id = $2 OR EXISTS (
			SELECT 1 FROM logmaster_api.user_collected_upload_sessions access
			WHERE access.user_open_id = $2 AND access.upload_session_id = upload.upload_session_id
		))`, taskID, ownerOpenID).Scan(&record.TaskID, &record.Provider, &record.Status,
		&record.Summary, &record.RiskLevel, &risks, &actions, &record.ErrorMessage, &record.ErrorCode, &record.GeneratedAt, &record.UpdatedAt)
	if err != nil {
		return TaskOverviewRecord{}, err
	}
	if err := json.Unmarshal(risks, &record.Risks); err != nil {
		return TaskOverviewRecord{}, fmt.Errorf("decode task overview risks: %w", err)
	}
	if err := json.Unmarshal(actions, &record.Actions); err != nil {
		return TaskOverviewRecord{}, fmt.Errorf("decode task overview actions: %w", err)
	}
	if record.Risks == nil {
		record.Risks = []TaskOverviewRisk{}
	}
	if record.Actions == nil {
		record.Actions = []string{}
	}
	return record, nil
}

func (record TaskOverviewRecord) AsAgentAnalysisRecord() AgentAnalysisRecord {
	findings := make([]AgentFinding, 0, len(record.Risks)+1)
	for _, risk := range record.Risks {
		findings = append(findings, AgentFinding{
			Category:   "task_overview",
			Severity:   risk.Severity,
			RootCause:  risk.Title,
			Evidence:   risk.Evidence,
			Impact:     risk.Impact,
			Suggestion: risk.Suggestion,
			Confidence: risk.Confidence,
			FilePath:   strings.Join(risk.Files, ", "),
		})
	}
	if len(record.Actions) > 0 {
		findings = append(findings, AgentFinding{
			Category: "task_overview", Severity: "info", RootCause: "建议操作",
			Suggestion: strings.Join(record.Actions, "\n"),
		})
	}
	return AgentAnalysisRecord{
		TaskID: record.TaskID, FilePath: "任务级 AI 总览", Provider: record.Provider,
		Status: record.Status, Summary: record.Summary, Findings: findings,
		ErrorMessage: record.ErrorMessage, ErrorCode: record.ErrorCode, CreatedAt: record.GeneratedAt, UpdatedAt: record.UpdatedAt,
	}
}

func (r *Repository) DeleteTask(ctx context.Context, taskID, ownerOpenID string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var uploadID, storagePath string
	err = tx.QueryRowContext(ctx, `SELECT u.id, u.storage_path FROM logmaster_api.log_uploads u
		JOIN logmaster_api.parse_tasks t ON t.upload_id=u.id
		WHERE t.id=$1 AND u.created_by_open_id=$2`, taskID, ownerOpenID).Scan(&uploadID, &storagePath)
	if err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM logmaster_api.log_uploads WHERE id=$1`, uploadID)
	if err != nil {
		return "", err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return "", sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return storagePath, nil
}

func (r *Repository) Projects(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM logmaster_api.projects WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		projects = append(projects, name)
	}
	return projects, rows.Err()
}

// ListCollectorProjects returns active projects with the IDs required by upload sessions.
func (r *Repository) ListCollectorProjects(ctx context.Context) ([]CollectorProject, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id::text, name
		FROM logmaster_api.projects WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]CollectorProject, 0)
	for rows.Next() {
		var project CollectorProject
		if err := rows.Scan(&project.ID, &project.Name); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}
