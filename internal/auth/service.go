package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"logmaster-agent/internal/config"
)

type Service struct {
	config     config.Config
	httpClient *http.Client
	sessionMu  sync.RWMutex
	sessions   map[string]UserInfo
}

func NewService(cfg config.Config) *Service {
	return &Service{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		sessions:   map[string]UserInfo{},
	}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/feishu-url", s.feishuURLHandler)
	mux.HandleFunc("/api/auth/callback", s.authCallbackHandler)
	mux.HandleFunc("/api/auth/logout", s.logoutHandler)
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
