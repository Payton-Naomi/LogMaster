package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"logmaster-agent/internal/response"
)

type externalRegisterRequest struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	Company         string `json:"company"`
}

type externalLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type externalChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type externalChangeEmailRequest struct {
	CurrentPassword string `json:"current_password"`
	Email           string `json:"email"`
}

func (s *Service) externalRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var input externalRegisterRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := validateExternalRegistration(input)
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "外包账号密码处理失败")
		return
	}
	if err := s.createExternalUser(r.Context(), &user, passwordHash); err != nil {
		if errors.Is(err, errEmailAlreadyRegistered) {
			writeAuthError(w, http.StatusConflict, "该邮箱已注册，不能重复注册")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "创建外包账号失败")
		return
	}
	s.saveSession(w, user)
	s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "外包账号注册", "success", "external account registered")
	response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "外包账号注册成功", Data: user})
}

func (s *Service) externalLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var input externalLoginRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil || strings.TrimSpace(input.Password) == "" {
		writeAuthError(w, http.StatusBadRequest, "请输入有效邮箱和密码")
		return
	}
	user, passwordHash, err := s.externalUserByEmail(r.Context(), email)
	if err != nil || !verifyPassword(passwordHash, input.Password) {
		writeAuthError(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	s.saveSession(w, user)
	s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "外包账号登录", "success", "external login succeeded")
	response.JSON(w, response.APIResponse{Code: 0, Message: "登录成功", Data: user})
}

func (s *Service) externalChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := s.CurrentUser(r)
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if user.IdentityType != "external" {
		writeAuthError(w, http.StatusForbidden, "飞书账号请在飞书平台修改密码")
		return
	}
	var input externalChangePasswordRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Password == "" || input.Password != input.ConfirmPassword {
		writeAuthError(w, http.StatusBadRequest, "新密码不能为空且两次输入必须一致")
		return
	}
	var currentHash string
	if err := s.db.QueryRowContext(r.Context(), `SELECT c.password_hash
		FROM logmaster_api.external_password_credentials c
		JOIN logmaster_api.users u ON u.id = c.user_id
		WHERE u.feishu_open_id = $1 AND u.identity_type = 'external'`, user.FeishuOpenID).Scan(&currentHash); err != nil || !verifyPassword(currentHash, input.CurrentPassword) {
		writeAuthError(w, http.StatusUnauthorized, "当前密码错误")
		return
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "密码处理失败")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `UPDATE logmaster_api.external_password_credentials c
		SET password_hash = $1, password_updated_at = NOW(), updated_at = NOW()
		FROM logmaster_api.users u WHERE c.user_id = u.id AND u.feishu_open_id = $2`, passwordHash, user.FeishuOpenID); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改密码失败")
		return
	}
	s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "外包账号修改密码", "success", "external password changed")
	response.JSON(w, response.APIResponse{Code: 0, Message: "密码修改成功", Data: nil})
}

func (s *Service) externalChangeEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	user, ok := s.CurrentUser(r)
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if user.IdentityType != "external" {
		writeAuthError(w, http.StatusForbidden, "飞书账号邮箱由飞书平台维护")
		return
	}
	var input externalChangeEmailRequest
	if err := decodeJSON(r, &input); err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, err.Error())
		return
	}
	var currentHash string
	if err := s.db.QueryRowContext(r.Context(), `SELECT c.password_hash
		FROM logmaster_api.external_password_credentials c
		JOIN logmaster_api.users u ON u.id = c.user_id
		WHERE u.feishu_open_id = $1 AND u.identity_type = 'external'`, user.FeishuOpenID).Scan(&currentHash); err != nil || !verifyPassword(currentHash, input.CurrentPassword) {
		writeAuthError(w, http.StatusUnauthorized, "当前密码错误")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改邮箱失败")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT pg_advisory_xact_lock(hashtext($1))`, email); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改邮箱失败")
		return
	}
	var exists bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM logmaster_api.users WHERE lower(trim(email)) = lower(trim($1)) AND feishu_open_id <> $2)`, email, user.FeishuOpenID).Scan(&exists); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改邮箱失败")
		return
	}
	if exists {
		writeAuthError(w, http.StatusConflict, "该邮箱已被使用")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE logmaster_api.users SET email = $1, updated_at = NOW() WHERE feishu_open_id = $2 AND identity_type = 'external'`, email, user.FeishuOpenID); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改邮箱失败")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "修改邮箱失败")
		return
	}
	user.Email = email
	s.saveSession(w, user)
	s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "外包账号修改邮箱", "success", "external email changed")
	response.JSON(w, response.APIResponse{Code: 0, Message: "邮箱修改成功", Data: user})
}

var errEmailAlreadyRegistered = errors.New("email already registered")

func (s *Service) createExternalUser(ctx context.Context, user *UserInfo, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, user.Email); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM logmaster_api.users WHERE lower(trim(email)) = lower(trim($1))
	)`, user.Email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errEmailAlreadyRegistered
	}
	var userID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO logmaster_api.users
		(feishu_open_id, name, email, role, role_source, identity_type, external_company)
		VALUES ($1, $2, $3, 'user', 'external', 'external', $4)
		RETURNING id`, user.FeishuOpenID, user.Name, user.Email, user.Company).Scan(&userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO logmaster_api.external_password_credentials (user_id, password_hash)
		VALUES ($1, $2)`, userID, passwordHash); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) externalUserByEmail(ctx context.Context, email string) (UserInfo, string, error) {
	var user UserInfo
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `SELECT u.feishu_open_id, u.name, u.email, u.role, u.role_source,
		u.identity_type, u.external_company, c.password_hash
		FROM logmaster_api.users u
		JOIN logmaster_api.external_password_credentials c ON c.user_id = u.id
		WHERE u.identity_type = 'external' AND lower(trim(u.email)) = lower(trim($1))`, email).
		Scan(&user.FeishuOpenID, &user.Name, &user.Email, &user.Role, &user.RoleSource,
			&user.IdentityType, &user.Company, &passwordHash)
	return user, passwordHash, err
}

func validateExternalRegistration(input externalRegisterRequest) (UserInfo, error) {
	name := strings.TrimSpace(input.Name)
	company := strings.TrimSpace(input.Company)
	if name == "" || utf8.RuneCountInString(name) > 128 {
		return UserInfo{}, fmt.Errorf("姓名不能为空且不能超过128个字符")
	}
	if company == "" || utf8.RuneCountInString(company) > 128 {
		return UserInfo{}, fmt.Errorf("外包公司不能为空且不能超过128个字符")
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return UserInfo{}, err
	}
	if input.Password != input.ConfirmPassword {
		return UserInfo{}, fmt.Errorf("两次输入的密码不一致")
	}
	if input.Password == "" {
		return UserInfo{}, fmt.Errorf("密码不能为空")
	}
	return UserInfo{
		FeishuOpenID: "external_" + randomToken(), Name: name, Email: email,
		Role: "user", RoleSource: "external", IdentityType: "external", Company: company,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 320 {
		return "", fmt.Errorf("请输入有效邮箱")
	}
	return email, nil
}

func decodeJSON(r *http.Request, output any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("请求参数无效")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("请求参数无效")
	}
	return nil
}

func methodNotAllowed(w http.ResponseWriter) {
	response.JSONStatus(w, http.StatusMethodNotAllowed, response.APIResponse{Code: 405, Message: "请求方法不支持", Data: nil})
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	response.JSONStatus(w, status, response.APIResponse{Code: status, Message: message, Data: nil})
}
