package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"logmaster-agent/internal/response"
)

type permissionRequest struct {
	ID              int64      `json:"id"`
	ApplicantUserID int64      `json:"applicant_user_id"`
	ApplicantOpenID string     `json:"applicant_open_id"`
	ApplicantName   string     `json:"applicant_name"`
	ApplicantEmail  string     `json:"applicant_email"`
	CurrentRole     string     `json:"current_role"`
	RequestedRole   string     `json:"requested_role"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	ReviewerOpenID  string     `json:"reviewer_open_id"`
	ReviewerName    string     `json:"reviewer_name"`
	ReviewComment   string     `json:"review_comment"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type createPermissionRequestInput struct {
	RequestedRole string `json:"requested_role"`
	Reason        string `json:"reason"`
}

type permissionDecisionInput struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}

var (
	errApplicantRoleChanged    = errors.New("applicant role changed after request")
	errInvalidPermissionReason = errors.New("invalid permission request reason")
)

const permissionRequestSelect = `SELECT pr.id, pr.applicant_user_id, pr.applicant_open_id,
	applicant.name, applicant.email, pr.source_role, pr.requested_role, pr.reason, pr.status,
	COALESCE(pr.reviewer_open_id, ''), COALESCE(reviewer.name, ''), pr.review_comment,
	pr.reviewed_at, pr.created_at, pr.updated_at
	FROM logmaster_api.permission_requests pr
	JOIN logmaster_api.users applicant ON applicant.id = pr.applicant_user_id
	LEFT JOIN logmaster_api.users reviewer ON reviewer.feishu_open_id = pr.reviewer_open_id`

func (s *Service) permissionRequestsHandler(w http.ResponseWriter, r *http.Request) {
	openID, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Feishu login required")
		return
	}
	role, err := s.roleForUser(r.Context(), openID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query user role failed")
		return
	}
	switch r.Method {
	case http.MethodGet:
		canReview := canReviewPermissionRequests(role)
		requests, err := s.listPermissionRequests(r, openID, canReview)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query permission requests failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{
			"requests": requests, "current_role": role, "can_apply": role != roleSuperAdmin, "can_review": canReview,
		}})
	case http.MethodPost:
		if role == roleSuperAdmin {
			writeError(w, http.StatusForbidden, "超级管理员无需申请权限变更")
			return
		}
		created, err := s.createPermissionRequest(r, openID, role)
		if errors.Is(err, errApplicantRoleChanged) {
			writeError(w, http.StatusBadRequest, "申请等级必须与当前等级不同")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "你已有一条待审批申请，请等待处理或先撤回")
			return
		}
		if errors.Is(err, errInvalidPermissionReason) {
			writeError(w, http.StatusBadRequest, "请填写不超过 1000 字的申请原因")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create permission request failed")
			return
		}
		response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "success", Data: created})
	default:
		methodNotAllowed(w)
	}
}

func canReviewPermissionRequests(role string) bool {
	return role == roleSuperAdmin
}

func (s *Service) permissionRequestHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/permission-requests/"), "/")
	parts := strings.Split(path, "/")
	requestID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || requestID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid permission request id")
		return
	}
	if r.Method == http.MethodDelete && len(parts) == 1 {
		s.cancelPermissionRequest(w, r, requestID)
		return
	}
	if r.Method != http.MethodPut || len(parts) != 2 || parts[1] != "decision" {
		methodNotAllowed(w)
		return
	}
	s.decidePermissionRequest(w, r, requestID)
}

func (s *Service) listPermissionRequests(r *http.Request, openID string, canReview bool) ([]permissionRequest, error) {
	query := permissionRequestSelect
	args := []any{}
	if canReview {
		query += ` ORDER BY CASE pr.status WHEN 'pending' THEN 1 ELSE 2 END, pr.created_at DESC`
	} else {
		query += ` WHERE pr.applicant_open_id = $1 ORDER BY pr.created_at DESC`
		args = append(args, openID)
	}
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]permissionRequest, 0)
	for rows.Next() {
		request, err := scanPermissionRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (s *Service) createPermissionRequest(r *http.Request, openID, currentRole string) (permissionRequest, error) {
	var input createPermissionRequestInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	if err := decoder.Decode(&input); err != nil {
		return permissionRequest{}, errors.New("申请内容无效")
	}
	input.RequestedRole = strings.TrimSpace(input.RequestedRole)
	input.Reason = strings.TrimSpace(input.Reason)
	if !validUserRole(input.RequestedRole) || input.RequestedRole == currentRole {
		return permissionRequest{}, errApplicantRoleChanged
	}
	if input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return permissionRequest{}, errInvalidPermissionReason
	}
	var requestID int64
	err := s.db.QueryRowContext(r.Context(), `INSERT INTO logmaster_api.permission_requests
		(applicant_user_id, applicant_open_id, source_role, requested_role, reason)
		SELECT id, feishu_open_id, role, $2, $3 FROM logmaster_api.users
		WHERE feishu_open_id = $1 AND role = $4 RETURNING id`, openID, input.RequestedRole, input.Reason, currentRole).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return permissionRequest{}, errApplicantRoleChanged
	}
	if err != nil {
		return permissionRequest{}, err
	}
	return s.getPermissionRequest(r, requestID)
}

func (s *Service) cancelPermissionRequest(w http.ResponseWriter, r *http.Request, requestID int64) {
	openID, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Feishu login required")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `UPDATE logmaster_api.permission_requests
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND applicant_open_id = $2 AND status = 'pending'`, requestID, openID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cancel permission request failed")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusConflict, "申请不存在或已处理")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
}

func (s *Service) decidePermissionRequest(w http.ResponseWriter, r *http.Request, requestID int64) {
	reviewerOpenID, ok := s.requirePermission(w, r, permissionApprovals)
	if !ok {
		return
	}
	reviewerRole, err := s.roleForUser(r.Context(), reviewerOpenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query reviewer role failed")
		return
	}
	if reviewerRole != roleSuperAdmin {
		writeError(w, http.StatusForbidden, "仅超级管理员可以审批权限申请")
		return
	}
	var input permissionDecisionInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "审批内容无效")
		return
	}
	input.Action = strings.TrimSpace(input.Action)
	input.Comment = strings.TrimSpace(input.Comment)
	if input.Action != "approve" && input.Action != "reject" || len([]rune(input.Comment)) > 1000 {
		writeError(w, http.StatusBadRequest, "审批操作或意见无效")
		return
	}
	updated, err := s.applyPermissionDecision(r, requestID, reviewerOpenID, reviewerRole, input)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusConflict, "申请不存在或已处理")
	case errors.Is(err, errApplicantRoleChanged):
		writeError(w, http.StatusConflict, "申请人的当前等级已变化，请驳回后重新申请")
	case errors.Is(err, errSelfApproval):
		writeError(w, http.StatusForbidden, "不能审批自己的权限申请")
	case errors.Is(err, errSuperAdminApprovalRequired):
		writeError(w, http.StatusForbidden, "升级为超级管理员必须由超级管理员审批")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "review permission request failed")
	default:
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: updated})
	}
}

var (
	errSelfApproval               = errors.New("self approval is forbidden")
	errSuperAdminApprovalRequired = errors.New("super admin approval required")
)

func (s *Service) applyPermissionDecision(r *http.Request, requestID int64, reviewerOpenID, reviewerRole string, input permissionDecisionInput) (permissionRequest, error) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return permissionRequest{}, err
	}
	defer tx.Rollback()
	var applicantUserID int64
	var applicantOpenID, currentRole, requestedRole string
	err = tx.QueryRowContext(r.Context(), `SELECT applicant_user_id, applicant_open_id, source_role, requested_role
		FROM logmaster_api.permission_requests WHERE id = $1 AND status = 'pending' FOR UPDATE`, requestID).Scan(
		&applicantUserID, &applicantOpenID, &currentRole, &requestedRole)
	if err != nil {
		return permissionRequest{}, err
	}
	if applicantOpenID == reviewerOpenID {
		return permissionRequest{}, errSelfApproval
	}
	if input.Action == "approve" && requestedRole == roleSuperAdmin && reviewerRole != roleSuperAdmin {
		return permissionRequest{}, errSuperAdminApprovalRequired
	}
	status := "rejected"
	if input.Action == "approve" {
		result, err := tx.ExecContext(r.Context(), `UPDATE logmaster_api.users SET role = $2, updated_at = NOW()
			WHERE id = $1 AND role = $3`, applicantUserID, requestedRole, currentRole)
		if err != nil {
			return permissionRequest{}, err
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			return permissionRequest{}, errApplicantRoleChanged
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO logmaster_api.user_role_audit_logs
			(target_user_id, target_open_id, old_role, new_role, changed_by_open_id)
			VALUES ($1, $2, $3, $4, $5)`, applicantUserID, applicantOpenID, currentRole, requestedRole, reviewerOpenID); err != nil {
			return permissionRequest{}, err
		}
		status = "approved"
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE logmaster_api.permission_requests
		SET status = $2, reviewer_open_id = $3, review_comment = $4, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1`, requestID, status, reviewerOpenID, input.Comment); err != nil {
		return permissionRequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return permissionRequest{}, err
	}
	return s.getPermissionRequest(r, requestID)
}

func (s *Service) getPermissionRequest(r *http.Request, id int64) (permissionRequest, error) {
	return scanPermissionRequest(s.db.QueryRowContext(r.Context(), permissionRequestSelect+` WHERE pr.id = $1`, id))
}

type permissionRequestScanner interface {
	Scan(dest ...any) error
}

func scanPermissionRequest(scanner permissionRequestScanner) (permissionRequest, error) {
	var request permissionRequest
	err := scanner.Scan(&request.ID, &request.ApplicantUserID, &request.ApplicantOpenID, &request.ApplicantName,
		&request.ApplicantEmail, &request.CurrentRole, &request.RequestedRole, &request.Reason, &request.Status,
		&request.ReviewerOpenID, &request.ReviewerName, &request.ReviewComment, &request.ReviewedAt,
		&request.CreatedAt, &request.UpdatedAt)
	return request, err
}
