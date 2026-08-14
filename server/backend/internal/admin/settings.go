package admin

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"logmaster-agent/internal/response"
)

const (
	minUploadCapacity = int64(1 << 20)
	maxUploadCapacity = int64(100 << 30)
	maxKeywordFile    = int64(2 << 20)
	maxKeywordRules   = 1000
)

type uploadCapacity struct {
	MaxUploadBytes    int64      `json:"max_upload_bytes"`
	MaxFilesPerUpload int        `json:"max_files_per_upload"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty"`
}

type keywordRule struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Keyword     string    `json:"keyword"`
	Scope       string    `json:"scope"`
	Level       string    `json:"level"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type keywordImportResult struct {
	Created int           `json:"created"`
	Updated int           `json:"updated"`
	Skipped int           `json:"skipped"`
	Rules   []keywordRule `json:"rules"`
}

func (s *Service) uploadCapacityHandler(w http.ResponseWriter, r *http.Request) {
	openID, ok := s.requirePermission(w, r, permissionCapacity)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		capacity, err := s.loadUploadCapacity(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query upload capacity failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: capacity})
	case http.MethodPut:
		var input uploadCapacity
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid upload capacity")
			return
		}
		if input.MaxUploadBytes < minUploadCapacity || input.MaxUploadBytes > maxUploadCapacity || input.MaxFilesPerUpload < 1 || input.MaxFilesPerUpload > 500 {
			writeError(w, http.StatusBadRequest, "capacity must be 1 MB to 100 GB and file count must be 1 to 500")
			return
		}
		var updatedAt time.Time
		err := s.db.QueryRowContext(r.Context(), `INSERT INTO logmaster_api.upload_capacity_config
			(singleton, max_upload_bytes, max_files_per_upload, updated_by_open_id, updated_at)
			VALUES (TRUE, $1, $2, $3, NOW())
			ON CONFLICT (singleton) DO UPDATE SET max_upload_bytes = EXCLUDED.max_upload_bytes,
			max_files_per_upload = EXCLUDED.max_files_per_upload, updated_by_open_id = EXCLUDED.updated_by_open_id, updated_at = NOW()
			RETURNING updated_at`, input.MaxUploadBytes, input.MaxFilesPerUpload, openID).Scan(&updatedAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "save upload capacity failed")
			return
		}
		input.UpdatedAt = &updatedAt
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: input})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) loadUploadCapacity(ctx context.Context) (uploadCapacity, error) {
	capacity := uploadCapacity{MaxUploadBytes: s.config.MaxUploadBytes, MaxFilesPerUpload: s.config.MaxFilesPerUpload}
	err := s.db.QueryRowContext(ctx, `SELECT max_upload_bytes, max_files_per_upload, updated_at
		FROM logmaster_api.upload_capacity_config WHERE singleton = TRUE`).
		Scan(&capacity.MaxUploadBytes, &capacity.MaxFilesPerUpload, &capacity.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return capacity, nil
	}
	return capacity, err
}

func (s *Service) keywordRulesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionKeywords); !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rules, err := s.listKeywordRules(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query keyword rules failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: rules})
}

func (s *Service) keywordRuleHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionKeywords); !ok {
		return
	}
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/admin/keyword-rules/")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	var inUse bool
	if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (
		SELECT 1 FROM logmaster_api.test_scenarios scenario WHERE EXISTS (
			SELECT 1 FROM jsonb_array_elements(scenario.checks) item WHERE item->>'rule_id' = $1::text
		)
	)`, id).Scan(&inUse); err != nil {
		writeError(w, http.StatusInternalServerError, "check keyword rule usage failed")
		return
	}
	if inUse {
		writeError(w, http.StatusConflict, "该关键词规则正在被测试场景使用，暂不能删除")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM logmaster_api.parse_rules
		WHERE id = $1 AND created_by_open_id IS NULL AND source = 'admin_keyword_upload'`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete keyword rule failed")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "keyword rule not found")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
}

func (s *Service) keywordRulesImportHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionKeywords); !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxKeywordFile+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "关键词文件无效或超过 2 MB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择 TXT 或 CSV 关键词文件")
		return
	}
	defer file.Close()
	if header.Size > maxKeywordFile {
		writeError(w, http.StatusBadRequest, "关键词文件不能超过 2 MB")
		return
	}
	defaults, err := keywordDefaultsFromForm(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rules, skipped, err := parseKeywordFile(file, header, defaults)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.saveKeywordRules(r, rules, skipped)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save keyword rules failed")
		return
	}
	response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "success", Data: result})
}

type keywordDefaults struct {
	Category    string
	Level       string
	Scope       string
	Description string
}

func keywordDefaultsFromForm(r *http.Request) (keywordDefaults, error) {
	defaults := keywordDefaults{
		Category:    strings.TrimSpace(r.FormValue("category")),
		Level:       strings.TrimSpace(r.FormValue("level")),
		Scope:       strings.TrimSpace(r.FormValue("scope")),
		Description: strings.TrimSpace(r.FormValue("description")),
	}
	if defaults.Category == "" {
		defaults.Category = "system"
	}
	if defaults.Level == "" {
		defaults.Level = "critical"
	}
	if defaults.Scope == "" {
		defaults.Scope = "全局"
	}
	if !validKeywordCategory(defaults.Category) || !validKeywordLevel(defaults.Level) {
		return keywordDefaults{}, errors.New("关键词分类或级别无效")
	}
	if utf8.RuneCountInString(defaults.Scope) > 128 || utf8.RuneCountInString(defaults.Description) > 1000 {
		return keywordDefaults{}, errors.New("适用范围或说明过长")
	}
	return defaults, nil
}

func parseKeywordFile(file multipart.File, header *multipart.FileHeader, defaults keywordDefaults) ([]keywordRule, int, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".csv" {
		return nil, 0, errors.New("仅支持 TXT 和 CSV 文件")
	}
	if ext == ".csv" {
		return parseKeywordCSV(file, defaults)
	}
	return parseKeywordTXT(file, defaults)
}

func parseKeywordTXT(reader io.Reader, defaults keywordDefaults) ([]keywordRule, int, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 512<<10)
	rules, skipped := make([]keywordRule, 0), 0
	seen := make(map[string]struct{})
	for scanner.Scan() {
		keyword := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if keyword == "" || strings.HasPrefix(keyword, "#") {
			skipped++
			continue
		}
		rule := keywordRule{Name: keywordRuleName(keyword), Keyword: keyword, Category: defaults.Category, Level: defaults.Level, Scope: defaults.Scope, Description: defaults.Description}
		if !appendKeywordRule(&rules, seen, rule) {
			skipped++
		}
		if len(rules) > maxKeywordRules {
			return nil, skipped, errors.New("单次最多导入 1000 条关键词")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, errors.New("读取 TXT 文件失败")
	}
	if len(rules) == 0 {
		return nil, skipped, errors.New("文件中没有可导入的关键词")
	}
	return rules, skipped, nil
}

func parseKeywordCSV(reader io.Reader, defaults keywordDefaults) ([]keywordRule, int, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true
	headers, err := csvReader.Read()
	if err != nil {
		return nil, 0, errors.New("CSV 文件缺少表头")
	}
	indexes := make(map[string]int)
	for index, value := range headers {
		indexes[normalizeKeywordHeader(value)] = index
	}
	keywordIndex, ok := indexes["keyword"]
	if !ok {
		return nil, 0, errors.New("CSV 必须包含 keyword 或 关键词 列")
	}
	rules, skipped := make([]keywordRule, 0), 0
	seen := make(map[string]struct{})
	for {
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, skipped, errors.New("CSV 内容格式不正确")
		}
		keyword := csvValue(record, keywordIndex)
		if keyword == "" {
			skipped++
			continue
		}
		rule := keywordRule{
			Name: firstCSVValue(record, indexes, "name", keywordRuleName(keyword)), Keyword: keyword,
			Category: firstCSVValue(record, indexes, "category", defaults.Category), Level: firstCSVValue(record, indexes, "level", defaults.Level),
			Scope: firstCSVValue(record, indexes, "scope", defaults.Scope), Description: firstCSVValue(record, indexes, "description", defaults.Description),
		}
		if !validKeywordCategory(rule.Category) || !validKeywordLevel(rule.Level) || !appendKeywordRule(&rules, seen, rule) {
			skipped++
		}
		if len(rules) > maxKeywordRules {
			return nil, skipped, errors.New("单次最多导入 1000 条关键词")
		}
	}
	if len(rules) == 0 {
		return nil, skipped, errors.New("文件中没有可导入的关键词")
	}
	return rules, skipped, nil
}

func appendKeywordRule(rules *[]keywordRule, seen map[string]struct{}, rule keywordRule) bool {
	rule.Name, rule.Keyword = strings.TrimSpace(rule.Name), strings.TrimSpace(rule.Keyword)
	rule.Category, rule.Level = strings.ToLower(strings.TrimSpace(rule.Category)), strings.ToLower(strings.TrimSpace(rule.Level))
	rule.Scope, rule.Description = strings.TrimSpace(rule.Scope), strings.TrimSpace(rule.Description)
	if rule.Name == "" || rule.Keyword == "" || utf8.RuneCountInString(rule.Name) > 128 || utf8.RuneCountInString(rule.Keyword) > 500 || utf8.RuneCountInString(rule.Scope) > 128 || utf8.RuneCountInString(rule.Description) > 1000 {
		return false
	}
	key := strings.ToLower(rule.Keyword) + "\x00" + rule.Category
	if _, exists := seen[key]; exists {
		return false
	}
	seen[key] = struct{}{}
	*rules = append(*rules, rule)
	return true
}

func (s *Service) saveKeywordRules(r *http.Request, rules []keywordRule, skipped int) (keywordImportResult, error) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return keywordImportResult{}, err
	}
	defer tx.Rollback()
	result := keywordImportResult{Skipped: skipped, Rules: make([]keywordRule, 0, len(rules))}
	for _, rule := range rules {
		var existingID int64
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM logmaster_api.parse_rules
			WHERE created_by_open_id IS NULL AND source = 'admin_keyword_upload' AND LOWER(keyword) = LOWER($1) AND category = $2 LIMIT 1`, rule.Keyword, rule.Category).Scan(&existingID)
		switch {
		case err == nil:
			err = tx.QueryRowContext(r.Context(), `UPDATE logmaster_api.parse_rules SET name = $2, scope = $3, level = $4,
				description = $5, enabled = TRUE, updated_at = NOW() WHERE id = $1
				RETURNING id, name, category, keyword, scope, level, COALESCE(description, ''), updated_at`,
				existingID, rule.Name, rule.Scope, rule.Level, rule.Description).Scan(&rule.ID, &rule.Name, &rule.Category, &rule.Keyword, &rule.Scope, &rule.Level, &rule.Description, &rule.UpdatedAt)
			result.Updated++
		case errors.Is(err, sql.ErrNoRows):
			err = tx.QueryRowContext(r.Context(), `INSERT INTO logmaster_api.parse_rules
				(name, category, keyword, scope, level, enabled, description, priority, source, created_by_open_id)
				VALUES ($1, $2, $3, $4, $5, TRUE, $6, 200, 'admin_keyword_upload', NULL)
				RETURNING id, name, category, keyword, scope, level, COALESCE(description, ''), updated_at`,
				rule.Name, rule.Category, rule.Keyword, rule.Scope, rule.Level, rule.Description).Scan(&rule.ID, &rule.Name, &rule.Category, &rule.Keyword, &rule.Scope, &rule.Level, &rule.Description, &rule.UpdatedAt)
			result.Created++
		}
		if err != nil {
			return keywordImportResult{}, err
		}
		result.Rules = append(result.Rules, rule)
	}
	if err := tx.Commit(); err != nil {
		return keywordImportResult{}, err
	}
	return result, nil
}

func (s *Service) listKeywordRules(r *http.Request) ([]keywordRule, error) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id, name, category, keyword, scope, level, COALESCE(description, ''), updated_at
		FROM logmaster_api.parse_rules WHERE created_by_open_id IS NULL AND source = 'admin_keyword_upload' ORDER BY updated_at DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]keywordRule, 0)
	for rows.Next() {
		var rule keywordRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Category, &rule.Keyword, &rule.Scope, &rule.Level, &rule.Description, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func normalizeKeywordHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "\ufeff")))
	if normalized, ok := map[string]string{"名称": "name", "规则名称": "name", "关键词": "keyword", "关键字": "keyword", "分类": "category", "级别": "level", "等级": "level", "范围": "scope", "适用范围": "scope", "说明": "description", "描述": "description"}[value]; ok {
		return normalized
	}
	return value
}

func csvValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func firstCSVValue(record []string, indexes map[string]int, key, fallback string) string {
	if index, ok := indexes[key]; ok {
		if value := csvValue(record, index); value != "" {
			return value
		}
	}
	return fallback
}

func keywordRuleName(keyword string) string {
	name := "研发异常：" + keyword
	runes := []rune(name)
	if len(runes) > 128 {
		return string(runes[:128])
	}
	return name
}

func validKeywordCategory(value string) bool {
	return map[string]bool{"power": true, "storage": true, "recording": true, "system": true, "connectivity": true, "feature": true, "tool": true}[strings.ToLower(value)]
}

func validKeywordLevel(value string) bool {
	return value == "critical" || value == "warning" || value == "info"
}
