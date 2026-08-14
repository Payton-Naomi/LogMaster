package admin

import "testing"

func TestRolePermissionMatrix(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		allowed    bool
	}{
		{roleDeveloper, permissionKeywords, true},
		{roleDeveloper, permissionApprovals, true},
		{roleDeveloper, permissionProjects, false},
		{roleAdmin, permissionProjects, true},
		{roleAdmin, permissionApprovals, true},
		{roleAdmin, permissionKeywords, true},
		{roleAdmin, permissionUsers, false},
		{roleSuperAdmin, permissionUsers, true},
		{roleSuperAdmin, permissionCapacity, true},
		{roleUser, permissionKeywords, false},
	}
	for _, test := range tests {
		if actual := roleHasPermission(test.role, test.permission); actual != test.allowed {
			t.Fatalf("role %s permission %s = %v, want %v", test.role, test.permission, actual, test.allowed)
		}
	}
}

func TestOnlySuperAdminCanReviewPermissionRequests(t *testing.T) {
	for _, role := range []string{roleUser, roleDeveloper, roleAdmin} {
		if canReviewPermissionRequests(role) {
			t.Fatalf("role %s must not review permission requests", role)
		}
	}
	if !canReviewPermissionRequests(roleSuperAdmin) {
		t.Fatal("super admin must be able to review permission requests")
	}
}
