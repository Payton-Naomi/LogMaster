package rolepolicy

import "testing"

func TestForJobTitle(t *testing.T) {
	tests := map[string]string{
		"主任软件测试工程师": SuperAdmin,
		"高级软件测试工程师": Admin,
		"软件工程师":     Developer,
		"助理硬件测试工程师": User,
		"助理软件测试工程师": User,
		"":          User,
	}
	for title, want := range tests {
		if got := ForJobTitle(title); got != want {
			t.Errorf("ForJobTitle(%q) = %q, want %q", title, got, want)
		}
	}
}
