package logservice

import (
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestUploadUserAcceptsConfiguredBearerToken(t *testing.T) {
	service := &Service{}
	service.SetUploadAuthenticator("secret-token", "ou_collector")
	request := httptest.NewRequest("POST", "/api/logs/upload", nil)
	request.Header.Set("Authorization", "Bearer secret-token")

	openID, ok := service.uploadUser(request)
	if !ok || openID != "ou_collector" {
		t.Fatalf("upload user = %q, %v", openID, ok)
	}
}

func TestUploadUserAcceptsBuiltinBearerToken(t *testing.T) {
	service := &Service{}
	service.SetUploadAuthenticator("", "ou_collector")
	request := httptest.NewRequest("POST", "/api/logs/upload", nil)
	request.Header.Set("Authorization", "Bearer "+builtinUploadToken)

	openID, ok := service.uploadUser(request)
	if !ok || openID != builtinUploadOwnerOpenID {
		t.Fatalf("upload user = %q, %v", openID, ok)
	}
}

func TestUploadUserRejectsInvalidBearerToken(t *testing.T) {
	service := &Service{}
	service.SetUploadAuthenticator("secret-token", "ou_collector")
	request := httptest.NewRequest("POST", "/api/logs/upload", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")

	if openID, ok := service.uploadUser(request); ok || openID != "" {
		t.Fatalf("upload user = %q, %v", openID, ok)
	}
}

func TestNewQueryCodeFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9A-F]{10}$`)
	prefixed := regexp.MustCompile(`^DR2860-[0-9A-F]{10}$`)
	first := newQueryCode("")
	second := newQueryCode("")
	if !pattern.MatchString(first) || !pattern.MatchString(second) || first == second {
		t.Fatalf("unexpected query codes %q and %q", first, second)
	}
	prefixedCode := newQueryCode("DR2860")
	if !prefixed.MatchString(prefixedCode) {
		t.Fatalf("project prefix missing in query code %q", prefixedCode)
	}
	if code := newQueryCode("dr2860"); !prefixed.MatchString(code) {
		t.Fatalf("project prefix must be normalized to uppercase: %q", code)
	}
	if code := newQueryCode("项目A-1 测试"); !regexp.MustCompile(`^[0-9A-Z]{1,16}-[0-9A-F]{10}$`).MatchString(code) {
		t.Fatalf("project prefix must only keep alphanumeric characters: %q", code)
	}
}

func TestQueryCodePrefix(t *testing.T) {
	cases := map[string]string{
		"DR2860":         "DR2860",
		"dr2860":         "DR2860",
		"  DR-2860 v2  ": "DR2860V2",
		"项目":             "",
		"":               "",
	}
	for input, want := range cases {
		if got := queryCodePrefix(input); got != want {
			t.Fatalf("queryCodePrefix(%q) = %q, want %q", input, got, want)
		}
	}
	long := queryCodePrefix("ABCDEFGHIJKLMNOPQRSTUVWXYZ123456")
	if len(long) != 16 {
		t.Fatalf("queryCodePrefix must truncate to 16 characters, got %q", long)
	}
}
