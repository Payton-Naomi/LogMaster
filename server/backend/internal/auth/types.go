package auth

type UserInfo struct {
	ID           string `json:"id"`
	FeishuOpenID string `json:"feishu_open_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Avatar       string `json:"avatar"`
	Role         string `json:"role"`
	RoleSource   string `json:"role_source"`
	JobTitle     string `json:"job_title"`
	IdentityType string `json:"identity_type"`
	Company      string `json:"company"`
}

type feishuTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken        string `json:"access_token"`
		UserAccessToken    string `json:"user_access_token"`
		UserAccessTokenAlt string `json:"user_access_token_v2"`
		TokenType          string `json:"token_type"`
		ExpiresIn          int    `json:"expires_in"`
	} `json:"data"`
	AccessToken     string `json:"access_token"`
	UserAccessToken string `json:"user_access_token"`
}

type feishuUserInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		OpenID    string `json:"open_id"`
		UnionID   string `json:"union_id"`
		Name      string `json:"name"`
		EnName    string `json:"en_name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		JobTitle  string `json:"job_title"`
	} `json:"data"`
}
