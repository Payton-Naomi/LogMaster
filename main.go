package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"logmaster-agent/internal/auth"
	"logmaster-agent/internal/config"
	"logmaster-agent/internal/database"
	logsapi "logmaster-agent/internal/logs"
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

	authService := auth.NewService(cfg)
	authService.RegisterRoutes(mux)
	logService := logsapi.NewService(cfg, logsapi.NewRepository(db))
	logService.RegisterRoutes(mux)
	frontendHandler, err := web.NewSPAHandler(cfg.FrontendDistDir)
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", frontendHandler)

	fmt.Println("LogMaster running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
