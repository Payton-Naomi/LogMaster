package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"logmaster-agent/internal/auth"
	"logmaster-agent/internal/config"
	"logmaster-agent/internal/stream"
)

//go:embed static
var staticFiles embed.FS

func main() {
	cfg := config.Load()
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))

	authService := auth.NewService(cfg)
	authService.RegisterRoutes(mux)
	stream.RegisterRoutes(mux, authService)

	fmt.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
