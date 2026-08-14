package main

import "testing"

func TestApplyLogConfigDefaultsLeavesBusinessSelectionBlank(t *testing.T) {
	service := &Service{
		appSettings: AppSettingsDTO{DefaultSaveEnabled: true, NoLogTimeoutSeconds: 300},
		catalog: CatalogConfig{Projects: []CatalogProject{{
			ID: "project-a", Name: "Project A", Versions: []string{"V1.0.0"},
			Tasks: []CatalogTask{
				{ID: "special", Name: "专项测试", Type: "special"},
				{ID: "normal", Name: "普通挂测", Type: "normal", KeywordProfiles: []CatalogKeywordProfile{{ID: "errors", Name: "默认异常", Rules: []CatalogKeywordRule{{ID: "timeout"}, {ID: "reboot"}}}}},
			},
		}}},
	}
	dto := service.applyLogConfigDefaultsLocked(DeviceConfigDTO{})
	if dto.ProjectID != "" || dto.Version != "" || dto.TestTaskID != "" || dto.KeywordProfileID != "" {
		t.Fatalf("business fields must start blank: %+v", dto)
	}
	if !dto.SaveEnabled || dto.UploadEnabled || dto.KeywordMatchingEnabled {
		t.Fatalf("unexpected default switches: %+v", dto)
	}
	if len(dto.KeywordRuleIDs) != 0 {
		t.Fatalf("unexpected rules: %#v", dto.KeywordRuleIDs)
	}
	if dto.Configured {
		t.Fatal("prefilled defaults must not be treated as saved configuration")
	}
}

func TestSaveDeviceConfigAllowsBlankBusinessSelection(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	dto := normalizeDeviceConfig(DeviceConfigDTO{DeviceID: "COM77", Name: "串口 COM77", PortName: "COM77", SaveEnabled: true})
	if err := service.SaveDeviceConfig(dto.DeviceID, dto); err != nil {
		t.Fatalf("save blank business selection: %v", err)
	}
	if !service.configs[dto.DeviceID].Configured {
		t.Fatal("saved serial configuration must be marked configured")
	}
}

func TestSaveDeviceConfigRequiresProjectAndVersionForUpload(t *testing.T) {
	service, err := newServiceAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdown()
	dto := normalizeDeviceConfig(DeviceConfigDTO{DeviceID: "COM78", Name: "串口 COM78", PortName: "COM78", SaveEnabled: true, UploadEnabled: true})
	if err := service.SaveDeviceConfig(dto.DeviceID, dto); err == nil {
		t.Fatal("upload without project and version must be rejected")
	}
}

func TestUploadPrerequisitesRequireUploaderName(t *testing.T) {
	dto := DeviceConfigDTO{SaveEnabled: true, UploadEnabled: true, ProjectID: "project-a", Version: "V1.0.0", UploaderName: "   "}
	if err := validateUploadPrerequisites(dto); err == nil {
		t.Fatal("upload without uploader name must be rejected")
	}
	dto.UploaderName = "张三"
	if err := validateUploadPrerequisites(dto); err != nil {
		t.Fatalf("upload with uploader name should pass prerequisite validation: %v", err)
	}
}

func TestCatalogSelectionsAreIndependent(t *testing.T) {
	catalog := CatalogConfig{Projects: []CatalogProject{
		{ID: "project-a", Name: "项目一", Tasks: []CatalogTask{{ID: "task-a", Name: "任务一"}}},
		{ID: "project-b", Name: "项目二", Tasks: []CatalogTask{{
			ID: "task-b", Name: "任务二", KeywordProfiles: []CatalogKeywordProfile{{
				ID: "profile-b", Name: "方案二", Rules: []CatalogKeywordRule{{ID: "rule-b", Name: "规则二", Match: "keyword", Mode: "contains"}},
			}},
		}}},
	}}
	dto := DeviceConfigDTO{ProjectID: "project-a", TestTaskID: "task-b", KeywordProfileID: "profile-b", KeywordRuleIDs: []string{"rule-b"}}
	if !catalogSelectionValidIn(catalog, dto) {
		t.Fatal("项目、测试任务和关键字方案应允许独立选择")
	}
	dto.ProjectID = ""
	if !catalogSelectionValidIn(catalog, dto) {
		t.Fatal("未选择项目时也应允许保存测试任务和关键字方案")
	}
	service := &Service{catalog: catalog}
	dto.KeywordMatchingEnabled = true
	rules := service.catalogRulesFor(dto)
	if len(rules) != 1 || rules[0].ID != "rule-b" {
		t.Fatalf("应按独立选择的关键字方案加载规则，实际为 %#v", rules)
	}
}
