package logservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type collectorIdentity struct {
	OpenID string
	Name   string
	Email  string
}

type feishuDirectory struct {
	appID       string
	appSecret   string
	baseURL     string
	httpClient  *http.Client
	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

func newFeishuDirectory(appID, appSecret string) *feishuDirectory {
	return &feishuDirectory{
		appID: appID, appSecret: appSecret, baseURL: "https://open.feishu.cn",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *feishuDirectory) identityByEmail(ctx context.Context, email string) (collectorIdentity, error) {
	token, err := d.tenantAccessToken(ctx)
	if err != nil {
		return collectorIdentity{}, err
	}
	body, err := json.Marshal(map[string]any{"emails": []string{email}, "include_resigned": false})
	if err != nil {
		return collectorIdentity{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/open-apis/contact/v3/users/batch_get_id?user_id_type=open_id", bytes.NewReader(body))
	if err != nil {
		return collectorIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	var lookup struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Users []struct {
				UserID string `json:"user_id"`
				Email  string `json:"email"`
				Status struct {
					IsFrozen    bool `json:"is_frozen"`
					IsResigned  bool `json:"is_resigned"`
					IsActivated bool `json:"is_activated"`
					IsExited    bool `json:"is_exited"`
					IsUnjoin    bool `json:"is_unjoin"`
				} `json:"status"`
			} `json:"user_list"`
		} `json:"data"`
	}
	if err := d.doJSON(req, &lookup); err != nil {
		return collectorIdentity{}, fmt.Errorf("query Feishu user by email: %w", err)
	}
	if lookup.Code != 0 {
		return collectorIdentity{}, fmt.Errorf("query Feishu user by email: code %d: %s", lookup.Code, lookup.Msg)
	}
	if len(lookup.Data.Users) != 1 {
		return collectorIdentity{}, ErrUploaderEmailNotInternal
	}
	member := lookup.Data.Users[0]
	if member.UserID == "" || member.Status.IsFrozen || member.Status.IsResigned || member.Status.IsExited || member.Status.IsUnjoin || !member.Status.IsActivated {
		return collectorIdentity{}, ErrUploaderEmailNotInternal
	}

	detailURL := d.baseURL + "/open-apis/contact/v3/users/" + url.PathEscape(member.UserID) + "?user_id_type=open_id"
	detailReq, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return collectorIdentity{}, err
	}
	detailReq.Header.Set("Authorization", "Bearer "+token)
	var detail struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			User struct {
				OpenID string `json:"open_id"`
				Name   string `json:"name"`
				Email  string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := d.doJSON(detailReq, &detail); err != nil {
		return collectorIdentity{}, fmt.Errorf("get Feishu user detail: %w", err)
	}
	if detail.Code != 0 || strings.TrimSpace(detail.Data.User.Name) == "" {
		return collectorIdentity{}, fmt.Errorf("get Feishu user detail: code %d: %s", detail.Code, detail.Msg)
	}
	resolvedEmail := strings.ToLower(strings.TrimSpace(detail.Data.User.Email))
	if resolvedEmail == "" {
		resolvedEmail = strings.ToLower(strings.TrimSpace(member.Email))
	}
	if resolvedEmail != strings.ToLower(strings.TrimSpace(email)) {
		return collectorIdentity{}, ErrUploaderEmailNotInternal
	}
	return collectorIdentity{OpenID: firstNonEmptyValue(detail.Data.User.OpenID, member.UserID), Name: strings.TrimSpace(detail.Data.User.Name), Email: resolvedEmail}, nil
}

func (d *feishuDirectory) tenantAccessToken(ctx context.Context) (string, error) {
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()
	if d.token != "" && time.Until(d.tokenExpiry) > time.Minute {
		return d.token, nil
	}
	body, _ := json.Marshal(map[string]string{"app_id": d.appID, "app_secret": d.appSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	var result struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Token  string `json:"tenant_access_token"`
		Expire int64  `json:"expire"`
	}
	if err := d.doJSON(req, &result); err != nil {
		return "", fmt.Errorf("get Feishu tenant access token: %w", err)
	}
	if result.Code != 0 || result.Token == "" {
		return "", fmt.Errorf("get Feishu tenant access token: code %d: %s", result.Code, result.Msg)
	}
	d.token, d.tokenExpiry = result.Token, time.Now().Add(time.Duration(result.Expire)*time.Second)
	return d.token, nil
}

func (d *feishuDirectory) doJSON(req *http.Request, target any) error {
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
