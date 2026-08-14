package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrRuleInUse = errors.New("rule is referenced by a test scenario")

type ParseRule struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Keyword       string    `json:"keyword"`
	Scope         string    `json:"scope"`
	Level         string    `json:"level"`
	Enabled       bool      `json:"enabled"`
	Description   string    `json:"description"`
	Priority      int       `json:"priority"`
	Source        string    `json:"source"`
	Editable      bool      `json:"editable"`
	ScenarioCount int       `json:"scenario_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TestScenario struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Remark      string          `json:"remark"`
	Enabled     bool            `json:"enabled"`
	AllProjects bool            `json:"all_projects"`
	Projects    []string        `json:"projects"`
	Keywords    []string        `json:"keywords"`
	Color       string          `json:"color"`
	Judgement   string          `json:"judgement"`
	Metadata    json.RawMessage `json:"metadata"`
	Checks      json.RawMessage `json:"checks"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type ScenarioMetadata struct {
	Status       string   `json:"status"`
	ProjectScope string   `json:"project_scope"`
	Projects     []string `json:"projects"`
	Tags         []string `json:"tags"`
}

type ScenarioCheck struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Severity      string     `json:"severity"`
	Enabled       bool       `json:"enabled"`
	Source        string     `json:"source"`
	RuleID        *int64     `json:"rule_id,omitempty"`
	RuleName      string     `json:"rule_name,omitempty"`
	RuleUpdatedAt *time.Time `json:"rule_updated_at,omitempty"`
	MatchType     string     `json:"match_type"`
	MinCount      int        `json:"min_count"`
	TimeWindow    int        `json:"time_window"`
	Keywords      []string   `json:"keywords"`
}

func (r *Repository) ListRules(ctx context.Context, ownerOpenID string) ([]ParseRule, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.name, r.category, r.keyword, r.scope, r.level,
		COALESCE(s.enabled, r.level IN ('critical', 'warning')), r.description, r.priority, r.source,
		COALESCE(r.created_by_open_id = $1, FALSE),
		(SELECT COUNT(*) FROM logmaster_api.test_scenarios scenario WHERE EXISTS (
			SELECT 1 FROM jsonb_array_elements(scenario.checks) item WHERE item->>'rule_id' = r.id::text
		)), r.created_at, r.updated_at
		FROM logmaster_api.parse_rules r
		LEFT JOIN logmaster_api.user_rule_settings s
			ON s.rule_id = r.id AND s.feishu_open_id = $1
		WHERE r.created_by_open_id IS NULL OR r.created_by_open_id = $1
		ORDER BY r.priority, r.id`, ownerOpenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]ParseRule, 0)
	for rows.Next() {
		var rule ParseRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Category, &rule.Keyword, &rule.Scope,
			&rule.Level, &rule.Enabled, &rule.Description, &rule.Priority, &rule.Source,
			&rule.Editable, &rule.ScenarioCount, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *Repository) SaveRule(ctx context.Context, ownerOpenID string, rule ParseRule) (ParseRule, error) {
	if rule.Priority <= 0 {
		rule.Priority = 100
	}
	if rule.Source == "" {
		rule.Source = "manual"
	}
	if rule.ID == 0 {
		rule.Enabled = defaultRuleEnabled(rule.Level)
		err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.parse_rules
			(name, category, keyword, scope, level, enabled, description, priority, source, created_by_open_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING id, created_at, updated_at`, rule.Name, rule.Category, rule.Keyword, rule.Scope,
			rule.Level, rule.Enabled, rule.Description, rule.Priority, rule.Source, ownerOpenID).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
		rule.Editable = err == nil
		return rule, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ParseRule{}, err
	}
	defer tx.Rollback()
	var current ParseRule
	var createdBy sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id, name, category, keyword, scope, level, enabled,
		description, priority, source, created_at, updated_at, created_by_open_id
		FROM logmaster_api.parse_rules
		WHERE id = $1 AND (created_by_open_id IS NULL OR created_by_open_id = $2)`, rule.ID, ownerOpenID).Scan(
		&current.ID, &current.Name, &current.Category, &current.Keyword, &current.Scope, &current.Level,
		&current.Enabled, &current.Description, &current.Priority, &current.Source,
		&current.CreatedAt, &current.UpdatedAt, &createdBy)
	if err != nil {
		return ParseRule{}, err
	}
	if createdBy.Valid {
		err = tx.QueryRowContext(ctx, `UPDATE logmaster_api.parse_rules SET name=$2, category=$3, keyword=$4,
			scope=$5, level=$6, description=$7, priority=$8, source=$9, updated_at=NOW()
			WHERE id=$1 AND created_by_open_id=$10 RETURNING created_at, updated_at`,
			rule.ID, rule.Name, rule.Category, rule.Keyword, rule.Scope, rule.Level,
			rule.Description, rule.Priority, rule.Source, ownerOpenID).Scan(&rule.CreatedAt, &rule.UpdatedAt)
		if err != nil {
			return ParseRule{}, err
		}
		current = rule
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO logmaster_api.user_rule_settings (feishu_open_id, rule_id, enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (feishu_open_id, rule_id) DO UPDATE
		SET enabled = EXCLUDED.enabled, updated_at = NOW()`, ownerOpenID, rule.ID, rule.Enabled)
	if err != nil {
		return ParseRule{}, err
	}
	if err := tx.Commit(); err != nil {
		return ParseRule{}, err
	}
	current.Enabled = rule.Enabled
	return current, nil
}

func (r *Repository) SaveRuleSettingsBatch(ctx context.Context, ownerOpenID string, ids []int64, enabled bool) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	updated := 0
	for _, id := range ids {
		var visible bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM logmaster_api.parse_rules
			WHERE id = $1 AND (created_by_open_id IS NULL OR created_by_open_id = $2))`, id, ownerOpenID).Scan(&visible); err != nil {
			return 0, err
		}
		if !visible {
			return 0, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO logmaster_api.user_rule_settings (feishu_open_id, rule_id, enabled)
			VALUES ($1, $2, $3)
			ON CONFLICT (feishu_open_id, rule_id) DO UPDATE
			SET enabled = EXCLUDED.enabled, updated_at = NOW()`, ownerOpenID, id, enabled); err != nil {
			return 0, err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func defaultRuleEnabled(level string) bool {
	return level == "critical" || level == "warning"
}

func (r *Repository) DeleteRule(ctx context.Context, ownerOpenID string, id int64) error {
	var referenceCount int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM logmaster_api.test_scenarios scenario
		WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(scenario.checks) item
			WHERE item->>'rule_id' = $1::text)`, id).Scan(&referenceCount)
	if err != nil {
		return err
	}
	if referenceCount > 0 {
		return ErrRuleInUse
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM logmaster_api.parse_rules
		WHERE id=$1 AND created_by_open_id=$2`, id, ownerOpenID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *Repository) ValidateAndSnapshotScenario(ctx context.Context, ownerOpenID string, scenario TestScenario) (TestScenario, error) {
	scenario.Name = strings.TrimSpace(scenario.Name)
	if scenario.Name == "" || len([]rune(scenario.Name)) > 128 {
		return TestScenario{}, fmt.Errorf("invalid scenario name")
	}
	if len([]rune(scenario.Remark)) > 1000 || len([]rune(scenario.Description)) > 1000 {
		return TestScenario{}, fmt.Errorf("scenario remark is too long")
	}

	metadata := ScenarioMetadata{Status: "published", ProjectScope: "all", Projects: []string{}, Tags: []string{}}
	simplifiedPayload := scenario.Keywords != nil || scenario.Projects != nil || scenario.AllProjects || scenario.Remark != ""
	if simplifiedPayload {
		metadata.Status = "disabled"
		if scenario.Enabled {
			metadata.Status = "published"
		}
		metadata.ProjectScope = "selected"
		metadata.Projects = normalizeScenarioStrings(scenario.Projects)
		if scenario.AllProjects {
			metadata.ProjectScope = "all"
			metadata.Projects = []string{}
		}
		scenario.Description = strings.TrimSpace(scenario.Remark)
	} else if len(scenario.Metadata) > 0 {
		if err := json.Unmarshal(scenario.Metadata, &metadata); err != nil {
			return TestScenario{}, fmt.Errorf("invalid scenario metadata")
		}
	}
	if metadata.Status != "draft" && metadata.Status != "published" && metadata.Status != "disabled" {
		return TestScenario{}, fmt.Errorf("invalid scenario status")
	}
	if metadata.ProjectScope != "all" && metadata.ProjectScope != "selected" {
		return TestScenario{}, fmt.Errorf("invalid project scope")
	}
	if len(metadata.Tags) > 20 {
		return TestScenario{}, fmt.Errorf("too many scenario tags")
	}
	if metadata.ProjectScope == "selected" {
		if len(metadata.Projects) == 0 || len(metadata.Projects) > 100 {
			return TestScenario{}, fmt.Errorf("select at least one valid project")
		}
		for _, project := range metadata.Projects {
			var exists bool
			if err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM logmaster_api.projects
				WHERE name = $1 AND is_active = TRUE)`, project).Scan(&exists); err != nil {
				return TestScenario{}, err
			}
			if !exists {
				return TestScenario{}, fmt.Errorf("project %s does not exist", project)
			}
		}
	} else {
		metadata.Projects = []string{}
	}

	checks := make([]ScenarioCheck, 0)
	structuredChecks := len(scenario.Checks) > 0 && string(scenario.Checks) != "null" && string(scenario.Checks) != "[]"
	if structuredChecks {
		if err := json.Unmarshal(scenario.Checks, &checks); err != nil {
			return TestScenario{}, fmt.Errorf("invalid scenario checks")
		}
	} else if simplifiedPayload {
		keywords := normalizeScenarioStrings(scenario.Keywords)
		if len(keywords) == 0 || len(keywords) > 200 {
			return TestScenario{}, fmt.Errorf("add between 1 and 200 scenario keywords")
		}
		for _, keyword := range keywords {
			if len([]rune(keyword)) > 512 {
				return TestScenario{}, fmt.Errorf("scenario keyword is too long")
			}
		}
		checks = append(checks, ScenarioCheck{
			ID:          "keywords-" + scenario.ID,
			Name:        scenario.Name + "关键词筛查",
			Description: scenario.Description,
			Severity:    "critical",
			Enabled:     true,
			Source:      "custom",
			MatchType:   "forbidden",
			MinCount:    1,
			Keywords:    keywords,
		})
	}
	if len(checks) > 200 {
		return TestScenario{}, fmt.Errorf("too many scenario checks")
	}
	for index := range checks {
		check := &checks[index]
		check.Name = strings.TrimSpace(check.Name)
		if check.Severity == "" {
			check.Severity = "warning"
		}
		if check.Source == "" {
			check.Source = "custom"
		}
		if check.MatchType == "" {
			check.MatchType = "required"
		}
		if check.MinCount == 0 {
			check.MinCount = 1
		}
		if check.Name == "" || len(check.Name) > 128 {
			return TestScenario{}, fmt.Errorf("invalid check name")
		}
		if check.Severity != "critical" && check.Severity != "warning" && check.Severity != "info" {
			return TestScenario{}, fmt.Errorf("invalid check severity")
		}
		if check.MatchType != "required" && check.MatchType != "forbidden" && check.MatchType != "min-count" {
			return TestScenario{}, fmt.Errorf("invalid check expectation")
		}
		if check.MinCount < 1 || check.TimeWindow < 0 || check.TimeWindow > 86400 {
			return TestScenario{}, fmt.Errorf("invalid check threshold")
		}
		if check.Source == "rule" {
			if check.RuleID == nil {
				return TestScenario{}, fmt.Errorf("select a parsing rule for %s", check.Name)
			}
			var updatedAt time.Time
			var keyword string
			err := r.db.QueryRowContext(ctx, `SELECT r.name, r.keyword, r.updated_at
				FROM logmaster_api.parse_rules r
				WHERE r.id = $1 AND (r.created_by_open_id IS NULL OR r.created_by_open_id = $2)`,
				*check.RuleID, ownerOpenID).Scan(&check.RuleName, &keyword, &updatedAt)
			if err == sql.ErrNoRows {
				return TestScenario{}, fmt.Errorf("parsing rule for %s is unavailable", check.Name)
			}
			if err != nil {
				return TestScenario{}, err
			}
			check.Keywords = []string{keyword}
			check.RuleUpdatedAt = &updatedAt
		} else if check.Source == "custom" {
			check.RuleID = nil
			check.RuleName = ""
			check.RuleUpdatedAt = nil
			if check.Enabled && len(check.Keywords) == 0 {
				return TestScenario{}, fmt.Errorf("add a keyword for %s", check.Name)
			}
		} else {
			return TestScenario{}, fmt.Errorf("invalid check source")
		}
	}

	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return TestScenario{}, err
	}
	encodedChecks, err := json.Marshal(checks)
	if err != nil {
		return TestScenario{}, err
	}
	scenario.Metadata = encodedMetadata
	scenario.Checks = encodedChecks
	hydrateScenarioFields(&scenario)
	return scenario, nil
}

func normalizeScenarioStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func hydrateScenarioFields(scenario *TestScenario) {
	metadata := ScenarioMetadata{Status: "published", ProjectScope: "all", Projects: []string{}}
	_ = json.Unmarshal(scenario.Metadata, &metadata)
	scenario.Enabled = metadata.Status == "" || metadata.Status == "published"
	scenario.AllProjects = metadata.ProjectScope == "" || metadata.ProjectScope == "all"
	if scenario.AllProjects {
		scenario.Projects = []string{}
	} else {
		scenario.Projects = normalizeScenarioStrings(metadata.Projects)
	}
	scenario.Remark = scenario.Description

	var checks []ScenarioCheck
	_ = json.Unmarshal(scenario.Checks, &checks)
	keywords := make([]string, 0)
	for _, check := range checks {
		if check.Enabled {
			keywords = append(keywords, check.Keywords...)
		}
	}
	scenario.Keywords = normalizeScenarioStrings(keywords)
}

func (r *Repository) ListScenarios(ctx context.Context) ([]TestScenario, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, color, judgement, metadata, checks,
		created_at, updated_at FROM logmaster_api.test_scenarios ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scenarios := make([]TestScenario, 0)
	for rows.Next() {
		var scenario TestScenario
		if err := rows.Scan(&scenario.ID, &scenario.Name, &scenario.Description, &scenario.Color,
			&scenario.Judgement, &scenario.Metadata, &scenario.Checks, &scenario.CreatedAt, &scenario.UpdatedAt); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
		hydrateScenarioFields(&scenarios[len(scenarios)-1])
	}
	return scenarios, rows.Err()
}

func (r *Repository) SaveScenario(ctx context.Context, scenario TestScenario) (TestScenario, error) {
	if len(scenario.Metadata) == 0 {
		scenario.Metadata = json.RawMessage("{}")
	}
	if len(scenario.Checks) == 0 {
		scenario.Checks = json.RawMessage("[]")
	}
	err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.test_scenarios
		(id,name,description,color,judgement,metadata,checks) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description,
		color=EXCLUDED.color, judgement=EXCLUDED.judgement, metadata=EXCLUDED.metadata,
		checks=EXCLUDED.checks, updated_at=NOW()
		RETURNING created_at, updated_at`, scenario.ID, scenario.Name, scenario.Description, scenario.Color,
		scenario.Judgement, scenario.Metadata, scenario.Checks).Scan(&scenario.CreatedAt, &scenario.UpdatedAt)
	if err == nil {
		hydrateScenarioFields(&scenario)
	}
	return scenario, err
}

func (r *Repository) GetScenario(ctx context.Context, id string) (TestScenario, error) {
	var scenario TestScenario
	err := r.db.QueryRowContext(ctx, `SELECT id, name, description, color, judgement, metadata, checks,
		created_at, updated_at FROM logmaster_api.test_scenarios WHERE id = $1`, id).Scan(
		&scenario.ID, &scenario.Name, &scenario.Description, &scenario.Color, &scenario.Judgement,
		&scenario.Metadata, &scenario.Checks, &scenario.CreatedAt, &scenario.UpdatedAt)
	if err == nil {
		hydrateScenarioFields(&scenario)
	}
	return scenario, err
}

func (r *Repository) SetScenarioEnabled(ctx context.Context, id string, enabled bool) (TestScenario, error) {
	status := "disabled"
	if enabled {
		status = "published"
	}
	var scenario TestScenario
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.test_scenarios
		SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{status}', to_jsonb($2::text), true),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, description, color, judgement, metadata, checks, created_at, updated_at`, id, status).Scan(
		&scenario.ID, &scenario.Name, &scenario.Description, &scenario.Color, &scenario.Judgement,
		&scenario.Metadata, &scenario.Checks, &scenario.CreatedAt, &scenario.UpdatedAt)
	if err == nil {
		hydrateScenarioFields(&scenario)
	}
	return scenario, err
}

func (r *Repository) DeleteScenario(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM logmaster_api.test_scenarios WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return sql.ErrNoRows
	}
	return err
}

type DashboardStats struct {
	TotalLines     int64            `json:"total_lines"`
	ErrorCount     int64            `json:"error_count"`
	WarningCount   int64            `json:"warning_count"`
	TaskCount      int64            `json:"task_count"`
	CompletedCount int64            `json:"completed_count"`
	FailedCount    int64            `json:"failed_count"`
	Trend          []DashboardTrend `json:"trend"`
	TopMatches     []DashboardMatch `json:"top_matches"`
	RecentTasks    []Upload         `json:"recent_tasks"`
}

type DashboardTrend struct {
	Date     string `json:"date"`
	Lines    int64  `json:"lines"`
	Errors   int64  `json:"errors"`
	Warnings int64  `json:"warnings"`
}
type DashboardMatch struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (r *Repository) Dashboard(ctx context.Context, ownerOpenID string, days int) (DashboardStats, error) {
	var stats DashboardStats
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_lines),0), COALESCE(SUM(error_count),0),
		COALESCE(SUM(warning_count),0), COUNT(*), COUNT(*) FILTER (WHERE t.status='completed'),
		COUNT(*) FILTER (WHERE t.status='failed')
		FROM logmaster_api.parse_tasks t
		JOIN logmaster_api.log_uploads u ON u.id = t.upload_id
		WHERE u.created_by_open_id = $1`, ownerOpenID).Scan(
		&stats.TotalLines, &stats.ErrorCount, &stats.WarningCount, &stats.TaskCount, &stats.CompletedCount, &stats.FailedCount)
	if err != nil {
		return stats, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT day::date::text, COALESCE(SUM(t.total_lines),0),
		COALESCE(SUM(t.error_count),0), COALESCE(SUM(t.warning_count),0)
		FROM generate_series(CURRENT_DATE-($2::int-1), CURRENT_DATE, interval '1 day') day
		LEFT JOIN (
			SELECT t.* FROM logmaster_api.parse_tasks t
			JOIN logmaster_api.log_uploads u ON u.id = t.upload_id
			WHERE u.created_by_open_id = $1
		) t ON t.created_at::date=day::date
		GROUP BY day ORDER BY day`, ownerOpenID, days)
	if err != nil {
		return stats, err
	}
	stats.Trend = make([]DashboardTrend, 0)
	for rows.Next() {
		var item DashboardTrend
		if err := rows.Scan(&item.Date, &item.Lines, &item.Errors, &item.Warnings); err != nil {
			rows.Close()
			return stats, err
		}
		stats.Trend = append(stats.Trend, item)
	}
	rows.Close()
	rows, err = r.db.QueryContext(ctx, `SELECT r.matched_text, COUNT(*)
		FROM logmaster_api.parse_results r
		JOIN logmaster_api.log_files f ON f.id = r.log_file_id
		JOIN logmaster_api.log_uploads u ON u.id = f.upload_id
		WHERE u.created_by_open_id = $1
		GROUP BY r.matched_text ORDER BY COUNT(*) DESC LIMIT 8`, ownerOpenID)
	if err != nil {
		return stats, err
	}
	stats.TopMatches = make([]DashboardMatch, 0)
	for rows.Next() {
		var item DashboardMatch
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			rows.Close()
			return stats, err
		}
		stats.TopMatches = append(stats.TopMatches, item)
	}
	rows.Close()
	recent, _, err := r.ListUploads(ctx, ownerOpenID, "", 5, 0)
	stats.RecentTasks = recent
	return stats, err
}

func validateRule(rule ParseRule) error {
	if rule.Name == "" || rule.Keyword == "" {
		return fmt.Errorf("name and keyword are required")
	}
	if rule.Level != "critical" && rule.Level != "warning" && rule.Level != "info" {
		return fmt.Errorf("invalid rule level")
	}
	return nil
}
