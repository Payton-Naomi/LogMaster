package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	FeishuAppID           string
	FeishuAppSecret       string
	FeishuRedirectURI     string
	DatabaseURL           string
	StorageDir            string
	MaxUploadBytes        int64
	MaxFilesPerUpload     int
	MaxExtractBytes       int64
	AgentAnalysisURL      string
	AgentAnalysisToken    string
	AgentAnalysisTimeout  time.Duration
	LLMAPIBaseURL         string
	LLMAPIKey             string
	LLMModel              string
	LLMTimeout            time.Duration
	LLMMaxMatches         int
	LLMMaxInputBytes      int
	AIMaxTokensPerFile    int
	AIDailyTokenQuota     int64
	ConfigEncryptionKey   string
	FrontendDistDir       string
	PublicBaseURL         string
	FeishuRoleTitleRules  string
	FeishuSuperAdminIDs   string
	FeishuSuperAdminNames string
	UploadToken           string
	UploadOwnerOpenID     string
}

func Load() Config {
	redirectURI := os.Getenv("FEISHU_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/auth/callback"
	}

	return Config{
		FeishuAppID:           firstNonEmpty(os.Getenv("FEISHU_APP_ID"), "cli_aac4efb073789bd0"),
		FeishuAppSecret:       os.Getenv("FEISHU_APP_SECRET"),
		FeishuRedirectURI:     redirectURI,
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		StorageDir:            firstNonEmpty(os.Getenv("LOG_STORAGE_DIR"), "data/logs"),
		MaxUploadBytes:        envInt64("MAX_UPLOAD_BYTES", 2<<30),
		MaxFilesPerUpload:     envInt("MAX_FILES_PER_UPLOAD", 100),
		MaxExtractBytes:       envInt64("MAX_EXTRACT_BYTES", 8<<30),
		AgentAnalysisURL:      os.Getenv("AGENT_ANALYSIS_URL"),
		AgentAnalysisToken:    os.Getenv("AGENT_ANALYSIS_TOKEN"),
		AgentAnalysisTimeout:  time.Duration(envInt64("AGENT_ANALYSIS_TIMEOUT_SECONDS", 60)) * time.Second,
		LLMAPIBaseURL:         os.Getenv("LLM_API_BASE_URL"),
		LLMAPIKey:             os.Getenv("LLM_API_KEY"),
		LLMModel:              firstNonEmpty(os.Getenv("LLM_MODEL"), "qwen-plus"),
		LLMTimeout:            time.Duration(envInt64("LLM_TIMEOUT_SECONDS", 120)) * time.Second,
		LLMMaxMatches:         envInt("LLM_MAX_MATCHES", 50),
		LLMMaxInputBytes:      envInt("LLM_MAX_INPUT_BYTES", 200000),
		AIMaxTokensPerFile:    envInt("AI_MAX_TOKENS_PER_FILE", 20000),
		AIDailyTokenQuota:     envInt64("AI_DAILY_TOKEN_QUOTA", 1000000),
		ConfigEncryptionKey:   os.Getenv("LOGMASTER_CONFIG_ENCRYPTION_KEY"),
		FrontendDistDir:       firstNonEmpty(os.Getenv("FRONTEND_DIST_DIR"), "frontend/dist"),
		PublicBaseURL:         strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
		FeishuRoleTitleRules:  os.Getenv("FEISHU_ROLE_TITLE_RULES"),
		FeishuSuperAdminIDs:   os.Getenv("FEISHU_SUPER_ADMIN_OPEN_IDS"),
		FeishuSuperAdminNames: firstNonEmpty(os.Getenv("FEISHU_SUPER_ADMIN_NAMES"), "刘欣彤"),
		UploadToken:           os.Getenv("LOGMASTER_UPLOAD_TOKEN"),
		UploadOwnerOpenID:     os.Getenv("LOGMASTER_UPLOAD_OWNER_OPEN_ID"),
	}
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	var parsed int
	if _, err := fmt.Sscan(value, &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt64(name string, fallback int64) int64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	var parsed int64
	if _, err := fmt.Sscan(value, &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
