package web

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func NewSPAHandler(distDir string) (http.Handler, error) {
	root, err := filepath.Abs(distDir)
	if err != nil {
		return nil, fmt.Errorf("resolve frontend directory: %w", err)
	}
	indexPath := filepath.Join(root, "index.html")
	if info, err := os.Stat(indexPath); err != nil || info.IsDir() {
		return nil, fmt.Errorf("frontend build not found at %s; run npm.cmd run build in frontend", indexPath)
	}

	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		assetPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if assetPath != "." && assetPath != "" {
			fullPath := filepath.Join(root, filepath.FromSlash(assetPath))
			if relative, err := filepath.Rel(root, fullPath); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		request := r.Clone(r.Context())
		request.URL = cloneURL(r.URL)
		request.URL.Path = "/"
		fileServer.ServeHTTP(w, request)
	}), nil
}

func cloneURL(value *url.URL) *url.URL {
	copy := *value
	return &copy
}
