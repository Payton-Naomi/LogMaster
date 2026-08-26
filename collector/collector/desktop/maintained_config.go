package main

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	logmasterconfig "logmaster-agent/agent/configs/logmaster"
)

type maintainedProjectsFile struct {
	SchemaVersion int              `yaml:"schema_version"`
	Projects      []CatalogProject `yaml:"projects"`
}

type maintainedTasksFile struct {
	SchemaVersion int           `yaml:"schema_version"`
	Tasks         []CatalogTask `yaml:"tasks"`
}

type maintainedKeywordsFile struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Profiles      []CatalogKeywordProfile `yaml:"profiles"`
}

type editableKeywordsFile struct {
	SchemaVersion int                      `yaml:"schema_version"`
	Profiles      []editableKeywordProfile `yaml:"profiles"`
}

type editableKeywordProfile struct {
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Rules  []editableKeywordRule  `yaml:"rules,omitempty"`
	Groups []editableKeywordGroup `yaml:"groups,omitempty"`
}

type editableKeywordGroup struct {
	ID    string                `yaml:"id"`
	Name  string                `yaml:"name"`
	Scope string                `yaml:"scope,omitempty"`
	Rules []editableKeywordRule `yaml:"rules"`
}

type editableKeywordRule struct {
	ID            string `yaml:"id"`
	Text          string `yaml:"text"`
	Name          string `yaml:"name,omitempty"`
	Mode          string `yaml:"mode,omitempty"`
	CaseSensitive bool   `yaml:"case_sensitive,omitempty"`
}

func (rule editableKeywordRule) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
	addText := func(key, value string) {
		if value == "" {
			return
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}
	addText("id", rule.ID)
	addText("text", rule.Text)
	addText("name", rule.Name)
	addText("mode", rule.Mode)
	if rule.CaseSensitive {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "case_sensitive"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		)
	}
	return node, nil
}

type maintainedDefaultsFile struct {
	SchemaVersion int `yaml:"schema_version"`
	Serial        struct {
		BaudRate int    `yaml:"baud_rate"`
		DataBits int    `yaml:"data_bits"`
		StopBits int    `yaml:"stop_bits"`
		Parity   string `yaml:"parity"`
		DTR      bool   `yaml:"dtr"`
		RTS      bool   `yaml:"rts"`
	} `yaml:"serial"`
	Log struct {
		Directory           string `yaml:"directory"`
		SaveEnabled         bool   `yaml:"save_enabled"`
		UploadEnabled       bool   `yaml:"upload_enabled"`
		NoLogTimeoutSeconds int    `yaml:"no_log_timeout_seconds"`
		MaxWindowLines      int    `yaml:"max_window_lines"`
		FontSize            int    `yaml:"font_size"`
		AutoWrap            bool   `yaml:"auto_wrap"`
		SegmentMinutes      int    `yaml:"segment_minutes"`
		SegmentSizeMB       int64  `yaml:"segment_size_mb"`
	} `yaml:"log"`
	Storage struct {
		LimitGB        int64 `yaml:"limit_gb"`
		WarningPercent int   `yaml:"warning_percent"`
	} `yaml:"storage"`
	Upload struct {
		BackendURL      string `yaml:"backend_url"`
		IntervalSeconds int    `yaml:"interval_seconds"`
		Concurrency     int    `yaml:"concurrency"`
		Gzip            bool   `yaml:"gzip"`
	} `yaml:"upload"`
}

func decodeMaintainedYAML(name string, data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("解析%s: %w", name, err)
	}
	return nil
}

func parseMaintainedCatalog(projectData, taskData, keywordData []byte) (CatalogConfig, error) {
	var projectFile maintainedProjectsFile
	var taskFile maintainedTasksFile
	var keywordFile maintainedKeywordsFile
	if err := decodeMaintainedYAML("项目配置", projectData, &projectFile); err != nil {
		return CatalogConfig{}, err
	}
	if err := decodeMaintainedYAML("测试任务配置", taskData, &taskFile); err != nil {
		return CatalogConfig{}, err
	}
	if err := decodeMaintainedYAML("关键字配置", keywordData, &keywordFile); err != nil {
		return CatalogConfig{}, err
	}
	normalizeMaintainedKeywordRules(keywordFile.Profiles)
	if projectFile.SchemaVersion != 1 || taskFile.SchemaVersion != 1 || keywordFile.SchemaVersion != 1 {
		return CatalogConfig{}, fmt.Errorf("配置文件 schema_version 必须为 1")
	}
	tasks := cloneCatalogTasks(taskFile.Tasks)
	for i := range tasks {
		tasks[i].KeywordProfiles = cloneCatalogProfiles(keywordFile.Profiles)
	}
	projects := append([]CatalogProject(nil), projectFile.Projects...)
	for i := range projects {
		for _, task := range tasks {
			if task.AllProjects || len(task.ApplicableProjects) == 0 || containsFold(task.ApplicableProjects, projects[i].ID) || containsFold(task.ApplicableProjects, projects[i].Name) {
				projects[i].Tasks = append(projects[i].Tasks, cloneCatalogTasks([]CatalogTask{task})...)
			}
		}
	}
	catalog := CatalogConfig{SchemaVersion: 1, Projects: projects}
	if err := validateCatalog(catalog); err != nil {
		return CatalogConfig{}, err
	}
	return catalog, nil
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func normalizeMaintainedKeywordRules(profiles []CatalogKeywordProfile) {
	normalize := func(rule *CatalogKeywordRule) {
		if strings.TrimSpace(rule.Match) == "" {
			rule.Match = strings.TrimSpace(rule.Text)
		}
		if strings.TrimSpace(rule.Name) == "" {
			rule.Name = rule.Match
		}
		if strings.TrimSpace(rule.Mode) == "" {
			rule.Mode = "contains"
		}
		rule.Text = ""
		rule.ReadOnly = strings.HasPrefix(rule.ID, "cloud-rule-")
	}
	for profileIndex := range profiles {
		for ruleIndex := range profiles[profileIndex].Rules {
			normalize(&profiles[profileIndex].Rules[ruleIndex])
		}
		for groupIndex := range profiles[profileIndex].Groups {
			for ruleIndex := range profiles[profileIndex].Groups[groupIndex].Rules {
				normalize(&profiles[profileIndex].Groups[groupIndex].Rules[ruleIndex])
			}
		}
	}
}

func compactMaintainedKeywords(data []byte) ([]byte, error) {
	var source maintainedKeywordsFile
	if err := decodeMaintainedYAML("关键字配置", data, &source); err != nil {
		return nil, err
	}
	toRule := func(rule CatalogKeywordRule) editableKeywordRule {
		item := editableKeywordRule{ID: rule.ID, Text: rule.Match, CaseSensitive: rule.CaseSensitive}
		if strings.TrimSpace(rule.Name) != "" && rule.Name != rule.Match {
			item.Name = rule.Name
		}
		if rule.Mode != "" && rule.Mode != "contains" {
			item.Mode = rule.Mode
		}
		return item
	}
	result := editableKeywordsFile{SchemaVersion: source.SchemaVersion}
	for _, profile := range source.Profiles {
		item := editableKeywordProfile{ID: profile.ID, Name: profile.Name}
		for _, rule := range profile.Rules {
			item.Rules = append(item.Rules, toRule(rule))
		}
		for _, group := range profile.Groups {
			groupItem := editableKeywordGroup{ID: group.ID, Name: group.Name, Scope: group.Scope}
			for _, rule := range group.Rules {
				groupItem.Rules = append(groupItem.Rules, toRule(rule))
			}
			item.Groups = append(item.Groups, groupItem)
		}
		result.Profiles = append(result.Profiles, item)
	}
	encoded, err := yaml.Marshal(result)
	if err != nil {
		return nil, err
	}
	header := []byte("# 精简关键字配置：text 是匹配内容；默认普通包含且不区分大小写。\n# 只有正则匹配或严格区分大小写时才需要填写 mode / case_sensitive。\n")
	return append(header, encoded...), nil
}

func mustLoadMaintainedDefaults() maintainedDefaultsFile {
	var defaults maintainedDefaultsFile
	if err := decodeMaintainedYAML("默认配置", logmasterconfig.DefaultsYAML, &defaults); err != nil {
		panic(err)
	}
	if defaults.SchemaVersion != 1 {
		panic("默认配置 schema_version 必须为 1")
	}
	if err := validateMaintainedDefaults(defaults); err != nil {
		panic(err)
	}
	return defaults
}

func validateMaintainedDefaults(defaults maintainedDefaultsFile) error {
	if defaults.Serial.BaudRate <= 0 || defaults.Serial.DataBits < 5 || defaults.Serial.DataBits > 8 || (defaults.Serial.StopBits != 1 && defaults.Serial.StopBits != 2) {
		return fmt.Errorf("默认串口参数无效")
	}
	validParity := map[string]bool{"none": true, "odd": true, "even": true, "mark": true, "space": true}
	if !validParity[strings.ToLower(strings.TrimSpace(defaults.Serial.Parity))] {
		return fmt.Errorf("默认校验位无效: %s", defaults.Serial.Parity)
	}
	if defaults.Log.NoLogTimeoutSeconds <= 0 || defaults.Log.MaxWindowLines < 100 || defaults.Log.FontSize < 10 || defaults.Log.FontSize > 24 || defaults.Log.SegmentMinutes <= 0 || defaults.Log.SegmentSizeMB <= 0 {
		return fmt.Errorf("默认日志参数无效")
	}
	if defaults.Log.UploadEnabled {
		return fmt.Errorf("默认云端上传必须关闭，开启上传需要通道项目和版本号")
	}
	if defaults.Storage.LimitGB <= 0 || defaults.Storage.WarningPercent < 1 || defaults.Storage.WarningPercent > 99 {
		return fmt.Errorf("默认存储保护参数无效")
	}
	if defaults.Upload.IntervalSeconds <= 0 || defaults.Upload.Concurrency < 1 {
		return fmt.Errorf("默认上传参数无效")
	}
	return nil
}

var packagedDefaults = mustLoadMaintainedDefaults()
