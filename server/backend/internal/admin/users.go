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

type adminUser struct {
	ID           int64     `json:"id"`
	FeishuOpenID string    `json:"feishu_open_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	RoleSource   string    `json:"role_source"`
	JobTitle     string    `json:"job_title"`
	IsCurrent    bool      `json:"is_current"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type userRoleInput struct {
	Role string `json:"role"`
}

func (s *Service) usersHandler(w http.ResponseWriter, r *http.Request) {
	openID, ok := s.requirePermission(w, r, permissionUsers)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id, feishu_open_id, name, email, role, role_source, job_title,
		feishu_open_id = $1, created_at, updated_at
		FROM logmaster_api.users
		ORDER BY CASE role WHEN 'super_admin' THEN 1 WHEN 'admin' THEN 2 WHEN 'developer' THEN 3 ELSE 4 END,
		name, created_at`, openID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query users failed")
		return
	}
	defer rows.Close()
	users := make([]adminUser, 0)
	for rows.Next() {
		var user adminUser
		if err := rows.Scan(&user.ID, &user.FeishuOpenID, &user.Name, &user.Email, &user.Role, &user.RoleSource, &user.JobTitle,
			&user.IsCurrent, &user.CreatedAt, &user.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "query users failed")
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "query users failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: users})
}

func (s *Service) userRoleHandler(w http.ResponseWriter, r *http.Request) {
	changedBy, ok := s.requirePermission(w, r, permissionUsers)
	if !ok {
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "role" {
		writeError(w, http.StatusNotFound, "user endpoint not found")
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var input userRoleInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := decoder.Decode(&input); err != nil || !validUserRole(input.Role) {
		writeError(w, http.StatusBadRequest, "invalid user role")
		return
	}
	user, err := s.updateUserRole(r, userID, input.Role, changedBy)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, errLastSuperAdmin) {
		writeError(w, http.StatusConflict, "系统至少需要保留一名超级管理员")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update user role failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: user})
}

func (s *Service) restoreUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	changedBy, ok := s.requirePermission(w, r, permissionUsers)
	if !ok {
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "restore-feishu-role" {
		writeError(w, http.StatusNotFound, "user endpoint not found")
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var user adminUser
	err = s.db.QueryRowContext(r.Context(), `UPDATE logmaster_api.users
		SET role = 'user', role_source = 'feishu', updated_at = NOW()
		WHERE id = $1
		RETURNING id, feishu_open_id, name, email, role, role_source, job_title, created_at, updated_at`, userID).
		Scan(&user.ID, &user.FeishuOpenID, &user.Name, &user.Email, &user.Role, &user.RoleSource, &user.JobTitle, &user.CreatedAt, &user.UpdatedAt)
	role := s.automaticRole(user.FeishuOpenID, user.JobTitle)
	if err := s.db.QueryRowContext(r.Context(), `UPDATE logmaster_api.users SET role = $2, updated_at = NOW() WHERE id = $1 RETURNING role, updated_at`, user.ID, role).Scan(&user.Role, &user.UpdatedAt); err != nil {
		writeError(w, http.StatusInternalServerError, "restore user role failed")
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "restore user role failed")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO logmaster_api.user_role_audit_logs
		(target_user_id, target_open_id, old_role, new_role, changed_by_open_id) VALUES ($1, $2, 'manual', $3, $4)`, user.ID, user.FeishuOpenID, user.Role, changedBy); err != nil {
		writeError(w, http.StatusInternalServerError, "record role audit failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: user})
}

var errLastSuperAdmin = errors.New("cannot demote the last super admin")

func (s *Service) updateUserRole(r *http.Request, userID int64, newRole, changedBy string) (adminUser, error) {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return adminUser{}, err
	}
	defer tx.Rollback()
	var user adminUser
	err = tx.QueryRowContext(r.Context(), `SELECT id, feishu_open_id, name, email, role, role_source, job_title, created_at, updated_at
		FROM logmaster_api.users WHERE id = $1 FOR UPDATE`, userID).Scan(
		&user.ID, &user.FeishuOpenID, &user.Name, &user.Email, &user.Role, &user.RoleSource, &user.JobTitle, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return adminUser{}, err
	}
	oldRole := user.Role
	if oldRole == newRole {
		user.IsCurrent = user.FeishuOpenID == changedBy
		return user, tx.Commit()
	}
	if oldRole == roleSuperAdmin && newRole != roleSuperAdmin {
		var superAdminCount int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM logmaster_api.users WHERE role = $1`, roleSuperAdmin).Scan(&superAdminCount); err != nil {
			return adminUser{}, err
		}
		if superAdminCount <= 1 {
			return adminUser{}, errLastSuperAdmin
		}
	}
	err = tx.QueryRowContext(r.Context(), `UPDATE logmaster_api.users SET role = $2, role_source = 'manual', updated_at = NOW()
		WHERE id = $1 RETURNING role, role_source, updated_at`, userID, newRole).Scan(&user.Role, &user.RoleSource, &user.UpdatedAt)
	if err != nil {
		return adminUser{}, err
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO logmaster_api.user_role_audit_logs
		(target_user_id, target_open_id, old_role, new_role, changed_by_open_id)
		VALUES ($1, $2, $3, $4, $5)`, user.ID, user.FeishuOpenID, oldRole, newRole, changedBy); err != nil {
		return adminUser{}, err
	}
	user.IsCurrent = user.FeishuOpenID == changedBy
	return user, tx.Commit()
}

func validUserRole(role string) bool {
	return role == roleUser || role == roleDeveloper || role == roleAdmin || role == roleSuperAdmin
}
