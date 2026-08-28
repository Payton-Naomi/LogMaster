package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	logmasterconfig "logmaster-agent/agent/configs/logmaster"
)

// runtimeCatalogFiles keeps the editable catalog separate from the legacy
// project-catalog.yaml import file and the cloud keyword cache.
type runtimeCatalogFiles struct {
	Directory string
	Projects  string
	Tasks     string
	Keywords  string
}

type CatalogFileDTO struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Exists     bool      `json:"exists"`
	SizeBytes  int64     `json:"sizeBytes"`
	ModifiedAt time.Time `json:"modifiedAt,omitempty"`
	Editable   bool      `json:"editable"`
}

type CatalogFilesDTO struct {
	Directory  string           `json:"directory"`
	Files      []CatalogFileDTO `json:"files"`
	CloudCache CatalogFileDTO   `json:"cloudCache"`
}

type EditableConfigFileDTO struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func catalogFilesForRoot(root string) runtimeCatalogFiles {
	directory := editableConfigDirectory(root)
	return runtimeCatalogFiles{
		Directory: directory,
		Projects:  filepath.Join(directory, "project-config.yaml"),
		Tasks:     filepath.Join(directory, "task-config.yaml"),
		Keywords:  filepath.Join(directory, "keyword-config.yaml"),
	}
}

func editableConfigDirectory(root string) string {
	if explicit := strings.TrimSpace(os.Getenv("LOGMASTER_CONFIG_DIR")); explicit != "" {
		return filepath.Clean(explicit)
	}
	// Installed builds are portable. Use the executable directory even before
	// config exists so the first launch creates one beside the executable.
	if base, err := os.UserConfigDir(); err == nil && strings.EqualFold(filepath.Clean(root), filepath.Join(base, "LogMaster")) {
		if executable, executableErr := os.Executable(); executableErr == nil {
			return filepath.Join(filepath.Dir(executable), "config")
		}
	}
	return filepath.Join(root, "config")
}

func ensureRuntimeCatalogFiles(paths runtimeCatalogFiles) error {
	if err := os.MkdirAll(paths.Directory, 0o755); err != nil {
		return err
	}
	compactKeywords, err := compactMaintainedKeywords(logmasterconfig.KeywordsYAML)
	if err != nil {
		return fmt.Errorf("生成精简关键字配置: %w", err)
	}
	defaults := []struct {
		path string
		data []byte
	}{
		{paths.Projects, logmasterconfig.ProjectsYAML},
		{paths.Tasks, logmasterconfig.TasksYAML},
		{paths.Keywords, compactKeywords},
	}
	for _, item := range defaults {
		if _, err := os.Stat(item.path); err == nil {
			if item.path == paths.Keywords {
				current, readErr := os.ReadFile(item.path)
				if readErr == nil && bytes.Equal(bytes.TrimSpace(current), bytes.TrimSpace(logmasterconfig.KeywordsYAML)) {
					if writeErr := atomicWriteFile(item.path, compactKeywords, 0o644); writeErr != nil {
						return fmt.Errorf("迁移精简关键字配置: %w", writeErr)
					}
				}
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := atomicWriteFile(item.path, item.data, 0o644); err != nil {
			return fmt.Errorf("创建本地配置 %s: %w", filepath.Base(item.path), err)
		}
	}
	return nil
}

func exportDefaultConfigDirectory(directory string) error {
	paths := runtimeCatalogFiles{Directory: directory, Projects: filepath.Join(directory, "project-config.yaml"), Tasks: filepath.Join(directory, "task-config.yaml"), Keywords: filepath.Join(directory, "keyword-config.yaml")}
	if err := ensureRuntimeCatalogFiles(paths); err != nil {
		return err
	}
	settings := defaultAppSettings(filepath.Dir(directory))
	settings.DefaultLogDirectory = filepath.Clean(packagedDefaults.Log.Directory)
	normalizeAppSettings(&settings, filepath.Dir(directory))
	return writeAppSettings(filepath.Join(directory, "settings-config.json"), settings)
}

func loadRuntimeCatalog(root, legacyPath string) (CatalogConfig, runtimeCatalogFiles, error) {
	paths := catalogFilesForRoot(root)
	if err := ensureRuntimeCatalogFiles(paths); err != nil {
		return CatalogConfig{}, paths, err
	}
	projects, err := os.ReadFile(paths.Projects)
	if err != nil {
		return CatalogConfig{}, paths, err
	}
	tasks, err := os.ReadFile(paths.Tasks)
	if err != nil {
		return CatalogConfig{}, paths, err
	}
	keywords, err := os.ReadFile(paths.Keywords)
	if err != nil {
		return CatalogConfig{}, paths, err
	}
	catalog, err := parseMaintainedCatalog(projects, tasks, keywords)
	if err != nil {
		// A hand-edited file must not prevent the collector from starting. Keep
		// the user's file for repair and restore only that file from the package.
		compactKeywords, compactErr := compactMaintainedKeywords(logmasterconfig.KeywordsYAML)
		if compactErr != nil {
			compactKeywords = logmasterconfig.KeywordsYAML
		}
		for _, item := range []struct {
			path string
			data []byte
		}{
			{paths.Projects, logmasterconfig.ProjectsYAML},
			{paths.Tasks, logmasterconfig.TasksYAML},
			{paths.Keywords, compactKeywords},
		} {
			if _, readErr := os.ReadFile(item.path); readErr == nil {
				broken := item.path + ".broken-" + time.Now().Format("20060102-150405")
				_ = os.Rename(item.path, broken)
			}
			_ = atomicWriteFile(item.path, item.data, 0o644)
		}
		catalog, err = parseMaintainedCatalog(logmasterconfig.ProjectsYAML, logmasterconfig.TasksYAML, logmasterconfig.KeywordsYAML)
		if err != nil {
			return CatalogConfig{}, paths, fmt.Errorf("恢复内置配置失败: %w", err)
		}
	}
	if strings.TrimSpace(legacyPath) != "" {
		if data, readErr := os.ReadFile(legacyPath); readErr == nil {
			if overlay, parseErr := parseCatalog(data); parseErr == nil {
				catalog = mergeCatalog(catalog, overlay)
			}
		}
	}
	return catalog, paths, nil
}

func catalogFileDTO(path string, editable bool) CatalogFileDTO {
	result := CatalogFileDTO{Name: filepath.Base(path), Path: path, Editable: editable}
	if info, err := os.Stat(path); err == nil {
		result.Exists, result.SizeBytes, result.ModifiedAt = true, info.Size(), info.ModTime()
	}
	return result
}

func (s *Service) GetCatalogFiles() CatalogFilesDTO {
	paths := catalogFilesForRoot(s.rootDirectory)
	return CatalogFilesDTO{Directory: paths.Directory, Files: []CatalogFileDTO{
		catalogFileDTO(paths.Projects, true), catalogFileDTO(paths.Tasks, true), catalogFileDTO(paths.Keywords, true), catalogFileDTO(s.settingsPath, true),
	}, CloudCache: catalogFileDTO(s.cloudKeywordPath, false)}
}

func (s *Service) OpenCatalogDirectory() error {
	return s.OpenLogFolder(s.catalogDirectory)
}

func (s *Service) ReloadCatalogFiles() (CatalogConfig, error) {
	catalog, _, err := loadRuntimeCatalog(s.rootDirectory, s.catalogPath)
	if err != nil {
		return CatalogConfig{}, err
	}
	if cache, cacheErr := readCloudKeywordCache(s.cloudKeywordPath); cacheErr == nil {
		if !hasCloudKeywordProfile(catalog) {
			catalog = mergeCloudKeywords(catalog, cache.Items)
		}
	}
	s.mu.Lock()
	s.catalog = catalog
	s.mu.Unlock()
	if s.canEmitRuntimeEvents() {
		runtime.EventsEmit(s.ctx, "catalog:updated", catalog)
	}
	return catalog, nil
}

func editableCatalogPath(directory, name string) (string, error) {
	switch name {
	case "project-config.yaml", "task-config.yaml", "keyword-config.yaml":
		return filepath.Join(directory, name), nil
	default:
		return "", fmt.Errorf("不支持的配置文件: %s", name)
	}
}

func (s *Service) GetEditableConfigFile(name string) (EditableConfigFileDTO, error) {
	path, err := editableCatalogPath(s.catalogDirectory, name)
	if err != nil {
		return EditableConfigFileDTO{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return EditableConfigFileDTO{}, err
	}
	return EditableConfigFileDTO{Name: name, Content: string(data)}, nil
}

func (s *Service) SaveEditableConfigFile(name, content string) (CatalogConfig, error) {
	path, err := editableCatalogPath(s.catalogDirectory, name)
	if err != nil {
		return CatalogConfig{}, err
	}
	paths := catalogFilesForRoot(s.rootDirectory)
	projectData, err := os.ReadFile(paths.Projects)
	if err != nil {
		return CatalogConfig{}, err
	}
	taskData, err := os.ReadFile(paths.Tasks)
	if err != nil {
		return CatalogConfig{}, err
	}
	keywordData, err := os.ReadFile(paths.Keywords)
	if err != nil {
		return CatalogConfig{}, err
	}
	switch name {
	case "project-config.yaml":
		projectData = []byte(content)
	case "task-config.yaml":
		taskData = []byte(content)
	case "keyword-config.yaml":
		keywordData = []byte(content)
	}
	if _, err := parseMaintainedCatalog(projectData, taskData, keywordData); err != nil {
		return CatalogConfig{}, err
	}
	if err := atomicWriteFile(path, []byte(content), 0o644); err != nil {
		return CatalogConfig{}, err
	}
	return s.ReloadCatalogFiles()
}
