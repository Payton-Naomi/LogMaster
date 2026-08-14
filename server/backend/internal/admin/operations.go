package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"logmaster-agent/internal/response"
)

type runtimeLog struct {
	ID          int64     `json:"id"`
	OwnerOpenID string    `json:"owner_open_id"`
	OwnerName   string    `json:"owner_name"`
	Module      string    `json:"module"`
	Event       string    `json:"event"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	TaskID      string    `json:"task_id"`
	QueryCode   string    `json:"query_code"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Service) runtimeLogsHandler(w http.ResponseWriter, r *http.Request) {
	openID, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Feishu login required")
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	role, err := s.roleForUser(r.Context(), openID)
	if err != nil {
		writeError(w, 500, "query user role failed")
		return
	}
	query := `SELECT l.id, l.owner_open_id, COALESCE(u.name,''), l.module, l.event, l.status, l.message, l.task_id, l.query_code, l.created_at
		FROM logmaster_api.runtime_logs l LEFT JOIN logmaster_api.users u ON u.feishu_open_id=l.owner_open_id`
	args := []any{}
	if role == roleUser || role == roleDeveloper {
		query += ` WHERE l.owner_open_id=$1`
		args = append(args, openID)
	}
	query += ` ORDER BY l.created_at DESC LIMIT 500`
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "query runtime logs failed")
		return
	}
	defer rows.Close()
	items := make([]runtimeLog, 0)
	for rows.Next() {
		var item runtimeLog
		if rows.Scan(&item.ID, &item.OwnerOpenID, &item.OwnerName, &item.Module, &item.Event, &item.Status, &item.Message, &item.TaskID, &item.QueryCode, &item.CreatedAt) != nil {
			writeError(w, 500, "query runtime logs failed")
			return
		}
		items = append(items, item)
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: items})
}

type projectRequest struct {
	ID              int64      `json:"id"`
	ApplicantOpenID string     `json:"applicant_open_id"`
	ApplicantName   string     `json:"applicant_name"`
	Name            string     `json:"name"`
	ProductLine     string     `json:"product_line"`
	ProductType     string     `json:"product_type"`
	Stage           string     `json:"stage"`
	Description     string     `json:"description"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	ReviewerName    string     `json:"reviewer_name"`
	ReviewComment   string     `json:"review_comment"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}
type projectRequestInput struct {
	Name        string `json:"name"`
	ProductLine string `json:"product_line"`
	ProductType string `json:"product_type"`
	Stage       string `json:"stage"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}
type projectDecisionInput struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}

const projectRequestSelect = `SELECT pr.id,pr.applicant_open_id,a.name,pr.name,pr.product_line,pr.product_type,pr.stage,pr.description,pr.reason,pr.status,COALESCE(rv.name,''),pr.review_comment,pr.reviewed_at,pr.created_at FROM logmaster_api.project_creation_requests pr JOIN logmaster_api.users a ON a.feishu_open_id=pr.applicant_open_id LEFT JOIN logmaster_api.users rv ON rv.feishu_open_id=pr.reviewer_open_id`

func (s *Service) projectRequestsHandler(w http.ResponseWriter, r *http.Request) {
	openID, ok := s.currentUser(r)
	if !ok {
		writeError(w, 401, "Feishu login required")
		return
	}
	role, err := s.roleForUser(r.Context(), openID)
	if err != nil {
		writeError(w, 500, "query user role failed")
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := projectRequestSelect
		args := []any{}
		if role == roleUser || role == roleDeveloper {
			query += ` WHERE pr.applicant_open_id=$1`
			args = append(args, openID)
		}
		query += ` ORDER BY CASE pr.status WHEN 'pending' THEN 1 ELSE 2 END,pr.created_at DESC`
		rows, err := s.db.QueryContext(r.Context(), query, args...)
		if err != nil {
			writeError(w, 500, "query project requests failed")
			return
		}
		defer rows.Close()
		items := []projectRequest{}
		for rows.Next() {
			item, err := scanProjectRequest(rows)
			if err != nil {
				writeError(w, 500, "query project requests failed")
				return
			}
			items = append(items, item)
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"requests": items, "can_review": role == roleAdmin || role == roleSuperAdmin}})
	case http.MethodPost:
		if role == roleAdmin || role == roleSuperAdmin {
			writeError(w, http.StatusForbidden, "administrators can create projects directly")
			return
		}
		var in projectRequestInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil {
			writeError(w, 400, "invalid project request")
			return
		}
		in.Name = strings.ToUpper(strings.TrimSpace(in.Name))
		in.ProductLine = strings.TrimSpace(in.ProductLine)
		in.ProductType = strings.TrimSpace(in.ProductType)
		in.Stage = strings.TrimSpace(in.Stage)
		in.Description = strings.TrimSpace(in.Description)
		in.Reason = strings.TrimSpace(in.Reason)
		if !projectNamePattern.MatchString(in.Name) || in.Reason == "" || len(in.Description) > 1000 || len(in.Reason) > 1000 {
			writeError(w, 400, "project name and reason are required")
			return
		}
		for kind, code := range map[string]string{"line": in.ProductLine, "type": in.ProductType, "stage": in.Stage} {
			var exists bool
			err := s.db.QueryRowContext(r.Context(), `SELECT EXISTS (
				SELECT 1 FROM logmaster_api.project_taxonomies WHERE kind=$1 AND code=$2 AND is_active=TRUE
			)`, kind, code).Scan(&exists)
			if err != nil || !exists {
				writeError(w, 400, "project attributes do not exist or are disabled")
				return
			}
		}
		var id int64
		err := s.db.QueryRowContext(r.Context(), `INSERT INTO logmaster_api.project_creation_requests(applicant_open_id,name,product_line,product_type,stage,description,reason) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, openID, in.Name, in.ProductLine, in.ProductType, in.Stage, in.Description, in.Reason).Scan(&id)
		if err != nil {
			writeError(w, 409, "project request already exists or is invalid")
			return
		}
		item, err := s.getProjectRequest(r, id)
		if err != nil {
			writeError(w, 500, "load project request failed")
			return
		}
		response.JSONStatus(w, 201, response.APIResponse{Code: 0, Message: "success", Data: item})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) projectRequestHandler(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := s.requirePermission(w, r, permissionProjects)
	if !ok {
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/project-requests/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "decision" {
		writeError(w, 404, "endpoint not found")
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, 400, "invalid request id")
		return
	}
	var in projectDecisionInput
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in) != nil {
		writeError(w, 400, "invalid decision")
		return
	}
	in.Action = strings.TrimSpace(in.Action)
	in.Comment = strings.TrimSpace(in.Comment)
	if in.Action != "approve" && in.Action != "reject" {
		writeError(w, 400, "invalid decision")
		return
	}
	if in.Action == "reject" && in.Comment == "" {
		writeError(w, 400, "rejection reason is required")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "review project request failed")
		return
	}
	defer tx.Rollback()
	var req projectRequestInput
	var applicant, status string
	err = tx.QueryRowContext(r.Context(), `SELECT applicant_open_id,name,product_line,product_type,stage,description,status FROM logmaster_api.project_creation_requests WHERE id=$1 FOR UPDATE`, id).Scan(&applicant, &req.Name, &req.ProductLine, &req.ProductType, &req.Stage, &req.Description, &status)
	if errors.Is(err, sql.ErrNoRows) || status != "pending" {
		writeError(w, 409, "request not found or already reviewed")
		return
	}
	newStatus := "rejected"
	if in.Action == "approve" {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO logmaster_api.projects(name,product_line,product_type,stage,description,is_active) VALUES($1,$2,$3,$4,$5,TRUE)`, req.Name, req.ProductLine, req.ProductType, req.Stage, req.Description)
		if err != nil {
			writeError(w, 409, "project already exists or is invalid")
			return
		}
		newStatus = "approved"
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE logmaster_api.project_creation_requests SET status=$2,reviewer_open_id=$3,review_comment=$4,reviewed_at=NOW(),updated_at=NOW() WHERE id=$1`, id, newStatus, reviewer, in.Comment)
	if err != nil || tx.Commit() != nil {
		writeError(w, 500, "review project request failed")
		return
	}
	item, _ := s.getProjectRequest(r, id)
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: item})
}

func (s *Service) getProjectRequest(r *http.Request, id int64) (projectRequest, error) {
	return scanProjectRequest(s.db.QueryRowContext(r.Context(), projectRequestSelect+` WHERE pr.id=$1`, id))
}

type projectRequestScanner interface{ Scan(...any) error }

func scanProjectRequest(s projectRequestScanner) (projectRequest, error) {
	var v projectRequest
	err := s.Scan(&v.ID, &v.ApplicantOpenID, &v.ApplicantName, &v.Name, &v.ProductLine, &v.ProductType, &v.Stage, &v.Description, &v.Reason, &v.Status, &v.ReviewerName, &v.ReviewComment, &v.ReviewedAt, &v.CreatedAt)
	return v, err
}
