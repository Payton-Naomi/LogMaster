package auth

import (
	"net/http"
	"net/url"

	"logmaster-agent/internal/response"
)

func (s *Service) feishuURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.config.FeishuAppID == "" || s.config.FeishuAppSecret == "" {
		response.JSONStatus(w, http.StatusInternalServerError, response.APIResponse{
			Code:    500,
			Message: "FEISHU_APP_ID or FEISHU_APP_SECRET is not configured",
			Data:    nil,
		})
		return
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

	response.JSON(w, response.APIResponse{
		Code:    0,
		Message: "success",
		Data: map[string]string{
			"url": authURL + "?" + params.Encode(),
		},
	})
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
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	userAccessToken, err := s.exchangeFeishuCode(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	user, err := s.fetchFeishuUserInfo(userAccessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	s.saveSession(w, user)
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
