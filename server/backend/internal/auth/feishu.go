package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const feishuResponseLimit = 1 << 20

func (s *Service) exchangeFeishuCode(code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("missing authorization code")
	}

	body := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     s.config.FeishuAppID,
		"client_secret": s.config.FeishuAppSecret,
		"code":          code,
		"redirect_uri":  s.config.FeishuRedirectURI,
	}

	rawBody, err := s.postFeishuJSONRaw("https://open.feishu.cn/open-apis/authen/v2/oauth/token", body)
	if err != nil {
		return "", err
	}

	var tokenResponse feishuTokenResponse
	if err := json.Unmarshal(rawBody, &tokenResponse); err != nil {
		return "", fmt.Errorf("decode feishu token response failed: %w", err)
	}

	if tokenResponse.Code != 0 {
		return "", fmt.Errorf("feishu token error: %s", safeFeishuMessage(tokenResponse.Msg))
	}

	accessToken := firstNonEmpty(
		tokenResponse.Data.UserAccessToken,
		tokenResponse.Data.UserAccessTokenAlt,
		tokenResponse.Data.AccessToken,
		tokenResponse.UserAccessToken,
		tokenResponse.AccessToken,
		tokenFromGenericJSON(rawBody, "user_access_token"),
		tokenFromGenericJSON(rawBody, "access_token"),
	)
	if accessToken == "" {
		return "", fmt.Errorf("feishu token error: user access token is empty")
	}

	return accessToken, nil
}

func (s *Service) fetchFeishuUserInfo(accessToken string) (UserInfo, error) {
	request, err := http.NewRequest(http.MethodGet, "https://open.feishu.cn/open-apis/authen/v1/user_info", nil)
	if err != nil {
		return UserInfo{}, err
	}

	request.Header.Set("Authorization", "Bearer "+accessToken)

	response, err := s.httpClient.Do(request)
	if err != nil {
		return UserInfo{}, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, feishuResponseLimit))
	if err != nil {
		return UserInfo{}, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UserInfo{}, fmt.Errorf("feishu user info http status %d", response.StatusCode)
	}

	var userResponse feishuUserInfoResponse
	if err := json.Unmarshal(responseBody, &userResponse); err != nil {
		return UserInfo{}, err
	}

	if userResponse.Code != 0 {
		return UserInfo{}, fmt.Errorf("feishu user info error: %s", userResponse.Msg)
	}

	name := firstNonEmpty(userResponse.Data.Name, userResponse.Data.EnName, "Feishu User")
	avatar := ""
	if name != "" {
		avatar = string([]rune(name)[0])
	}

	return UserInfo{
		ID:           firstNonEmpty(userResponse.Data.OpenID, userResponse.Data.UnionID),
		FeishuOpenID: userResponse.Data.OpenID,
		Name:         name,
		Email:        userResponse.Data.Email,
		Avatar:       avatar,
		JobTitle:     userResponse.Data.JobTitle,
	}, nil
}

func (s *Service) postFeishuJSONRaw(endpoint string, requestBody interface{}) ([]byte, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(response.Body, feishuResponseLimit))
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("feishu http status %d", response.StatusCode)
	}

	return rawBody, nil
}

func safeFeishuMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "unknown error"
	}
	// Feishu normally returns a short human-readable message. Keep logs useful
	// while preventing accidental token-sized or multiline response injection.
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 256 {
		message = message[:256]
	}
	return message
}

func tokenFromGenericJSON(rawBody []byte, key string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return ""
	}

	return findStringValue(payload, key)
}

func findStringValue(value interface{}, key string) string {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		if directValue, ok := typedValue[key].(string); ok {
			return directValue
		}

		for _, child := range typedValue {
			if found := findStringValue(child, key); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typedValue {
			if found := findStringValue(child, key); found != "" {
				return found
			}
		}
	}

	return ""
}
