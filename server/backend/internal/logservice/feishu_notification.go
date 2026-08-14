package logservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type AnalysisNotificationMatch struct {
	Keyword string
	Count   int64
}

type AnalysisNotification struct {
	TaskID          string
	RecipientOpenID string
	ProjectName     string
	Version         string
	OriginalName    string
	TotalLines      int64
	ErrorCount      int64
	WarningCount    int64
	TopMatches      []AnalysisNotificationMatch
	ResultURL       string
}

type AnalysisNotifier interface {
	Notify(context.Context, AnalysisNotification) error
}

type FeishuNotifier struct {
	appID       string
	appSecret   string
	baseURL     string
	httpClient  *http.Client
	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewFeishuNotifier(appID, appSecret string) *FeishuNotifier {
	return newFeishuNotifier(appID, appSecret, "https://open.feishu.cn", &http.Client{Timeout: 10 * time.Second})
}

func newFeishuNotifier(appID, appSecret, baseURL string, client *http.Client) *FeishuNotifier {
	return &FeishuNotifier{
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: client,
	}
}

func (n *FeishuNotifier) Notify(ctx context.Context, notification AnalysisNotification) error {
	token, err := n.tenantAccessToken(ctx)
	if err != nil {
		return err
	}
	content, err := json.Marshal(map[string]string{"text": formatAnalysisNotification(notification)})
	if err != nil {
		return fmt.Errorf("encode Feishu message content: %w", err)
	}
	body, err := json.Marshal(map[string]string{
		"receive_id": notification.RecipientOpenID,
		"msg_type":   "text",
		"content":    string(content),
	})
	if err != nil {
		return fmt.Errorf("encode Feishu message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		n.baseURL+"/open-apis/im/v1/messages?receive_id_type=open_id", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Feishu message request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := n.doJSON(req, &result); err != nil {
		return fmt.Errorf("send Feishu message: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("send Feishu message: code %d: %s", result.Code, result.Msg)
	}
	return nil
}

func (n *FeishuNotifier) tenantAccessToken(ctx context.Context) (string, error) {
	n.tokenMu.Lock()
	defer n.tokenMu.Unlock()
	if n.token != "" && time.Until(n.tokenExpiry) > time.Minute {
		return n.token, nil
	}
	body, err := json.Marshal(map[string]string{"app_id": n.appID, "app_secret": n.appSecret})
	if err != nil {
		return "", fmt.Errorf("encode Feishu token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		n.baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create Feishu token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int64  `json:"expire"`
	}
	if err := n.doJSON(req, &result); err != nil {
		return "", fmt.Errorf("get Feishu tenant access token: %w", err)
	}
	if result.Code != 0 || result.TenantAccessToken == "" {
		return "", fmt.Errorf("get Feishu tenant access token: code %d: %s", result.Code, result.Msg)
	}
	n.token = result.TenantAccessToken
	n.tokenExpiry = time.Now().Add(time.Duration(result.Expire) * time.Second)
	return n.token, nil
}

func (n *FeishuNotifier) doJSON(req *http.Request, target any) error {
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func formatAnalysisNotification(notification AnalysisNotification) string {
	var message strings.Builder
	message.WriteString("日志分析完成\n")
	fmt.Fprintf(&message, "项目：%s\n", notification.ProjectName)
	if notification.Version != "" {
		fmt.Fprintf(&message, "版本：%s\n", notification.Version)
	}
	if notification.OriginalName != "" {
		fmt.Fprintf(&message, "文件：%s\n", notification.OriginalName)
	}
	fmt.Fprintf(&message, "日志总行数：%d\n错误：%d\n警告：%d", notification.TotalLines, notification.ErrorCount, notification.WarningCount)
	if len(notification.TopMatches) > 0 {
		message.WriteString("\n主要命中：")
		for _, match := range notification.TopMatches {
			fmt.Fprintf(&message, "\n- %s：%d", match.Keyword, match.Count)
		}
	}
	if notification.ResultURL != "" {
		fmt.Fprintf(&message, "\n查看完整结果：%s", notification.ResultURL)
	}
	return message.String()
}
