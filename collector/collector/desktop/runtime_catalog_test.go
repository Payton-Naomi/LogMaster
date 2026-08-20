package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeCatalogFilesAreCreatedAndReloaded(t *testing.T) {
	root := t.TempDir()
	catalog, paths, err := loadRuntimeCatalog(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) == 0 {
		t.Fatal("expected packaged projects")
	}
	for _, path := range []string{paths.Projects, paths.Tasks, paths.Keywords} {
		if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
			t.Fatalf("runtime catalog file missing: %s", path)
		}
	}
	keywords, err := os.ReadFile(paths.Keywords)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(keywords, []byte("text:")) || bytes.Contains(keywords, []byte("mode: contains")) || bytes.Contains(keywords, []byte("case_sensitive: false")) {
		t.Fatalf("keyword config was not simplified: %s", keywords[:min(len(keywords), 300)])
	}
	projects := []byte("schema_version: 1\nprojects:\n  - id: local-project\n    name: 本地项目\n    versions: [V9]\n")
	if err := os.WriteFile(filepath.Join(paths.Directory, "project-config.yaml"), projects, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := loadRuntimeCatalog(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].ID != "local-project" {
		t.Fatalf("local project file was not reloaded: %#v", loaded.Projects)
	}
}
