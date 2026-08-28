package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"

	"logmaster-agent/agent/internal/collector"
)

type CatalogConfig struct {
	SchemaVersion           int                                `yaml:"schema_version" json:"schemaVersion"`
	KeywordProfileTemplates map[string][]CatalogKeywordProfile `yaml:"keyword_profile_templates" json:"-"`
	TaskTemplates           map[string][]CatalogTask           `yaml:"task_templates" json:"-"`
	Projects                []CatalogProject                   `yaml:"projects" json:"projects"`
}

type CatalogProject struct {
	ID           string        `yaml:"id" json:"id"`
	Name         string        `yaml:"name" json:"name"`
	Versions     []string      `yaml:"versions" json:"versions"`
	TaskTemplate string        `yaml:"task_template" json:"-"`
	Tasks        []CatalogTask `yaml:"tasks" json:"tasks"`
}

type CatalogTask struct {
	ID                     string                  `yaml:"id" json:"id"`
	Name                   string                  `yaml:"name" json:"name"`
	Type                   string                  `yaml:"type" json:"type"`
	KeywordProfileTemplate string                  `yaml:"keyword_profile_template" json:"-"`
	KeywordProfiles        []CatalogKeywordProfile `yaml:"keyword_profiles" json:"keywordProfiles"`
	AllProjects            bool                    `yaml:"all_projects,omitempty" json:"allProjects,omitempty"`
	ApplicableProjects     []string                `yaml:"projects,omitempty" json:"applicableProjects,omitempty"`
}

type CatalogKeywordProfile struct {
	ID     string                `yaml:"id" json:"id"`
	Name   string                `yaml:"name" json:"name"`
	Rules  []CatalogKeywordRule  `yaml:"rules" json:"rules"`
	Groups []CatalogKeywordGroup `yaml:"groups" json:"groups"`
}

type CatalogKeywordGroup struct {
	ID    string               `yaml:"id" json:"id"`
	Name  string               `yaml:"name" json:"name"`
	Scope string               `yaml:"scope" json:"scope"`
	Rules []CatalogKeywordRule `yaml:"rules" json:"rules"`
}

type CatalogKeywordRule struct {
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	Match         string `yaml:"match" json:"match"`
	Text          string `yaml:"text,omitempty" json:"-"`
	Mode          string `yaml:"mode" json:"mode"`
	CaseSensitive bool   `yaml:"case_sensitive" json:"caseSensitive"`
	Level         string `yaml:"level,omitempty" json:"level,omitempty"`
	Description   string `yaml:"description,omitempty" json:"description,omitempty"`
	ReadOnly      bool   `yaml:"-" json:"readOnly,omitempty"`
}

type CatalogChangeDTO struct {
	Kind     string `json:"kind"`
	Entity   string `json:"entity"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
	Impact   string `json:"impact"`
}

type CatalogImportPreviewDTO struct {
	Token     string             `json:"token"`
	FileName  string             `json:"fileName"`
	SHA256    string             `json:"sha256"`
	Changes   []CatalogChangeDTO `json:"changes"`
	Warnings  []string           `json:"warnings"`
	Added     int                `json:"added"`
	Modified  int                `json:"modified"`
	Deleted   int                `json:"deleted"`
	Unchanged int                `json:"unchanged"`
}

type pendingCatalogImport struct {
	path    string
	hash    string
	content []byte
	catalog CatalogConfig
	created time.Time
}

func defaultCatalog() CatalogConfig {
	return CatalogConfig{SchemaVersion: 1, Projects: []CatalogProject{}}
}

func loadCatalog(path string) (CatalogConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultCatalog(), nil
	}
	if err != nil {
		return CatalogConfig{}, err
	}
	return parseCatalog(data)
}

func parseCatalog(data []byte) (CatalogConfig, error) {
	var catalog CatalogConfig
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return CatalogConfig{}, fmt.Errorf("解析项目配置: %w", err)
	}
	if catalog.SchemaVersion != 1 {
		return CatalogConfig{}, fmt.Errorf("不支持的项目配置版本 %d", catalog.SchemaVersion)
	}
	if err := expandCatalogTemplates(&catalog); err != nil {
		return CatalogConfig{}, err
	}
	if err := validateCatalog(catalog); err != nil {
		return CatalogConfig{}, err
	}
	return catalog, nil
}

func expandCatalogTemplates(catalog *CatalogConfig) error {
	for i := range catalog.TaskTemplates {
		tasks, err := expandTaskKeywordProfiles(catalog.TaskTemplates[i], catalog.KeywordProfileTemplates)
		if err != nil {
			return fmt.Errorf("任务模板 %s: %w", i, err)
		}
		catalog.TaskTemplates[i] = tasks
	}
	for i := range catalog.Projects {
		project := &catalog.Projects[i]
		if project.TaskTemplate != "" {
			if len(project.Tasks) != 0 {
				return fmt.Errorf("项目 %s 不能同时设置 task_template 和 tasks", project.ID)
			}
			tasks, ok := catalog.TaskTemplates[project.TaskTemplate]
			if !ok {
				return fmt.Errorf("项目 %s 引用了不存在的任务模板 %s", project.ID, project.TaskTemplate)
			}
			project.Tasks = cloneCatalogTasks(tasks)
			continue
		}
		tasks, err := expandTaskKeywordProfiles(project.Tasks, catalog.KeywordProfileTemplates)
		if err != nil {
			return fmt.Errorf("项目 %s: %w", project.ID, err)
		}
		project.Tasks = tasks
	}
	return nil
}

func expandTaskKeywordProfiles(tasks []CatalogTask, templates map[string][]CatalogKeywordProfile) ([]CatalogTask, error) {
	result := cloneCatalogTasks(tasks)
	for i := range result {
		task := &result[i]
		if task.KeywordProfileTemplate == "" {
			continue
		}
		if len(task.KeywordProfiles) != 0 {
			return nil, fmt.Errorf("任务 %s 不能同时设置 keyword_profile_template 和 keyword_profiles", task.ID)
		}
		profiles, ok := templates[task.KeywordProfileTemplate]
		if !ok {
			return nil, fmt.Errorf("任务 %s 引用了不存在的关键字模板 %s", task.ID, task.KeywordProfileTemplate)
		}
		task.KeywordProfiles = cloneCatalogProfiles(profiles)
	}
	return result, nil
}

func cloneCatalogTasks(tasks []CatalogTask) []CatalogTask {
	result := append([]CatalogTask(nil), tasks...)
	for i := range result {
		result[i].ApplicableProjects = append([]string(nil), result[i].ApplicableProjects...)
		result[i].KeywordProfiles = cloneCatalogProfiles(result[i].KeywordProfiles)
	}
	return result
}

func cloneCatalogProfiles(profiles []CatalogKeywordProfile) []CatalogKeywordProfile {
	result := append([]CatalogKeywordProfile(nil), profiles...)
	for i := range result {
		result[i].Rules = append([]CatalogKeywordRule(nil), result[i].Rules...)
		result[i].Groups = append([]CatalogKeywordGroup(nil), result[i].Groups...)
		for groupIndex := range result[i].Groups {
			result[i].Groups[groupIndex].Rules = append([]CatalogKeywordRule(nil), result[i].Groups[groupIndex].Rules...)
		}
	}
	return result
}

func catalogKeywordRules(profile CatalogKeywordProfile) []CatalogKeywordRule {
	rules := append([]CatalogKeywordRule(nil), profile.Rules...)
	for _, group := range profile.Groups {
		rules = append(rules, group.Rules...)
	}
	return rules
}

func validateCatalog(catalog CatalogConfig) error {
	seen := map[string]string{}
	add := func(kind, id, name string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("%s 缺少稳定 ID", kind)
		}
		key := kind + ":" + id
		if prior := seen[key]; prior != "" {
			return fmt.Errorf("%s ID %s 重复：%s / %s", kind, id, prior, name)
		}
		seen[key] = name
		return nil
	}
	for _, project := range catalog.Projects {
		if err := add("项目", project.ID, project.Name); err != nil {
			return err
		}
		if strings.TrimSpace(project.Name) == "" {
			return fmt.Errorf("项目 %s 缺少名称", project.ID)
		}
		for _, task := range project.Tasks {
			if err := add("任务", project.ID+"/"+task.ID, task.Name); err != nil {
				return err
			}
			for _, profile := range task.KeywordProfiles {
				if err := add("关键字方案", project.ID+"/"+task.ID+"/"+profile.ID, profile.Name); err != nil {
					return err
				}
				for _, group := range profile.Groups {
					if err := add("关键字类别", project.ID+"/"+task.ID+"/"+profile.ID+"/"+group.ID, group.Name); err != nil {
						return err
					}
				}
				for _, rule := range catalogKeywordRules(profile) {
					if err := add("关键字", project.ID+"/"+task.ID+"/"+profile.ID+"/"+rule.ID, rule.Name); err != nil {
						return err
					}
					if rule.Mode != "contains" && rule.Mode != "regex" {
						return fmt.Errorf("关键字 %s 的 mode 必须是 contains 或 regex", rule.ID)
					}
					if strings.TrimSpace(rule.Match) == "" {
						return fmt.Errorf("关键字 %s 缺少匹配内容", rule.ID)
					}
				}
			}
		}
	}
	return nil
}

func (s *Service) GetCatalog() CatalogConfig { s.mu.RLock(); defer s.mu.RUnlock(); return s.catalog }

// loadDesktopCatalog loads the built-in project/keyword catalog and merges any
// locally imported user catalog (project-catalog.yaml) on top of it. This lets
// testers add or override keyword plans without a new release and without a
// cloud sync endpoint.
func loadDesktopCatalog(userPath string) (CatalogConfig, error) {
	catalog, err := loadEmbeddedCatalog()
	if err != nil {
		return CatalogConfig{}, err
	}
	data, err := os.ReadFile(userPath)
	if errors.Is(err, os.ErrNotExist) {
		return catalog, nil
	}
	if err != nil {
		return CatalogConfig{}, err
	}
	user, err := parseCatalog(data)
	if err != nil {
		return CatalogConfig{}, fmt.Errorf("解析本地项目配置: %w", err)
	}
	return mergeCatalog(catalog, user), nil
}

func mergeCatalog(base, overlay CatalogConfig) CatalogConfig {
	result := base
	index := make(map[string]int, len(result.Projects))
	for i, project := range result.Projects {
		index[project.ID] = i
	}
	for _, project := range overlay.Projects {
		if i, ok := index[project.ID]; ok {
			result.Projects[i] = project
		} else {
			result.Projects = append(result.Projects, project)
		}
	}
	return result
}

func (s *Service) SelectCatalogFile() (CatalogImportPreviewDTO, error) {
	path, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{Title: "选择项目配置文件", Filters: []runtime.FileFilter{{DisplayName: "YAML 配置", Pattern: "*.yaml;*.yml"}}})
	if err != nil {
		return CatalogImportPreviewDTO{}, err
	}
	if path == "" {
		return CatalogImportPreviewDTO{}, errors.New("未选择配置文件")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CatalogImportPreviewDTO{}, err
	}
	candidate, err := parseCatalog(data)
	if err != nil {
		return CatalogImportPreviewDTO{}, err
	}
	s.mu.RLock()
	current := s.catalog
	configs := make(map[string]DeviceConfigDTO, len(s.configs))
	for id, cfg := range s.configs {
		configs[id] = cfg
	}
	s.mu.RUnlock()
	merged := mergeCatalog(current, candidate)
	preview := diffCatalog(current, merged, configs)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	token, err := catalogToken()
	if err != nil {
		return CatalogImportPreviewDTO{}, err
	}
	encoded, err := yaml.Marshal(merged)
	if err != nil {
		return CatalogImportPreviewDTO{}, err
	}
	preview.Token = token
	preview.FileName = filepath.Base(path)
	preview.SHA256 = hash
	s.mu.Lock()
	s.pendingImports[token] = pendingCatalogImport{path: path, hash: hash, content: encoded, catalog: merged, created: time.Now()}
	s.mu.Unlock()
	return preview, nil
}

func (s *Service) ApplyCatalogImport(token string) error {
	s.mu.Lock()
	pending, ok := s.pendingImports[token]
	if ok {
		delete(s.pendingImports, token)
	}
	s.mu.Unlock()
	if !ok {
		return errors.New("配置预览已失效，请重新选择文件")
	}
	data, err := os.ReadFile(pending.path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != pending.hash {
		return errors.New("配置文件在预览后发生变化，请重新预览")
	}
	if err := atomicWriteFile(s.catalogPath, pending.content, 0o644); err != nil {
		return err
	}
	if cache, cacheErr := readCloudKeywordCache(s.cloudKeywordPath); cacheErr == nil {
		if !hasCloudKeywordProfile(pending.catalog) {
			pending.catalog = mergeCloudKeywords(pending.catalog, cache.Items)
		}
	}
	s.mu.Lock()
	s.catalog = pending.catalog
	s.mu.Unlock()
	runtime.EventsEmit(s.ctx, "catalog:updated", pending.catalog)
	return nil
}

func diffCatalog(oldCatalog, newCatalog CatalogConfig, configs map[string]DeviceConfigDTO) CatalogImportPreviewDTO {
	oldItems := flattenCatalog(oldCatalog)
	newItems := flattenCatalog(newCatalog)
	keys := map[string]struct{}{}
	for key := range oldItems {
		keys[key] = struct{}{}
	}
	for key := range newItems {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	result := CatalogImportPreviewDTO{}
	for _, key := range ordered {
		oldItem, oldOK := oldItems[key]
		newItem, newOK := newItems[key]
		change := CatalogChangeDTO{Entity: strings.SplitN(key, ":", 2)[0], ID: strings.SplitN(key, ":", 2)[1]}
		switch {
		case !oldOK && newOK:
			change.Kind = "added"
			change.Name = newItem.name
			change.NewValue = newItem.value
			result.Added++
		case oldOK && !newOK:
			change.Kind = "deleted"
			change.Name = oldItem.name
			change.OldValue = oldItem.value
			change.Impact = catalogImpact(change.Entity, change.ID, configs)
			result.Deleted++
		case oldItem.value != newItem.value:
			change.Kind = "modified"
			change.Name = newItem.name
			change.OldValue = oldItem.value
			change.NewValue = newItem.value
			result.Modified++
		default:
			result.Unchanged++
			continue
		}
		result.Changes = append(result.Changes, change)
	}
	return result
}

type catalogFlatItem struct{ name, value string }

func flattenCatalog(c CatalogConfig) map[string]catalogFlatItem {
	result := map[string]catalogFlatItem{}
	put := func(kind, id, name string, value any) {
		data, _ := json.Marshal(value)
		result[kind+":"+id] = catalogFlatItem{name: name, value: string(data)}
	}
	for _, p := range c.Projects {
		put("project", p.ID, p.Name, p)
		for _, t := range p.Tasks {
			tid := p.ID + "/" + t.ID
			put("task", tid, t.Name, t)
			for _, profile := range t.KeywordProfiles {
				pid := tid + "/" + profile.ID
				put("profile", pid, profile.Name, profile)
				for _, group := range profile.Groups {
					put("group", pid+"/"+group.ID, group.Name, group)
				}
				for _, rule := range catalogKeywordRules(profile) {
					put("rule", pid+"/"+rule.ID, rule.Name, rule)
				}
			}
		}
	}
	return result
}

func catalogImpact(kind, id string, configs map[string]DeviceConfigDTO) string {
	var ports []string
	for _, cfg := range configs {
		matched := kind == "project" && cfg.ProjectID == id || kind == "task" && cfg.ProjectID+"/"+cfg.TestTaskID == id || kind == "profile" && cfg.ProjectID+"/"+cfg.TestTaskID+"/"+cfg.KeywordProfileID == id
		if matched {
			ports = append(ports, cfg.PortName)
		}
	}
	if len(ports) == 0 {
		return ""
	}
	sort.Strings(ports)
	return "正在被通道 " + strings.Join(ports, "、") + " 使用"
}

func catalogToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Service) catalogRulesFor(dto DeviceConfigDTO) []collector.Rule {
	if !dto.KeywordMatchingEnabled {
		return nil
	}
	s.mu.RLock()
	catalog := s.catalog
	s.mu.RUnlock()
	selected := map[string]struct{}{}
	for _, id := range dto.KeywordRuleIDs {
		selected[id] = struct{}{}
	}
	if len(selected) == 0 {
		return nil
	}
	for _, p := range catalog.Projects {
		for _, t := range p.Tasks {
			for _, profile := range t.KeywordProfiles {
				if profile.ID != dto.KeywordProfileID {
					continue
				}
				var rules []collector.Rule
				for _, rule := range catalogKeywordRules(profile) {
					if _, ok := selected[rule.ID]; !ok {
						continue
					}
					item := collector.Rule{ID: rule.ID, Name: rule.Name, Severity: "MATCH", Module: profile.Name}
					if rule.Mode == "regex" {
						item.Pattern = rule.Match
					} else if rule.CaseSensitive {
						item.Pattern = ".*" + regexpQuote(rule.Match) + ".*"
					} else {
						item.Keywords = []string{rule.Match}
					}
					rules = append(rules, item)
				}
				return rules
			}
		}
	}
	return nil
}

func regexpQuote(value string) string {
	return regexp.QuoteMeta(value)
}

func (s *Service) catalogSelectionValid(dto DeviceConfigDTO) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return catalogSelectionValidIn(s.catalog, dto)
}

func catalogSelectionValidIn(catalog CatalogConfig, dto DeviceConfigDTO) bool {
	projectFound := dto.ProjectID == ""
	taskFound := dto.TestTaskID == ""
	profileFound := dto.KeywordProfileID == ""
	allowedRules := map[string]struct{}{}
	for _, p := range catalog.Projects {
		if p.ID == dto.ProjectID {
			projectFound = true
		}
		for _, t := range p.Tasks {
			if t.ID == dto.TestTaskID {
				taskFound = true
			}
			for _, profile := range t.KeywordProfiles {
				if profile.ID == dto.KeywordProfileID {
					profileFound = true
					for _, rule := range catalogKeywordRules(profile) {
						allowedRules[rule.ID] = struct{}{}
					}
				}
			}
		}
	}
	if !projectFound || !taskFound || !profileFound {
		return false
	}
	if dto.KeywordProfileID == "" {
		return len(dto.KeywordRuleIDs) == 0
	}
	for _, ruleID := range dto.KeywordRuleIDs {
		if _, ok := allowedRules[ruleID]; !ok {
			return false
		}
	}
	return true
}
