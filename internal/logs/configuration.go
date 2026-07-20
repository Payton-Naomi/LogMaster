package logs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ParseRule struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Keyword     string    `json:"keyword"`
	Scope       string    `json:"scope"`
	Level       string    `json:"level"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TestScenario struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Color       string          `json:"color"`
	Judgement   string          `json:"judgement"`
	Checks      json.RawMessage `json:"checks"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (r *Repository) ListRules(ctx context.Context) ([]ParseRule, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, category, keyword, scope, level, enabled,
		description, created_at, updated_at FROM logmaster_api.parse_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]ParseRule, 0)
	for rows.Next() {
		var rule ParseRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Category, &rule.Keyword, &rule.Scope,
			&rule.Level, &rule.Enabled, &rule.Description, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *Repository) SaveRule(ctx context.Context, rule ParseRule) (ParseRule, error) {
	if rule.ID == 0 {
		err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.parse_rules
			(name, category, keyword, scope, level, enabled, description) VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id, created_at, updated_at`, rule.Name, rule.Category, rule.Keyword, rule.Scope,
			rule.Level, rule.Enabled, rule.Description).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
		return rule, err
	}
	err := r.db.QueryRowContext(ctx, `UPDATE logmaster_api.parse_rules SET name=$2, category=$3, keyword=$4,
		scope=$5, level=$6, enabled=$7, description=$8, updated_at=NOW() WHERE id=$1
		RETURNING created_at, updated_at`, rule.ID, rule.Name, rule.Category, rule.Keyword, rule.Scope,
		rule.Level, rule.Enabled, rule.Description).Scan(&rule.CreatedAt, &rule.UpdatedAt)
	return rule, err
}

func (r *Repository) DeleteRule(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM logmaster_api.parse_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *Repository) ListScenarios(ctx context.Context) ([]TestScenario, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, color, judgement, checks,
		created_at, updated_at FROM logmaster_api.test_scenarios ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scenarios := make([]TestScenario, 0)
	for rows.Next() {
		var scenario TestScenario
		if err := rows.Scan(&scenario.ID, &scenario.Name, &scenario.Description, &scenario.Color,
			&scenario.Judgement, &scenario.Checks, &scenario.CreatedAt, &scenario.UpdatedAt); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scenario)
	}
	return scenarios, rows.Err()
}

func (r *Repository) SaveScenario(ctx context.Context, scenario TestScenario) (TestScenario, error) {
	if len(scenario.Checks) == 0 {
		scenario.Checks = json.RawMessage("[]")
	}
	err := r.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.test_scenarios
		(id,name,description,color,judgement,checks) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description,
		color=EXCLUDED.color, judgement=EXCLUDED.judgement, checks=EXCLUDED.checks, updated_at=NOW()
		RETURNING created_at, updated_at`, scenario.ID, scenario.Name, scenario.Description, scenario.Color,
		scenario.Judgement, scenario.Checks).Scan(&scenario.CreatedAt, &scenario.UpdatedAt)
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

func (r *Repository) Dashboard(ctx context.Context, days int) (DashboardStats, error) {
	var stats DashboardStats
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_lines),0), COALESCE(SUM(error_count),0),
		COALESCE(SUM(warning_count),0), COUNT(*), COUNT(*) FILTER (WHERE status='completed'),
		COUNT(*) FILTER (WHERE status='failed') FROM logmaster_api.parse_tasks`).Scan(
		&stats.TotalLines, &stats.ErrorCount, &stats.WarningCount, &stats.TaskCount, &stats.CompletedCount, &stats.FailedCount)
	if err != nil {
		return stats, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT day::date::text, COALESCE(SUM(t.total_lines),0),
		COALESCE(SUM(t.error_count),0), COALESCE(SUM(t.warning_count),0)
		FROM generate_series(CURRENT_DATE-($1::int-1), CURRENT_DATE, interval '1 day') day
		LEFT JOIN logmaster_api.parse_tasks t ON t.created_at::date=day::date
		GROUP BY day ORDER BY day`, days)
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
	rows, err = r.db.QueryContext(ctx, `SELECT matched_text, COUNT(*) FROM logmaster_api.parse_results GROUP BY matched_text ORDER BY COUNT(*) DESC LIMIT 8`)
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
	recent, _, err := r.ListUploads(ctx, 5, 0)
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
