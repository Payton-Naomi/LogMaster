package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"logmaster-agent/agent/internal/backend"
)

func TestSyncCloudConfigOverwritesThreeEditableFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/collector/sync" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "success", "data": map[string]any{
			"projects":  []map[string]any{{"id": "42", "name": "DR2860"}},
			"scenarios": []map[string]any{{"id": "aging", "name": "普通挂测", "enabled": true}},
			"keywords":  []map[string]any{{"id": 7, "name": "断连", "category": "连接", "keyword": "disconnect", "scope": "line", "level": "warning"}},
			"synced_at": "2026-08-26T00:00:00Z",
		}})
	}))
	defer server.Close()
	root := t.TempDir()
	service, err := newServiceAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	service.backendClient = backend.New(backend.Config{BaseURL: server.URL + "/api", Timeout: time.Second})
	result, err := service.SyncCloudConfig()
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("result = %+v", result)
	}
	paths := catalogFilesForRoot(root)
	projects, _ := os.ReadFile(paths.Projects)
	tasks, _ := os.ReadFile(paths.Tasks)
	keywords, _ := os.ReadFile(paths.Keywords)
	catalog, err := parseMaintainedCatalog(projects, tasks, keywords)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) != 1 || catalog.Projects[0].ID != "42" {
		t.Fatalf("projects = %+v", catalog.Projects)
	}
	if len(catalog.Projects[0].Tasks) != 1 || catalog.Projects[0].Tasks[0].ID != "aging" {
		t.Fatalf("tasks = %+v", catalog.Projects[0].Tasks)
	}
	if string(keywords) == "" || filepath.Base(paths.Keywords) != "keyword-config.yaml" {
		t.Fatal("keyword file not written")
	}
	modified := strings.Replace(string(keywords), "disconnect", "disconnect-local", 1)
	updated, err := service.SaveEditableConfigFile("keyword-config.yaml", modified)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range catalogKeywordRules(updated.Projects[0].Tasks[0].KeywordProfiles[0]) {
		if rule.Match == "disconnect-local" {
			found = true
		}
	}
	if !found {
		t.Fatal("manual keyword edit was overwritten by cloud cache")
	}
}
