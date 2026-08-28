package auth

import (
	"testing"

	"logmaster-agent/internal/config"
)

func TestRoleForFeishuUserUsesFixedJobTitlePolicy(t *testing.T) {
	service := &Service{}
	tests := map[string]string{
		"主任软件测试工程师": "super_admin",
		"高级硬件工程师":   "admin",
		"软件工程师":     "developer",
		"助理软件测试工程师": "user",
	}
	for title, want := range tests {
		if got := service.roleForFeishuUser("ou_any", "任意姓名", title); got != want {
			t.Errorf("roleForFeishuUser(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestConfiguredSuperAdminsSupportMultipleIDsAndNames(t *testing.T) {
	service := &Service{config: config.Config{
		FeishuSuperAdminIDs:   "ou_first, ou_second",
		FeishuSuperAdminNames: "王占赢, 王占赢二",
	}}
	for _, user := range []struct {
		openID string
		name   string
	}{
		{openID: "ou_first", name: "普通用户"},
		{openID: "ou_second", name: "普通用户"},
		{openID: "ou_other", name: "王占赢二"},
	} {
		if !service.isConfiguredSuperAdmin(user.openID, user.name) {
			t.Errorf("expected configured super admin: %+v", user)
		}
	}
	if service.isConfiguredSuperAdmin("ou_other", "普通用户") {
		t.Fatal("unexpected super admin match")
	}
}
