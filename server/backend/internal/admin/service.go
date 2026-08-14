package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"logmaster-agent/internal/config"
	"logmaster-agent/internal/response"
)

var projectNamePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,127}$`)

type Service struct {
	config              config.Config
	db                  *sql.DB
	currentUserResolver func(*http.Request) (string, bool)
	roleResolver        func(context.Context, string) (string, error)
}

const (
	roleUser       = "user"
	roleDeveloper  = "developer"
	roleAdmin      = "admin"
	roleSuperAdmin = "super_admin"

	permissionProjects  = "projects"
	permissionUsers     = "users"
	permissionCapacity  = "capacity"
	permissionApprovals = "approvals"
	permissionKeywords  = "keywords"
)

type Project struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ProductLine string    `json:"product_line"`
	ProductType string    `json:"product_type"`
	Stage       string    `json:"stage"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectOption struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

type projectOptionInput struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func NewService(cfg config.Config, db *sql.DB) *Service {
	return &Service{config: cfg, db: db}
}

func (s *Service) SetCurrentUserResolver(resolver func(*http.Request) (string, bool)) {
	s.currentUserResolver = resolver
}

func (s *Service) SetUserRoleResolver(resolver func(context.Context, string) (string, error)) {
	s.roleResolver = resolver
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/users", s.usersHandler)
	mux.HandleFunc("/api/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/restore-feishu-role") {
			s.restoreUserRoleHandler(w, r)
			return
		}
		s.userRoleHandler(w, r)
	})
	mux.HandleFunc("/api/admin/permission-requests", s.permissionRequestsHandler)
	mux.HandleFunc("/api/admin/permission-requests/", s.permissionRequestHandler)
	mux.HandleFunc("/api/admin/runtime-logs", s.runtimeLogsHandler)
	mux.HandleFunc("/api/admin/project-requests", s.projectRequestsHandler)
	mux.HandleFunc("/api/admin/project-requests/", s.projectRequestHandler)
	mux.HandleFunc("/api/admin/upload-capacity", s.uploadCapacityHandler)
	mux.HandleFunc("/api/admin/keyword-rules/import", s.keywordRulesImportHandler)
	mux.HandleFunc("/api/admin/keyword-rules", s.keywordRulesHandler)
	mux.HandleFunc("/api/admin/keyword-rules/", s.keywordRuleHandler)
	mux.HandleFunc("/api/admin/projects", s.projectsHandler)
	mux.HandleFunc("/api/admin/projects/", s.projectHandler)
	mux.HandleFunc("/api/admin/project-options", s.projectOptionsHandler)
	mux.HandleFunc("/api/admin/project-options/", s.projectOptionHandler)
}

func (s *Service) projectsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionProjects); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		projects, err := s.listProjects(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query projects failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: projects})
	case http.MethodPost:
		project, ok := s.decodeProject(w, r)
		if !ok {
			return
		}
		created, err := s.createProject(r.Context(), project)
		if err != nil {
			if isUniqueViolation(err) || errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusConflict, "项目名称已存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "create project failed")
			return
		}
		response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "success", Data: created})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) projectHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionProjects); !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/admin/projects/"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		project, ok := s.decodeProject(w, r)
		if !ok {
			return
		}
		updated, err := s.updateProject(r.Context(), id, project)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			if isUniqueViolation(err) {
				writeError(w, http.StatusConflict, "项目名称已存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "update project failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: updated})
	case http.MethodDelete:
		result, err := s.db.ExecContext(r.Context(), `UPDATE logmaster_api.projects SET is_active = FALSE, updated_at = NOW()
			WHERE id = $1 AND is_active = TRUE`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "delete project failed")
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) projectOptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if _, ok := s.currentUser(r); !ok {
			writeError(w, http.StatusUnauthorized, "Feishu login required")
			return
		}
	} else {
		if _, ok := s.requirePermission(w, r, permissionProjects); !ok {
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.QueryContext(r.Context(), `SELECT id, kind, code, name, is_system
			FROM logmaster_api.project_taxonomies WHERE is_active = TRUE
			ORDER BY kind, is_system DESC, name`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query project options failed")
			return
		}
		defer rows.Close()
		options := map[string][]ProjectOption{"lines": {}, "types": {}, "stages": {}}
		for rows.Next() {
			var option ProjectOption
			if err := rows.Scan(&option.ID, &option.Kind, &option.Code, &option.Name, &option.IsSystem); err != nil {
				writeError(w, http.StatusInternalServerError, "query project options failed")
				return
			}
			options[option.Kind+"s"] = append(options[option.Kind+"s"], option)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "query project options failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: options})
	case http.MethodPost:
		input, ok := decodeProjectOption(w, r)
		if !ok {
			return
		}
		var option ProjectOption
		code := "custom_" + strconv.FormatInt(time.Now().UnixNano(), 36)
		err := s.db.QueryRowContext(r.Context(), `INSERT INTO logmaster_api.project_taxonomies (kind, code, name)
			VALUES ($1, $2, $3)
			ON CONFLICT (kind, name) DO UPDATE SET is_active = TRUE, updated_at = NOW()
			WHERE project_taxonomies.is_active = FALSE
			RETURNING id, kind, code, name, is_system`, input.Kind, code, input.Name).
			Scan(&option.ID, &option.Kind, &option.Code, &option.Name, &option.IsSystem)
		if err != nil {
			if isUniqueViolation(err) || errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusConflict, "该名称已存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "create project option failed")
			return
		}
		response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "success", Data: option})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) projectOptionHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionProjects); !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/admin/project-options/"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid project option id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		input, ok := decodeProjectOption(w, r)
		if !ok {
			return
		}
		var option ProjectOption
		err := s.db.QueryRowContext(r.Context(), `UPDATE logmaster_api.project_taxonomies
			SET name = $2, updated_at = NOW() WHERE id = $1 AND kind = $3 AND is_active = TRUE
			RETURNING id, kind, code, name, is_system`, id, input.Name, input.Kind).
			Scan(&option.ID, &option.Kind, &option.Code, &option.Name, &option.IsSystem)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project option not found")
			return
		}
		if err != nil {
			if isUniqueViolation(err) {
				writeError(w, http.StatusConflict, "该名称已存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "update project option failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: option})
	case http.MethodDelete:
		var kind, code string
		var system bool
		err := s.db.QueryRowContext(r.Context(), `SELECT kind, code, is_system FROM logmaster_api.project_taxonomies
			WHERE id = $1 AND is_active = TRUE`, id).Scan(&kind, &code, &system)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project option not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query project option failed")
			return
		}
		if system {
			writeError(w, http.StatusConflict, "系统预置项不能删除")
			return
		}
		column := map[string]string{"line": "product_line", "type": "product_type", "stage": "stage"}[kind]
		var used bool
		query := `SELECT EXISTS (SELECT 1 FROM logmaster_api.projects WHERE is_active = TRUE AND ` + column + ` = $1)`
		if err := s.db.QueryRowContext(r.Context(), query, code).Scan(&used); err != nil {
			writeError(w, http.StatusInternalServerError, "check project option usage failed")
			return
		}
		if used {
			writeError(w, http.StatusConflict, "该选项正在被项目使用，暂不能删除")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE logmaster_api.project_taxonomies
			SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, id); err != nil {
			writeError(w, http.StatusInternalServerError, "delete project option failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
	default:
		methodNotAllowed(w)
	}
}

func decodeProjectOption(w http.ResponseWriter, r *http.Request) (projectOptionInput, bool) {
	var input projectOptionInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project option data")
		return projectOptionInput{}, false
	}
	input.Kind = strings.TrimSpace(input.Kind)
	input.Name = strings.TrimSpace(input.Name)
	if input.Kind != "line" && input.Kind != "type" && input.Kind != "stage" {
		writeError(w, http.StatusBadRequest, "invalid project option kind")
		return projectOptionInput{}, false
	}
	if input.Name == "" || len([]rune(input.Name)) > 64 {
		writeError(w, http.StatusBadRequest, "选项名称不能为空且不能超过 64 个字符")
		return projectOptionInput{}, false
	}
	return input, true
}

func (s *Service) decodeProject(w http.ResponseWriter, r *http.Request) (Project, bool) {
	var project Project
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	if err := decoder.Decode(&project); err != nil {
		writeError(w, http.StatusBadRequest, "invalid project data")
		return Project{}, false
	}
	project.Name = strings.ToUpper(strings.TrimSpace(project.Name))
	project.Description = strings.TrimSpace(project.Description)
	if !projectNamePattern.MatchString(project.Name) {
		writeError(w, http.StatusBadRequest, "项目名称仅支持大写字母、数字和连字符")
		return Project{}, false
	}
	for kind, code := range map[string]string{"line": project.ProductLine, "type": project.ProductType, "stage": project.Stage} {
		var exists bool
		if err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (
			SELECT 1 FROM logmaster_api.project_taxonomies WHERE kind = $1 AND code = $2 AND is_active = TRUE
		)`, kind, code).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusBadRequest, "项目属性不存在或已停用")
			return Project{}, false
		}
	}
	if len(project.Description) > 1000 {
		writeError(w, http.StatusBadRequest, "description is too long")
		return Project{}, false
	}
	return project, true
}

func (s *Service) listProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, product_line, product_type, stage, description, created_at, updated_at
		FROM logmaster_api.projects WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.Name, &project.ProductLine, &project.ProductType, &project.Stage,
			&project.Description, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Service) createProject(ctx context.Context, project Project) (Project, error) {
	err := s.db.QueryRowContext(ctx, `INSERT INTO logmaster_api.projects (name, product_line, product_type, stage, description, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		ON CONFLICT (name) DO UPDATE SET
			product_line = EXCLUDED.product_line,
			product_type = EXCLUDED.product_type,
			stage = EXCLUDED.stage,
			description = EXCLUDED.description,
			is_active = TRUE,
			updated_at = NOW()
		WHERE projects.is_active = FALSE
		RETURNING id, name, product_line, product_type, stage, description, created_at, updated_at`,
		project.Name, project.ProductLine, project.ProductType, project.Stage, project.Description).
		Scan(&project.ID, &project.Name, &project.ProductLine, &project.ProductType, &project.Stage,
			&project.Description, &project.CreatedAt, &project.UpdatedAt)
	return project, err
}

func (s *Service) updateProject(ctx context.Context, id int64, project Project) (Project, error) {
	err := s.db.QueryRowContext(ctx, `UPDATE logmaster_api.projects
		SET name = $2, product_line = $3, product_type = $4, stage = $5, description = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, product_line, product_type, stage, description, created_at, updated_at`,
		id, project.Name, project.ProductLine, project.ProductType, project.Stage, project.Description).
		Scan(&project.ID, &project.Name, &project.ProductLine, &project.ProductType, &project.Stage,
			&project.Description, &project.CreatedAt, &project.UpdatedAt)
	return project, err
}

func (s *Service) currentUser(r *http.Request) (string, bool) {
	if s.currentUserResolver == nil {
		return "", false
	}
	return s.currentUserResolver(r)
}

func (s *Service) requirePermission(w http.ResponseWriter, r *http.Request, permission string) (string, bool) {
	openID, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Feishu login required")
		return "", false
	}
	role, err := s.roleForUser(r.Context(), openID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query user role failed")
		return "", false
	}
	if !roleHasPermission(role, permission) {
		writeError(w, http.StatusForbidden, "当前账号无权执行此操作")
		return "", false
	}
	return openID, true
}

func (s *Service) roleForUser(ctx context.Context, openID string) (string, error) {
	if s.roleResolver != nil {
		return s.roleResolver(ctx, openID)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE logmaster_api.users SET role = $2, updated_at = NOW()
		WHERE feishu_open_id = $1 AND name = '刘欣彤' AND role = $3
		AND NOT EXISTS (SELECT 1 FROM logmaster_api.users WHERE role = $2)`, openID, roleSuperAdmin, roleUser); err != nil {
		return "", err
	}
	var role string
	err := s.db.QueryRowContext(ctx, `SELECT role FROM logmaster_api.users WHERE feishu_open_id = $1`, openID).Scan(&role)
	return role, err
}

func permissionsForRole(role string) []string {
	switch role {
	case roleDeveloper:
		return []string{permissionApprovals, permissionKeywords}
	case roleAdmin:
		return []string{permissionProjects, permissionApprovals, permissionKeywords}
	case roleSuperAdmin:
		return []string{permissionUsers, permissionProjects, permissionCapacity, permissionApprovals, permissionKeywords}
	default:
		return []string{}
	}
}

func roleHasPermission(role, permission string) bool {
	for _, allowed := range permissionsForRole(role) {
		if allowed == permission {
			return true
		}
	}
	return false
}

func (s *Service) automaticRole(openID, jobTitle string) string {
	for _, id := range strings.Split(s.config.FeishuSuperAdminIDs, ",") {
		if openID != "" && strings.TrimSpace(id) == openID {
			return roleSuperAdmin
		}
	}
	for _, rule := range strings.Split(s.config.FeishuRoleTitleRules, ";") {
		parts := strings.SplitN(rule, "=", 2)
		if len(parts) != 2 || !strings.Contains(strings.ToLower(jobTitle), strings.ToLower(strings.TrimSpace(parts[0]))) {
			continue
		}
		role := strings.TrimSpace(parts[1])
		if role == roleUser || role == roleDeveloper || role == roleAdmin {
			return role
		}
	}
	return roleUser
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func writeError(w http.ResponseWriter, status int, message string) {
	response.JSONStatus(w, status, response.APIResponse{Code: status, Message: message, Data: nil})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
