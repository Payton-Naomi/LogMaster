package auth

type UserInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
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
	} `json:"data"`
}
