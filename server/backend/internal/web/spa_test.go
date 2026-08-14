package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandler(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<main>app</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "app.js"), []byte("console.log('app')"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewSPAHandler(directory)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		path, contains, cacheControl string
	}{
		{"/", "<main>app</main>", "no-cache"},
		{"/tasks/123", "<main>app</main>", "no-cache"},
		{"/app.js", "console.log", ""},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("%s: status=%d body=%q", test.path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != test.cacheControl {
			t.Fatalf("%s: Cache-Control=%q", test.path, response.Header().Get("Cache-Control"))
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/unknown", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("API fallback status = %d", response.Code)
	}
}
