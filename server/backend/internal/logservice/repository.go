package logservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct{ db *sql.DB }

var (
	ErrProjectNotFound          = errors.New("project not found")
	ErrScenarioNotApplicable    = errors.New("scenario does not exist, is not published, or does not apply to the project")
	ErrScenarioRuleUnavailable  = errors.New("scenario contains a rule unavailable to the current user")
	ErrUploaderEmailNotFound    = errors.New("uploader email is not registered")
	ErrUploaderEmailAmbiguous   = errors.New("uploader email matches multiple users")
	ErrUploaderEmailMismatch    = errors.New("uploader name does not match uploader email")
	ErrUploaderEmailNotInternal = errors.New("uploader email is not an active enterprise member")
)

func (r *Repository) UpsertCollectorIdentity(ctx context.Context, identity collectorIdentity) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO logmaster_api.users (feishu_open_id, name, email, role, role_source)
		VALUES ($1,$2,$3,'user','feishu')
		ON CONFLICT (feishu_open_id) DO UPDATE SET name=EXCLUDED.name,email=EXCLUDED.email,updated_at=NOW()`,
		identity.OpenID, identity.Name, identity.Email)
	return err
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
	}, scenarioIDs, storagePath, creatorOpenID)
}

func (r *Repository) CreateUploadWithMetadata(ctx context.Context, uploadID, taskID string, metadata UploadMetadata, scenarioIDs []string, storagePath, creatorOpenID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
		 test_task_id, test_task_name, uploader_name, uploader_id, remark, client_request_id,
		 collector_version, timezone, client_created_at, started_at, ended_at, query_code, upload_session_id)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, 'uploading', $6, NULLIF($7, ''),
			 NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''), $14, $15, $16, $17, $18, $19, NULLIF($20, '')::uuid )`,
		uploadID, projectID, metadata.Version, primaryScenarioID, snapshotJSON, storagePath, creatorOpenID,
		metadata.TestTaskID, metadata.TestTaskName, metadata.UploaderName, metadata.UploaderID, metadata.Remark,
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
		CASE WHEN COUNT(u.id)=0 THEN 'uploading' WHEN BOOL_OR(u.status='failed') THEN 'failed'
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
		return rules, nil
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
	return rulesFromScenarios(rules, uploadSnapshot.Scenarios, disableParsingRules)
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
		WHERE u.id = $1 AND t.status = 'completed'`, uploadID).Scan(
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
		&u.Status, &u.OriginalName, &u.OriginalSize,
		&u.FileCount, &u.TotalFiles, &u.ProcessedFiles, &u.TotalBytes, &u.ProcessedBytes,
		&u.TotalLines, &u.ErrorCount, &u.WarningCount,
		&u.ErrorMessage, &u.CreatedAt, &u.UpdatedAt, &u.UploaderEmail)
	u.Progress = uploadProgress(u)
	return u, err
}

func uploadProgress(u Upload) int {
	switch u.Status {
	case "completed":
		return 100
	case "failed":
		return 100
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

func (r *Repository) ListTasks(ctx context.Context, ownerOpenID string, limit, offset int) ([]Upload, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM logmaster_api.log_uploads WHERE created_by_open_id = $1`, ownerOpenID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, uploadSelect+` WHERE u.created_by_open_id = $1
		GROUP BY u.id, t.id, p.name ORDER BY u.created_at DESC LIMIT $2 OFFSET $3`, ownerOpenID, limit, offset)
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
	rows, err := r.db.QueryContext(ctx, `SELECT r.level, r.matched_text, r.line_number, r.content, f.relative_path,
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
		var eventTime, contextStart, contextEnd sql.NullTime
		var contextLines, relatedCauses []byte
		if err := rows.Scan(&result.Level, &result.MatchedText, &result.LineNumber, &result.Content, &result.FilePath,
			&ruleID, &result.RuleName, &result.Category, &eventTime, &contextStart, &contextEnd,
			&contextLines, &relatedCauses); err != nil {
			return nil, err
		}
		if ruleID.Valid {
			result.RuleID = ruleID.Int64
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

func (r *Repository) AgentResults(ctx context.Context, taskID, ownerOpenID string) ([]AgentAnalysisRecord, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.task_id, a.log_file_id, f.relative_path,
		a.provider, a.status, a.summary, a.findings, a.error_message, a.created_at, a.updated_at
		FROM logmaster_api.agent_analyses a JOIN logmaster_api.log_files f ON f.id = a.log_file_id
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

func (r *Repository) AIAnalysisSettings(ctx context.Context, fallback AIAnalysisSettings) (AIAnalysisSettings, error) {
	settings := fallback
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(llm_api_base_url,''),$1), llm_api_key_encrypted,
		COALESCE(NULLIF(llm_model,''),$2), COALESCE(NULLIF(llm_timeout_seconds,0),$3),
		COALESCE(NULLIF(llm_max_matches,0),$4), COALESCE(NULLIF(llm_max_input_bytes,0),$5),
		max_tokens_per_file, daily_token_quota FROM logmaster_api.ai_analysis_config WHERE singleton = TRUE`,
		fallback.LLMAPIBaseURL, fallback.LLMModel, fallback.LLMTimeoutSeconds, fallback.LLMMaxMatches, fallback.LLMMaxInputBytes).
		Scan(&settings.LLMAPIBaseURL, &settings.LLMAPIKeyEncrypted, &settings.LLMModel, &settings.LLMTimeoutSeconds,
			&settings.LLMMaxMatches, &settings.LLMMaxInputBytes, &settings.MaxTokensPerFile, &settings.DailyTokenQuota)
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
