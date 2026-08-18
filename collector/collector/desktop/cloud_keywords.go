package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	s.catalog = mergeCloudKeywords(s.catalog, cache.Items)
	s.mu.Unlock()
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
	if s.ctx != nil {
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
				if s.ctx != nil {
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
