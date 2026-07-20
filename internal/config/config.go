package config

import "os"

type Config struct {
	FeishuAppID       string
	FeishuAppSecret   string
	FeishuRedirectURI string
}

func Load() Config {
	redirectURI := os.Getenv("FEISHU_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/callback"
	}

	return Config{
		FeishuAppID:       firstNonEmpty(os.Getenv("FEISHU_APP_ID"), "cli_aac4efb073789bd0"),
		FeishuAppSecret:   os.Getenv("FEISHU_APP_SECRET"),
		FeishuRedirectURI: redirectURI,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
