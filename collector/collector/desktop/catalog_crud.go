package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type CatalogProjectInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

type CatalogTaskInput struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Type               string   `json:"type"`
	AllProjects        bool     `json:"allProjects"`
	ApplicableProjects []string `json:"applicableProjects"`
}

type CatalogKeywordProfileInput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CatalogKeywordGroupInput struct {
	ProfileID string `json:"profileId"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope"`
}

type CatalogKeywordRuleInput struct {
	ProfileID     string `json:"profileId"`
	GroupID       string `json:"groupId"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Match         string `json:"match"`
	Mode          string `json:"mode"`
	CaseSensitive bool   `json:"caseSensitive"`
}

func normalizeCatalogID(value, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s ID 不能为空", label)
	}
	return value, nil
}

func (s *Service) maintainedFiles() (maintainedProjectsFile, maintainedTasksFile, maintainedKeywordsFile, runtimeCatalogFiles, error) {
	paths := catalogFilesForRoot(s.rootDirectory)
	projectData, err := readFile(paths.Projects)
	if err != nil {
		return maintainedProjectsFile{}, maintainedTasksFile{}, maintainedKeywordsFile{}, paths, err
	}
	taskData, err := readFile(paths.Tasks)
	if err != nil {
		return maintainedProjectsFile{}, maintainedTasksFile{}, maintainedKeywordsFile{}, paths, err
	}
	keywordData, err := readFile(paths.Keywords)
	if err != nil {
		return maintainedProjectsFile{}, maintainedTasksFile{}, maintainedKeywordsFile{}, paths, err
	}
	var projects maintainedProjectsFile
	var tasks maintainedTasksFile
	var keywords maintainedKeywordsFile
	if err := decodeMaintainedYAML("项目配置", projectData, &projects); err != nil {
		return projects, tasks, keywords, paths, err
	}
	if err := decodeMaintainedYAML("任务配置", taskData, &tasks); err != nil {
		return projects, tasks, keywords, paths, err
	}
	if err := decodeMaintainedYAML("关键字配置", keywordData, &keywords); err != nil {
		return projects, tasks, keywords, paths, err
	}
	return projects, tasks, keywords, paths, nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) saveMaintainedFiles(projects maintainedProjectsFile, tasks maintainedTasksFile, keywords maintainedKeywordsFile, paths runtimeCatalogFiles, changed string) (CatalogConfig, error) {
	projectData, err := yaml.Marshal(projects)
	if err != nil {
		return CatalogConfig{}, err
	}
	taskData, err := yaml.Marshal(tasks)
	if err != nil {
		return CatalogConfig{}, err
	}
	keywordData, err := compactMaintainedKeywordsFile(keywords)
	if err != nil {
		return CatalogConfig{}, err
	}
	if err := validateMaintainedReferences(projects, tasks); err != nil {
		return CatalogConfig{}, err
	}
	if _, err := parseMaintainedCatalog(projectData, taskData, keywordData); err != nil {
		return CatalogConfig{}, err
	}
	var path string
	var data []byte
	switch changed {
	case "project":
		path, data = paths.Projects, projectData
	case "task":
		path, data = paths.Tasks, taskData
	case "keyword":
		path, data = paths.Keywords, keywordData
	default:
		return CatalogConfig{}, errors.New("未知配置类型")
	}
	if err := atomicWriteFile(path, data, 0o644); err != nil {
		return CatalogConfig{}, err
	}
	return s.ReloadCatalogFiles()
}

func validateMaintainedReferences(projects maintainedProjectsFile, tasks maintainedTasksFile) error {
	known := make(map[string]struct{}, len(projects.Projects))
	for _, project := range projects.Projects {
		known[strings.TrimSpace(project.ID)] = struct{}{}
	}
	for _, task := range tasks.Tasks {
		if task.AllProjects {
			continue
		}
		for _, id := range task.ApplicableProjects {
			if _, ok := known[strings.TrimSpace(id)]; !ok {
				return fmt.Errorf("任务 %s 引用了不存在的项目 %s", task.Name, id)
			}
		}
	}
	return nil
}

func (s *Service) SaveCatalogProject(input CatalogProjectInput) (CatalogConfig, error) {
	id, err := normalizeCatalogID(input.ID, "项目")
	if err != nil {
		return CatalogConfig{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CatalogConfig{}, errors.New("项目名称不能为空")
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	value := CatalogProject{ID: id, Name: name, Versions: trimStrings(input.Versions)}
	found := false
	for i := range projects.Projects {
		if projects.Projects[i].ID == id {
			projects.Projects[i] = value
			found = true
			break
		}
	}
	if !found {
		projects.Projects = append(projects.Projects, value)
	}
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "project")
}

func (s *Service) DeleteCatalogProject(id string) (CatalogConfig, error) {
	id, err := normalizeCatalogID(id, "项目")
	if err != nil {
		return CatalogConfig{}, err
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	for _, task := range tasks.Tasks {
		for _, projectID := range task.ApplicableProjects {
			if projectID == id {
				return CatalogConfig{}, fmt.Errorf("项目正在被任务 %s 使用，请先修改任务适用范围", task.Name)
			}
		}
	}
	filtered := projects.Projects[:0]
	for _, project := range projects.Projects {
		if project.ID != id {
			filtered = append(filtered, project)
		}
	}
	if len(filtered) == len(projects.Projects) {
		return CatalogConfig{}, errors.New("项目不存在")
	}
	projects.Projects = filtered
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "project")
}

func (s *Service) SaveCatalogTask(input CatalogTaskInput) (CatalogConfig, error) {
	id, err := normalizeCatalogID(input.ID, "任务")
	if err != nil {
		return CatalogConfig{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CatalogConfig{}, errors.New("任务名称不能为空")
	}
	typeName := strings.TrimSpace(input.Type)
	if typeName == "" {
		typeName = "special"
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	value := CatalogTask{ID: id, Name: name, Type: typeName, AllProjects: input.AllProjects, ApplicableProjects: trimStrings(input.ApplicableProjects)}
	found := false
	for i := range tasks.Tasks {
		if tasks.Tasks[i].ID == id {
			tasks.Tasks[i] = value
			found = true
			break
		}
	}
	if !found {
		tasks.Tasks = append(tasks.Tasks, value)
	}
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "task")
}

func (s *Service) DeleteCatalogTask(id string) (CatalogConfig, error) {
	id, err := normalizeCatalogID(id, "任务")
	if err != nil {
		return CatalogConfig{}, err
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	filtered := tasks.Tasks[:0]
	for _, task := range tasks.Tasks {
		if task.ID != id {
			filtered = append(filtered, task)
		}
	}
	if len(filtered) == len(tasks.Tasks) {
		return CatalogConfig{}, errors.New("任务不存在")
	}
	tasks.Tasks = filtered
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "task")
}

func (s *Service) SaveCatalogKeywordProfile(input CatalogKeywordProfileInput) (CatalogConfig, error) {
	id, err := normalizeCatalogID(input.ID, "关键字方案")
	if err != nil {
		return CatalogConfig{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CatalogConfig{}, errors.New("方案名称不能为空")
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	found := false
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == id {
			keywords.Profiles[i].Name = name
			found = true
			break
		}
	}
	if !found {
		keywords.Profiles = append(keywords.Profiles, CatalogKeywordProfile{ID: id, Name: name})
	}
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
}

func (s *Service) DeleteCatalogKeywordProfile(id string) (CatalogConfig, error) {
	id, err := normalizeCatalogID(id, "关键字方案")
	if err != nil {
		return CatalogConfig{}, err
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	filtered := keywords.Profiles[:0]
	for _, profile := range keywords.Profiles {
		if profile.ID != id {
			filtered = append(filtered, profile)
		}
	}
	if len(filtered) == len(keywords.Profiles) {
		return CatalogConfig{}, errors.New("关键字方案不存在")
	}
	keywords.Profiles = filtered
	return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
}

func (s *Service) SaveCatalogKeywordGroup(input CatalogKeywordGroupInput) (CatalogConfig, error) {
	profileID, err := normalizeCatalogID(input.ProfileID, "关键字方案")
	if err != nil {
		return CatalogConfig{}, err
	}
	id, err := normalizeCatalogID(input.ID, "关键字分类")
	if err != nil {
		return CatalogConfig{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CatalogConfig{}, errors.New("分类名称不能为空")
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == profileID {
			value := CatalogKeywordGroup{ID: id, Name: name, Scope: strings.TrimSpace(input.Scope)}
			for j := range keywords.Profiles[i].Groups {
				if keywords.Profiles[i].Groups[j].ID == id {
					value.Rules = keywords.Profiles[i].Groups[j].Rules
					keywords.Profiles[i].Groups[j] = value
					return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
				}
			}
			keywords.Profiles[i].Groups = append(keywords.Profiles[i].Groups, value)
			return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
		}
	}
	return CatalogConfig{}, errors.New("关键字方案不存在")
}

func (s *Service) DeleteCatalogKeywordGroup(profileID, id string) (CatalogConfig, error) {
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == strings.TrimSpace(profileID) {
			filtered := keywords.Profiles[i].Groups[:0]
			for _, group := range keywords.Profiles[i].Groups {
				if group.ID != strings.TrimSpace(id) {
					filtered = append(filtered, group)
				}
			}
			if len(filtered) == len(keywords.Profiles[i].Groups) {
				return CatalogConfig{}, errors.New("关键字分类不存在")
			}
			keywords.Profiles[i].Groups = filtered
			return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
		}
	}
	return CatalogConfig{}, errors.New("关键字方案不存在")
}

func (s *Service) SaveCatalogKeywordRule(input CatalogKeywordRuleInput) (CatalogConfig, error) {
	profileID, err := normalizeCatalogID(input.ProfileID, "关键字方案")
	if err != nil {
		return CatalogConfig{}, err
	}
	id, err := normalizeCatalogID(input.ID, "关键字")
	if err != nil {
		return CatalogConfig{}, err
	}
	match := strings.TrimSpace(input.Match)
	if match == "" {
		return CatalogConfig{}, errors.New("匹配内容不能为空")
	}
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "contains"
	}
	if mode != "contains" && mode != "regex" {
		return CatalogConfig{}, errors.New("匹配方式只能是普通包含或正则")
	}
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	value := CatalogKeywordRule{ID: id, Name: strings.TrimSpace(input.Name), Match: match, Mode: mode, CaseSensitive: input.CaseSensitive}
	if value.Name == "" {
		value.Name = match
	}
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == profileID {
			var target *[]CatalogKeywordRule = &keywords.Profiles[i].Rules
			if groupID := strings.TrimSpace(input.GroupID); groupID != "" {
				for j := range keywords.Profiles[i].Groups {
					if keywords.Profiles[i].Groups[j].ID == groupID {
						target = &keywords.Profiles[i].Groups[j].Rules
						break
					}
				}
				if target == &keywords.Profiles[i].Rules {
					return CatalogConfig{}, errors.New("关键字分类不存在")
				}
			}
			for j := range *target {
				if (*target)[j].ID == id {
					(*target)[j] = value
					return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
				}
			}
			*target = append(*target, value)
			return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
		}
	}
	return CatalogConfig{}, errors.New("关键字方案不存在")
}

func (s *Service) DeleteCatalogKeywordRule(profileID, groupID, id string) (CatalogConfig, error) {
	projects, tasks, keywords, paths, err := s.maintainedFiles()
	if err != nil {
		return CatalogConfig{}, err
	}
	for i := range keywords.Profiles {
		if keywords.Profiles[i].ID == strings.TrimSpace(profileID) {
			var target *[]CatalogKeywordRule = &keywords.Profiles[i].Rules
			if groupID = strings.TrimSpace(groupID); groupID != "" {
				for j := range keywords.Profiles[i].Groups {
					if keywords.Profiles[i].Groups[j].ID == groupID {
						target = &keywords.Profiles[i].Groups[j].Rules
						break
					}
				}
				if target == &keywords.Profiles[i].Rules {
					return CatalogConfig{}, errors.New("关键字分类不存在")
				}
			}
			filtered := (*target)[:0]
			for _, rule := range *target {
				if rule.ID != strings.TrimSpace(id) {
					filtered = append(filtered, rule)
				}
			}
			if len(filtered) == len(*target) {
				return CatalogConfig{}, errors.New("关键字不存在")
			}
			*target = filtered
			return s.saveMaintainedFiles(projects, tasks, keywords, paths, "keyword")
		}
	}
	return CatalogConfig{}, errors.New("关键字方案不存在")
}

func trimStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
