package auth

import (
	"fmt"
	"net/http"
	"net/url"

	"logmaster-agent/internal/response"
)

func (s *Service) feishuURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	loginURL, err := s.prepareFeishuLogin(w)
	if err != nil {
		response.JSONStatus(w, http.StatusInternalServerError, response.APIResponse{
			Code:    500,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]string{"url": loginURL}})
}

func (s *Service) feishuLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	loginURL, err := s.prepareFeishuLogin(w)
	if err != nil {
		response.JSONStatus(w, http.StatusInternalServerError, response.APIResponse{Code: 500, Message: err.Error(), Data: nil})
		return
	}
	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (s *Service) prepareFeishuLogin(w http.ResponseWriter) (string, error) {
	if s.config.FeishuAppID == "" || s.config.FeishuAppSecret == "" {
		return "", fmt.Errorf("FEISHU_APP_ID or FEISHU_APP_SECRET is not configured")
	}

	state := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     "feishu_oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := "https://accounts.feishu.cn/open-apis/authen/v1/authorize"
	params := url.Values{}
	params.Set("app_id", s.config.FeishuAppID)
	params.Set("redirect_uri", s.config.FeishuRedirectURI)
	params.Set("state", state)
	return authURL + "?" + params.Encode(), nil
}

func (s *Service) authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie("feishu_oauth_state")
	if err != nil || state == "" || stateCookie.Value != state {
		s.recordRuntimeLog(r.Context(), "", "validate Feishu OAuth state", "failed", "invalid oauth state")
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	userAccessToken, err := s.exchangeFeishuCode(code)
	if err != nil {
		s.recordRuntimeLog(r.Context(), "", "exchange Feishu authorization code", "failed", err.Error())
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	user, err := s.fetchFeishuUserInfo(userAccessToken)
	if err != nil {
		s.recordRuntimeLog(r.Context(), "", "fetch Feishu user info", "failed", err.Error())
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	role, err := s.saveUser(r.Context(), user)
	if err != nil {
		s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "save Feishu user mapping", "failed", err.Error())
		http.Error(w, "save user mapping failed", http.StatusInternalServerError)
		return
	}
	user.Role = role

	s.saveSession(w, user)
	s.recordRuntimeLog(r.Context(), user.FeishuOpenID, "Feishu login", "success", "login succeeded")
	clearOAuthStateCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Service) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.deleteSession(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
}

func (s *Service) userInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, ok := s.CurrentUser(r)
	if !ok {
		response.JSONStatus(w, http.StatusUnauthorized, response.APIResponse{
			Code:    401,
			Message: "please login first",
			Data:    nil,
		})
		return
	}
	role, err := s.userRole(r.Context(), user.FeishuOpenID)
	if err != nil {
		response.JSONStatus(w, http.StatusInternalServerError, response.APIResponse{Code: 500, Message: "query user role failed", Data: nil})
		return
	}
	user.Role = role
	_ = s.db.QueryRowContext(r.Context(), `SELECT role_source, job_title, identity_type, external_company
		FROM logmaster_api.users WHERE feishu_open_id = $1`, user.FeishuOpenID).
		Scan(&user.RoleSource, &user.JobTitle, &user.IdentityType, &user.Company)

	response.JSON(w, response.APIResponse{
		Code:    0,
		Message: "success",
		Data:    user,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "feishu_oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
