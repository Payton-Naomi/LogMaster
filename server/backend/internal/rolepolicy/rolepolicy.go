package rolepolicy

import "strings"

const (
	User       = "user"
	Developer  = "developer"
	Admin      = "admin"
	SuperAdmin = "super_admin"
)

// ForJobTitle derives the Feishu-managed role from the employee's current job title.
func ForJobTitle(jobTitle string) string {
	title := strings.TrimSpace(jobTitle)
	switch {
	case strings.Contains(title, "主任"):
		return SuperAdmin
	case strings.Contains(title, "高级"):
		return Admin
	case strings.Contains(title, "软件工程师"), strings.Contains(title, "硬件工程师"):
		return Developer
	default:
		return User
	}
}
