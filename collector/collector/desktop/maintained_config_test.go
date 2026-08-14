package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaintainedProjectCatalogParses(t *testing.T) {
	base := filepath.Join("..", "configs", "logmaster")
	projects, err := os.ReadFile(filepath.Join(base, "projects.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := os.ReadFile(filepath.Join(base, "tasks.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	keywords, err := os.ReadFile(filepath.Join(base, "keywords.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := parseMaintainedCatalog(projects, tasks, keywords)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Projects) != 43 {
		t.Fatalf("project count = %d", len(catalog.Projects))
	}
	if len(catalog.Projects[0].Tasks) != 14 {
		t.Fatalf("recorder task count = %d", len(catalog.Projects[0].Tasks))
	}
	profiles := catalog.Projects[0].Tasks[0].KeywordProfiles
	if len(profiles) != 1 || len(profiles[0].Groups) != 31 || len(catalogKeywordRules(profiles[0])) != 80 {
		t.Fatalf("recorder keyword profiles = %+v", profiles)
	}
	var systemMode, rearCamera *CatalogKeywordGroup
	for i := range profiles[0].Groups {
		group := &profiles[0].Groups[i]
		if group.ID == "system-mode" {
			systemMode = group
		}
		if group.ID == "rear-camera" {
			rearCamera = group
		}
	}
	if systemMode == nil || len(systemMode.Rules) != 7 || rearCamera == nil || len(rearCamera.Rules) != 1 || rearCamera.Rules[0].Match != "AHD" {
		t.Fatalf("keyword hierarchy not expanded: system=%+v rear=%+v", systemMode, rearCamera)
	}
	if len(catalog.Projects[39].Tasks[0].KeywordProfiles) != 1 {
		t.Fatal("项目、任务和关键字方案应保持独立可选")
	}
}

func TestMaintainedDesktopSettingsParse(t *testing.T) {
	root := t.TempDir()
	settings := defaultAppSettings(root)
	if filepath.Clean(packagedDefaults.Log.Directory) != filepath.Clean("D:/LogMaster/LocalLog") || settings.MaxLogLines != 100000 || settings.ProgramVersion != "0.0.10" {
		t.Fatalf("unexpected maintained settings: %+v", settings)
	}
	if filepath.Clean(settings.DefaultLogDirectory) != filepath.Join(root, "data", "spool") {
		t.Fatalf("temporary runtime must keep logs under its own root: %s", settings.DefaultLogDirectory)
	}
}
