package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"logmaster-agent/internal/admin"
	"logmaster-agent/internal/auth"
	"logmaster-agent/internal/config"
	"logmaster-agent/internal/database"
	"logmaster-agent/internal/logservice"
	"logmaster-agent/internal/response"
	"logmaster-agent/internal/web"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required; set it to your PostgreSQL connection string")
	}
	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]string{"status": "ok"}})
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/health", healthHandler)

	authService := auth.NewService(cfg, db)
	authService.RegisterRoutes(mux)
	adminService := admin.NewService(cfg, db)
	adminService.SetCurrentUserResolver(func(r *http.Request) (string, bool) {
		user, ok := authService.CurrentUser(r)
		return user.FeishuOpenID, ok
	})
	adminService.RegisterRoutes(mux)
	logService := logservice.NewService(cfg, logservice.NewRepository(db))
	if cfg.UploadToken != "" && cfg.UploadOwnerOpenID == "" {
		log.Print("configured collector upload token disabled: LOGMASTER_UPLOAD_OWNER_OPEN_ID is required; built-in internal collector remains enabled")
	}
	logService.SetUploadAuthenticator(cfg.UploadToken, cfg.UploadOwnerOpenID)
	logService.SetCurrentUserResolver(func(r *http.Request) (string, bool) {
		user, ok := authService.CurrentUser(r)
		return user.FeishuOpenID, ok
	})
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		logService.SetAnalysisNotifier(logservice.NewFeishuNotifier(cfg.FeishuAppID, cfg.FeishuAppSecret))
	} else {
		log.Print("Feishu analysis notifications disabled: FEISHU_APP_ID or FEISHU_APP_SECRET is empty")
	}
	logService.RegisterRoutes(mux)
	frontendHandler, err := web.NewSPAHandler(cfg.FrontendDistDir)
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", frontendHandler)

	fmt.Println("LogMaster running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
