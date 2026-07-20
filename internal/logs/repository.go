package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Repository struct{ db *sql.DB }

type Upload struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	ProjectName  string    `json:"project_name"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	OriginalName string    `json:"original_name"`
	OriginalSize int64     `json:"original_size"`
	FileCount    int       `json:"file_count"`
	TotalLines   int64     `json:"total_lines"`
	ErrorCount   int64     `json:"error_count"`
	WarningCount int64     `json:"warning_count"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LogFile struct {
	ID           int64  `json:"id"`
	RelativePath string `json:"relative_path"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	LineCount    int64  `json:"line_count"`
}

type ParseResult struct {
	Level       string `json:"level"`
	MatchedText string `json:"matched_text"`
	LineNumber  int64  `json:"line_number"`
	Content     string `json:"content"`
	FilePath    string `json:"file_path"`
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateUpload(ctx context.Context, uploadID, taskID, projectName, version, storagePath string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var projectID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO logmaster_api.projects (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, projectName).Scan(&projectID)
	if err != nil {
		return fmt.Errorf("upsert project: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.log_uploads
		(id, project_id, version, status, storage_path) VALUES ($1, $2, $3, 'uploading', $4)`,
		uploadID, projectID, version, storagePath)
	if err != nil {
		return fmt.Errorf("create upload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.parse_tasks (id, upload_id, status) VALUES ($1, $2, 'queued')`, taskID, uploadID)
	if err != nil {
		return fmt.Errorf("create parse task: %w", err)
	}
	return tx.Commit()
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
	_, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET total_files = $2, updated_at = NOW() WHERE upload_id = $1`, uploadID, len(files))
	if err != nil {
		return fmt.Errorf("update parse task: %w", err)
	}
	return tx.Commit()
}

func (r *Repository) MarkFailed(ctx context.Context, uploadID, message string) {
	_, _ = r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status = 'failed', error_message = $2, updated_at = NOW() WHERE id = $1`, uploadID, message)
	_, _ = r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status = 'failed', error_message = $2,
		completed_at = NOW(), updated_at = NOW() WHERE upload_id = $1`, uploadID, message)
}

func (r *Repository) StartParsing(ctx context.Context, uploadID string) (string, []LogFile, error) {
	var taskID string
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.parse_tasks SET status = 'running', started_at = NOW(), updated_at = NOW()
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

func (r *Repository) SaveFileResults(ctx context.Context, taskID string, fileID, lineCount, errorCount, warningCount int64, results []ParseResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, result := range results {
		_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.parse_results
			(task_id, log_file_id, level, matched_text, line_number, content) VALUES ($1, $2, $3, $4, $5, $6)`,
			taskID, fileID, result.Level, result.MatchedText, result.LineNumber, result.Content)
		if err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE logmaster_api.log_files SET line_count = $2 WHERE id = $1`, fileID, lineCount); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET processed_files = processed_files + 1,
		total_lines = total_lines + $2, error_count = error_count + $3, warning_count = warning_count + $4,
		updated_at = NOW() WHERE id = $1`, taskID, lineCount, errorCount, warningCount)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CompleteParsing(ctx context.Context, uploadID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status = 'completed', completed_at = NOW(), updated_at = NOW() WHERE upload_id = $1`, uploadID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status = 'completed', updated_at = NOW() WHERE id = $1`, uploadID)
	return err
}

const uploadSelect = `SELECT u.id, t.id, p.name, u.version, u.status, u.original_name, u.original_size,
	COUNT(DISTINCT f.id), COALESCE(t.total_lines, 0), COALESCE(t.error_count, 0), COALESCE(t.warning_count, 0),
	u.error_message, u.created_at, u.updated_at
	FROM logmaster_api.log_uploads u JOIN logmaster_api.projects p ON p.id = u.project_id
	JOIN logmaster_api.parse_tasks t ON t.upload_id = u.id LEFT JOIN logmaster_api.log_files f ON f.upload_id = u.id`

func scanUpload(row interface{ Scan(...any) error }) (Upload, error) {
	var u Upload
	err := row.Scan(&u.ID, &u.TaskID, &u.ProjectName, &u.Version, &u.Status, &u.OriginalName, &u.OriginalSize,
		&u.FileCount, &u.TotalLines, &u.ErrorCount, &u.WarningCount, &u.ErrorMessage, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *Repository) GetUpload(ctx context.Context, id string) (Upload, []LogFile, error) {
	u, err := scanUpload(r.db.QueryRowContext(ctx, uploadSelect+` WHERE u.id = $1
		GROUP BY u.id, t.id, p.name`, id))
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

func (r *Repository) GetUploadByTask(ctx context.Context, taskID string) (Upload, []LogFile, error) {
	u, err := scanUpload(r.db.QueryRowContext(ctx, uploadSelect+` WHERE t.id = $1
		GROUP BY u.id, t.id, p.name`, taskID))
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

func (r *Repository) ListUploads(ctx context.Context, limit, offset int) ([]Upload, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM logmaster_api.log_uploads`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, uploadSelect+` GROUP BY u.id, t.id, p.name ORDER BY u.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
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

func (r *Repository) Results(ctx context.Context, taskID string, limit, offset int) ([]ParseResult, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.level, r.matched_text, r.line_number, r.content, f.relative_path
		FROM logmaster_api.parse_results r JOIN logmaster_api.log_files f ON f.id = r.log_file_id WHERE r.task_id = $1
		ORDER BY r.id LIMIT $2 OFFSET $3`, taskID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]ParseResult, 0)
	for rows.Next() {
		var result ParseResult
		if err := rows.Scan(&result.Level, &result.MatchedText, &result.LineNumber, &result.Content, &result.FilePath); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (r *Repository) SaveAgentAnalysis(ctx context.Context, taskID string, fileID int64, provider string, result AgentAnalysisResponse, analysisErr error) error {
	status, errorMessage := "completed", ""
	if analysisErr != nil {
		status, errorMessage = "failed", analysisErr.Error()
	}
	findings, err := json.Marshal(result.Findings)
	if err != nil {
		return fmt.Errorf("marshal agent findings: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO logmaster_api.agent_analyses
		(task_id, log_file_id, provider, status, summary, findings, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (task_id, log_file_id, provider) DO UPDATE SET
		status = EXCLUDED.status, summary = EXCLUDED.summary, findings = EXCLUDED.findings,
		error_message = EXCLUDED.error_message, updated_at = NOW()`,
		taskID, fileID, provider, status, result.Summary, findings, errorMessage)
	return err
}

func (r *Repository) AgentResults(ctx context.Context, taskID string) ([]AgentAnalysisRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.task_id, a.log_file_id, f.relative_path,
		a.provider, a.status, a.summary, a.findings, a.error_message, a.created_at, a.updated_at
		FROM logmaster_api.agent_analyses a JOIN logmaster_api.log_files f ON f.id = a.log_file_id
		WHERE a.task_id = $1 ORDER BY a.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]AgentAnalysisRecord, 0)
	for rows.Next() {
		var record AgentAnalysisRecord
		var findings []byte
		if err := rows.Scan(&record.ID, &record.TaskID, &record.LogFileID, &record.FilePath,
			&record.Provider, &record.Status, &record.Summary, &findings, &record.ErrorMessage,
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

func (r *Repository) DeleteTask(ctx context.Context, taskID string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var uploadID, storagePath string
	var projectID int64
	err = tx.QueryRowContext(ctx, `SELECT u.id, u.storage_path, u.project_id FROM logmaster_api.log_uploads u
		JOIN logmaster_api.parse_tasks t ON t.upload_id=u.id WHERE t.id=$1`, taskID).Scan(&uploadID, &storagePath, &projectID)
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM logmaster_api.projects p WHERE p.id=$1
		AND NOT EXISTS (SELECT 1 FROM logmaster_api.log_uploads u WHERE u.project_id=p.id)`, projectID); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return storagePath, nil
}

func (r *Repository) Projects(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM logmaster_api.projects ORDER BY name`)
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
