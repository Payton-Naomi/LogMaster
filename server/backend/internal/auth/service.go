package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"logmaster-agent/internal/config"
	"logmaster-agent/internal/rolepolicy"
)

type Service struct {
	config     config.Config
	db         *sql.DB
	httpClient *http.Client
	sessionMu  sync.RWMutex
	sessions   map[string]UserInfo
}

func NewService(cfg config.Config, db *sql.DB) *Service {
	return &Service{
		config:     cfg,
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		sessions:   map[string]UserInfo{},
	}
}

func (s *Service) saveUser(ctx context.Context, user UserInfo) (string, error) {
	openID := strings.TrimSpace(user.FeishuOpenID)
	if openID == "" {
		return "", fmt.Errorf("feishu open id is empty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(4904437574657255)`); err != nil {
		return "", err
	}
	var hasSuperAdmin bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM logmaster_api.users WHERE role = 'super_admin'
	)`).Scan(&hasSuperAdmin); err != nil {
		return "", err
	}
	var role, roleSource string
	automaticRole := s.roleForFeishuUser(openID, user.Name, user.JobTitle)
	err = tx.QueryRowContext(ctx, `INSERT INTO logmaster_api.users (feishu_open_id, name, email, job_title, role, role_source)
		VALUES ($1, $2, $3, $4, $5, 'feishu')
		ON CONFLICT (feishu_open_id)
		DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			job_title = EXCLUDED.job_title,
			role = CASE WHEN logmaster_api.users.role_source = 'feishu' THEN EXCLUDED.role ELSE logmaster_api.users.role END,
			updated_at = NOW()
		RETURNING role, role_source`, openID, user.Name, user.Email, user.JobTitle, automaticRole).Scan(&role, &roleSource)
	if err != nil {
		return "", err
	}
	// An explicit name list always wins for matching Feishu users. When the list
	// is empty, bootstrap only the first user while no super admin exists.
	if s.isConfiguredSuperAdmin(user.FeishuOpenID, user.Name) || (strings.TrimSpace(s.config.FeishuSuperAdminNames) == "" && strings.TrimSpace(s.config.FeishuSuperAdminIDs) == "" && !hasSuperAdmin) {
		if err := tx.QueryRowContext(ctx, `UPDATE logmaster_api.users
			SET role = 'super_admin', role_source = 'manual', updated_at = NOW()
			WHERE feishu_open_id = $1
			RETURNING role, role_source`, openID).Scan(&role, &roleSource); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	user.RoleSource = roleSource
	return role, nil
}

func (s *Service) roleForJobTitle(jobTitle string) string {
	return rolepolicy.ForJobTitle(jobTitle)
}

func (s *Service) roleForFeishuUser(openID, name, jobTitle string) string {
	return s.roleForJobTitle(jobTitle)
}

func (s *Service) userRole(ctx context.Context, openID string) (string, error) {
	var role string
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM logmaster_api.users WHERE feishu_open_id = $1`, openID).Scan(&role); err != nil {
		return "", err
	}
	return role, nil
}

func (s *Service) isConfiguredSuperAdminName(name string) bool {
	for _, configuredName := range strings.Split(s.config.FeishuSuperAdminNames, ",") {
		if strings.TrimSpace(configuredName) != "" && strings.TrimSpace(configuredName) == strings.TrimSpace(name) {
			return true
		}
	}
	return false
}

func (s *Service) isConfiguredSuperAdmin(openID, name string) bool {
	for _, configuredID := range strings.Split(s.config.FeishuSuperAdminIDs, ",") {
		if strings.TrimSpace(configuredID) != "" && strings.TrimSpace(configuredID) == strings.TrimSpace(openID) {
			return true
		}
	}
	return s.isConfiguredSuperAdminName(name)
}

// UserRoleResolver is intentionally read-only. Admin permission checks must
// never bootstrap or modify roles as a side effect of a GET request.
func (s *Service) UserRoleResolver(ctx context.Context, openID string) (string, error) {
	return s.userRole(ctx, openID)
}

func (s *Service) recordRuntimeLog(ctx context.Context, ownerOpenID, event, status, message string) {
	_, _ = s.db.ExecContext(ctx, `INSERT INTO logmaster_api.runtime_logs
		(owner_open_id, module, event, status, message)
		VALUES ($1, 'auth', $2, $3, $4)`, ownerOpenID, event, status, message)
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/feishu-url", s.feishuURLHandler)
	mux.HandleFunc("/api/auth/feishu-login", s.feishuLoginHandler)
	mux.HandleFunc("/api/auth/callback", s.authCallbackHandler)
	mux.HandleFunc("/api/auth/external/register", s.externalRegisterHandler)
	mux.HandleFunc("/api/auth/external/login", s.externalLoginHandler)
	mux.HandleFunc("/api/auth/external/change-password", s.externalChangePasswordHandler)
	mux.HandleFunc("/api/auth/external/change-email", s.externalChangeEmailHandler)
	mux.HandleFunc("/api/auth/logout", s.logoutHandler)
	mux.HandleFunc("/api/auth/me", s.userInfoHandler)
	mux.HandleFunc("/api/user/info", s.userInfoHandler)
}

func (s *Service) CurrentUser(r *http.Request) (UserInfo, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return UserInfo{}, false
	}

	s.sessionMu.RLock()
	user, ok := s.sessions[cookie.Value]
	s.sessionMu.RUnlock()
	return user, ok
}

func (s *Service) saveSession(w http.ResponseWriter, user UserInfo) {
	sessionToken := randomToken()
	s.sessionMu.Lock()
	s.sessions[sessionToken] = user
	s.sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) deleteSession(r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return
	}

	s.sessionMu.Lock()
	delete(s.sessions, cookie.Value)
	s.sessionMu.Unlock()
}

func randomToken() string {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return base64.RawURLEncoding.EncodeToString(buffer)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
