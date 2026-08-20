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

func catalogFilesForRoot(root string) runtimeCatalogFiles {
	directory := filepath.Join(root, "config")
	return runtimeCatalogFiles{
		Directory: directory,
		Projects:  filepath.Join(directory, "project-config.yaml"),
		Tasks:     filepath.Join(directory, "task-config.yaml"),
		Keywords:  filepath.Join(directory, "keyword-config.yaml"),
	}
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
		catalogFileDTO(paths.Projects, true), catalogFileDTO(paths.Tasks, true), catalogFileDTO(paths.Keywords, true),
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
		catalog = mergeCloudKeywords(catalog, cache.Items)
	}
	s.mu.Lock()
	s.catalog = catalog
	s.mu.Unlock()
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "catalog:updated", catalog)
	}
	return catalog, nil
}
