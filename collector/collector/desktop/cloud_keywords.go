package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
	"logmaster-agent/agent/internal/backend"
	"logmaster-agent/agent/internal/spool"
)

const cloudKeywordProfileID = "cloud-standard-keywords"

type cloudKeywordCache struct {
	SyncedAt time.Time                 `json:"syncedAt"`
	Items    []backend.StandardKeyword `json:"items"`
}

type CloudKeywordSyncResult struct {
	Count    int       `json:"count"`
	SyncedAt time.Time `json:"syncedAt"`
	Message  string    `json:"message"`
}

// SyncCloudConfig replaces all three editable catalog files with one coherent
// server snapshot, then reloads the in-memory catalog.
func (s *Service) SyncCloudConfig() (CloudKeywordSyncResult, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
	defer cancel()
	snapshot, err := s.backendClient.SyncCollectorConfig(ctx)
	if err != nil {
		// Older deployments may not have the aggregate endpoint. Keep project and
		// task files intact, but still refresh keywords through the stable endpoint.
		items, keywordErr := s.backendClient.SyncKeywords(ctx)
		if keywordErr != nil {
			return CloudKeywordSyncResult{}, fmt.Errorf("项目、任务配置已保留；关键字同步失败：%w", keywordErr)
		}
		return s.syncCloudKeywordsOnly(items, err.Error())
	}
	paths := catalogFilesForRoot(s.rootDirectory)
	projects := maintainedProjectsFile{SchemaVersion: 1}
	for _, project := range snapshot.Projects {
		id, name := strings.TrimSpace(project.ID), strings.TrimSpace(project.Name)
		if id == "" || name == "" {
			return CloudKeywordSyncResult{}, errors.New("云端项目存在空 ID 或名称")
		}
		projects.Projects = append(projects.Projects, CatalogProject{ID: id, Name: name})
	}
	projectData, err := yaml.Marshal(projects)
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	tasks := maintainedTasksFile{SchemaVersion: 1}
	for _, scenario := range snapshot.Scenarios {
		id, name := strings.TrimSpace(scenario.ID), strings.TrimSpace(scenario.Name)
		if !scenario.Enabled {
			continue
		}
		if id == "" || name == "" {
			return CloudKeywordSyncResult{}, errors.New("云端任务存在空 ID 或名称")
		}
		tasks.Tasks = append(tasks.Tasks, CatalogTask{ID: id, Name: name, Type: "special", AllProjects: scenario.AllProjects, ApplicableProjects: append([]string(nil), scenario.Projects...)})
	}
	taskData, err := yaml.Marshal(tasks)
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	keywordFile := maintainedKeywordsFile{SchemaVersion: 1}
	profile := CatalogKeywordProfile{ID: cloudKeywordProfileID, Name: "云端标准关键字"}
	for _, item := range snapshot.Keywords {
		category, scope := strings.TrimSpace(item.Category), strings.TrimSpace(item.Scope)
		if category == "" {
			category = "未分类"
		}
		if scope == "" {
			scope = "全局"
		}
		groupID := "cloud-group-" + safeCloudID(category) + "-" + safeCloudID(scope)
		var group *CatalogKeywordGroup
		for i := range profile.Groups {
			if profile.Groups[i].ID == groupID {
				group = &profile.Groups[i]
				break
			}
		}
		if group == nil {
			profile.Groups = append(profile.Groups, CatalogKeywordGroup{ID: groupID, Name: category, Scope: scope})
			group = &profile.Groups[len(profile.Groups)-1]
		}
		group.Rules = append(group.Rules, CatalogKeywordRule{ID: fmt.Sprintf("cloud-rule-%d", item.ID), Name: item.Name, Match: item.Keyword, Mode: "contains", Level: item.Level, Description: item.Description, ReadOnly: true})
	}
	keywordFile.Profiles = []CatalogKeywordProfile{profile}
	keywordData, err := compactMaintainedKeywordsFile(keywordFile)
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if _, err := parseMaintainedCatalog(projectData, taskData, keywordData); err != nil {
		return CloudKeywordSyncResult{}, fmt.Errorf("云端配置校验失败: %w", err)
	}
	if err := atomicWriteFile(paths.Projects, projectData, 0o644); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if err := atomicWriteFile(paths.Tasks, taskData, 0o644); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if err := atomicWriteFile(paths.Keywords, keywordData, 0o644); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	cache := cloudKeywordCache{SyncedAt: snapshot.SyncedAt, Items: snapshot.Keywords}
	cacheData, err := json.MarshalIndent(cache, "", " ")
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if err := atomicWriteFile(s.cloudKeywordPath, append(cacheData, '\n'), 0o600); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if _, err := s.ReloadCatalogFiles(); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	return CloudKeywordSyncResult{Count: len(snapshot.Keywords), SyncedAt: snapshot.SyncedAt, Message: fmt.Sprintf("已同步：项目 %d 个，任务 %d 个，关键字 %d 条", len(projects.Projects), len(tasks.Tasks), len(snapshot.Keywords))}, nil
}

func (s *Service) syncCloudKeywordsOnly(items []backend.StandardKeyword, reason string) (CloudKeywordSyncResult, error) {
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	profile := CatalogKeywordProfile{ID: cloudKeywordProfileID, Name: "云端标准关键字"}
	for _, item := range items {
		category, scope := strings.TrimSpace(item.Category), strings.TrimSpace(item.Scope)
		if category == "" {
			category = "未分类"
		}
		if scope == "" {
			scope = "全局"
		}
		groupID := "cloud-group-" + safeCloudID(category) + "-" + safeCloudID(scope)
		groupIndex := -1
		for i := range profile.Groups {
			if profile.Groups[i].ID == groupID {
				groupIndex = i
				break
			}
		}
		if groupIndex < 0 {
			profile.Groups = append(profile.Groups, CatalogKeywordGroup{ID: groupID, Name: category, Scope: scope})
			groupIndex = len(profile.Groups) - 1
		}
		profile.Groups[groupIndex].Rules = append(profile.Groups[groupIndex].Rules, CatalogKeywordRule{ID: fmt.Sprintf("cloud-rule-%d", item.ID), Name: item.Name, Match: item.Keyword, Mode: "contains", Level: item.Level, Description: item.Description, ReadOnly: true})
	}
	updated := false
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == cloudKeywordProfileID {
			keywords.Profiles[i] = profile
			updated = true
			break
		}
	}
	if !updated {
		keywords.Profiles = append(keywords.Profiles, profile)
	}
	if _, err := s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword"); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	cacheData, err := json.MarshalIndent(cloudKeywordCache{SyncedAt: time.Now(), Items: items}, "", " ")
	if err == nil {
		_ = atomicWriteFile(s.cloudKeywordPath, append(cacheData, '\n'), 0o600)
	}
	return CloudKeywordSyncResult{Count: len(items), SyncedAt: time.Now(), Message: fmt.Sprintf("项目、任务配置已保留；已同步关键字 %d 条（%s）", len(items), reason)}, nil
}

func compactMaintainedKeywordsFile(file maintainedKeywordsFile) ([]byte, error) {
	data, err := yaml.Marshal(file)
	if err != nil {
		return nil, err
	}
	return append([]byte("# 云端同步关键字配置，云端规则只读。\n"), data...), nil
}

type ServerFailureNotice struct {
	UploadID     string `json:"uploadId"`
	QueryCode    string `json:"queryCode"`
	FileName     string `json:"fileName"`
	ErrorType    string `json:"errorType"`
	ErrorMessage string `json:"errorMessage"`
}

func (s *Service) loadCloudKeywordCache() {
	cache, err := readCloudKeywordCache(s.cloudKeywordPath)
	if err != nil {
		return
	}
	s.mu.Lock()
	if !hasCloudKeywordProfile(s.catalog) {
		s.catalog = mergeCloudKeywords(s.catalog, cache.Items)
	}
	s.mu.Unlock()
}

func hasCloudKeywordProfile(catalog CatalogConfig) bool {
	for _, project := range catalog.Projects {
		for _, task := range project.Tasks {
			for _, profile := range task.KeywordProfiles {
				if profile.ID == cloudKeywordProfileID {
					return true
				}
			}
		}
	}
	return false
}

func readCloudKeywordCache(path string) (cloudKeywordCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cloudKeywordCache{}, err
	}
	var cache cloudKeywordCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return cloudKeywordCache{}, err
	}
	return cache, nil
}

func (s *Service) SyncCloudKeywords() (CloudKeywordSyncResult, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
	defer cancel()
	items, err := s.backendClient.SyncKeywords(ctx)
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	cache := cloudKeywordCache{SyncedAt: time.Now().UTC(), Items: items}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return CloudKeywordSyncResult{}, err
	}
	if err := atomicWriteFile(s.cloudKeywordPath, append(data, '\n'), 0o600); err != nil {
		return CloudKeywordSyncResult{}, err
	}
	s.mu.Lock()
	s.catalog = mergeCloudKeywords(s.catalog, items)
	value := s.catalog
	s.mu.Unlock()
	if s.canEmitRuntimeEvents() {
		runtime.EventsEmit(s.ctx, "catalog:updated", value)
	}
	if s.logger != nil {
		s.logger.Info("cloud keywords synchronized", "component", "desktop.keywords", "count", len(items))
	}
	return CloudKeywordSyncResult{Count: len(items), SyncedAt: cache.SyncedAt, Message: fmt.Sprintf("已同步 %d 条云端标准关键字", len(items))}, nil
}

func mergeCloudKeywords(catalog CatalogConfig, items []backend.StandardKeyword) CatalogConfig {
	groups := map[string]*CatalogKeywordGroup{}
	keys := make([]string, 0)
	for _, item := range items {
		category, scope := strings.TrimSpace(item.Category), strings.TrimSpace(item.Scope)
		if category == "" {
			category = "未分类"
		}
		if scope == "" {
			scope = "全局"
		}
		key := category + "|" + scope
		group := groups[key]
		if group == nil {
			group = &CatalogKeywordGroup{ID: "cloud-group-" + safeCloudID(category) + "-" + safeCloudID(scope), Name: category, Scope: scope}
			groups[key] = group
			keys = append(keys, key)
		}
		group.Rules = append(group.Rules, CatalogKeywordRule{ID: fmt.Sprintf("cloud-rule-%d", item.ID), Name: item.Name, Match: item.Keyword, Mode: "contains", Level: item.Level, Description: item.Description, ReadOnly: true})
	}
	sort.Strings(keys)
	profile := CatalogKeywordProfile{ID: cloudKeywordProfileID, Name: "云端标准关键字"}
	for _, key := range keys {
		profile.Groups = append(profile.Groups, *groups[key])
	}
	for projectIndex := range catalog.Projects {
		for taskIndex := range catalog.Projects[projectIndex].Tasks {
			profiles := catalog.Projects[projectIndex].Tasks[taskIndex].KeywordProfiles
			filtered := profiles[:0]
			for _, existing := range profiles {
				if existing.ID != cloudKeywordProfileID {
					filtered = append(filtered, existing)
				}
			}
			if len(items) > 0 {
				filtered = append(filtered, profile)
			}
			catalog.Projects[projectIndex].Tasks[taskIndex].KeywordProfiles = filtered
		}
	}
	return catalog
}

func safeCloudID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(value)))[:8]
	if slug == "" {
		return digest
	}
	return slug + "-" + digest
}

func (s *Service) monitorUploadSessionFailures() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	check := func() {
		s.mu.RLock()
		codes := map[string]struct{}{}
		for _, cfg := range s.configs {
			if cfg.UploadEnabled && cfg.QueryCode != "" {
				codes[cfg.QueryCode] = struct{}{}
			}
		}
		s.mu.RUnlock()
		if batches, _, err := s.store.ListBatches(s.ctx, spool.BatchFilter{IncludeUploaded: true, Limit: 200}); err == nil {
			for _, batch := range batches {
				if batch.QueryCode != "" {
					codes[batch.QueryCode] = struct{}{}
				}
			}
		}
		for code := range codes {
			ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			status, err := s.backendClient.QueryUploadSession(ctx, code)
			cancel()
			if err != nil {
				continue
			}
			for _, batch := range status.Batches {
				if batch.Status != "failed" {
					continue
				}
				errorType := batch.ErrorType
				if errorType == "" {
					errorType = "unknown_failed"
				}
				key := batch.UploadID + "|" + errorType + "|" + batch.ErrorMessage
				s.mu.Lock()
				_, seen := s.reportedServerFailures[key]
				if !seen {
					s.reportedServerFailures[key] = struct{}{}
				}
				s.mu.Unlock()
				if seen {
					continue
				}
				notice := ServerFailureNotice{UploadID: batch.UploadID, QueryCode: status.QueryCode, FileName: batch.OriginalName, ErrorType: errorType, ErrorMessage: batch.ErrorMessage}
				if s.logger != nil {
					s.logger.Error("server failed to process uploaded log", "component", "desktop.upload-status", "upload_id", batch.UploadID, "query_code", status.QueryCode, "error_type", errorType, "error", batch.ErrorMessage)
				}
				if s.canEmitRuntimeEvents() {
					runtime.EventsEmit(s.ctx, "upload:server-failure", notice)
				}
			}
		}
	}
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
