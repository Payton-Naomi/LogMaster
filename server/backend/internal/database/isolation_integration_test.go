package database_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"logmaster-agent/internal/config"
	"logmaster-agent/internal/database"
	"logmaster-agent/internal/logservice"
)

func TestUserUploadAndRuleIsolation(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userOne := "ou_isolation_one_" + suffix
	userTwo := "ou_isolation_two_" + suffix
	uploadOne := testUUID(t)
	uploadTwo := testUUID(t)
	taskOne := testUUID(t)
	taskTwo := testUUID(t)
	projectOne := "isolation-project-one-" + suffix
	projectTwo := "isolation-project-two-" + suffix

	_, err = db.ExecContext(ctx, `INSERT INTO logmaster_api.users (feishu_open_id, name)
		VALUES ($1, 'isolation one'), ($2, 'isolation two')`, userOne, userTwo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO logmaster_api.projects (name, is_active) VALUES ($1, TRUE), ($2, TRUE)`, projectOne, projectTwo); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM logmaster_api.log_uploads WHERE id IN ($1, $2)`, uploadOne, uploadTwo)
		_, _ = db.ExecContext(ctx, `DELETE FROM logmaster_api.parse_rules WHERE created_by_open_id IN ($1, $2)`, userOne, userTwo)
		_, _ = db.ExecContext(ctx, `DELETE FROM logmaster_api.users WHERE feishu_open_id IN ($1, $2)`, userOne, userTwo)
		_, _ = db.ExecContext(ctx, `DELETE FROM logmaster_api.projects WHERE name IN ($1, $2)`, projectOne, projectTwo)
	}()

	repository := logservice.NewRepository(db)
	storageOne := t.TempDir()
	storedRelativePath := filepath.ToSlash(filepath.Join("items", "0", "sample.log"))
	storedPath := filepath.Join(storageOne, filepath.FromSlash(storedRelativePath))
	if err := os.MkdirAll(filepath.Dir(storedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedPath, []byte("INFO preview line\nERROR preview failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateUpload(ctx, uploadOne, taskOne, projectOne, "v1", nil, storageOne, userOne); err != nil {
		t.Fatal(err)
	}
	if err := repository.QueueUpload(ctx, uploadOne, "sample.log", 40, []logservice.LogFile{{RelativePath: storedRelativePath, SizeBytes: 40}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateUpload(ctx, uploadTwo, taskTwo, projectTwo, "v2", nil, "test/path/two", userTwo); err != nil {
		t.Fatal(err)
	}
	uploads, total, err := repository.ListUploads(ctx, userOne, "", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(uploads) != 1 || uploads[0].ID != uploadOne {
		t.Fatalf("user one uploads = %#v, total = %d", uploads, total)
	}
	if _, _, err := repository.GetUpload(ctx, uploadTwo, userOne); err != sql.ErrNoRows {
		t.Fatalf("cross-user upload lookup error = %v, want sql.ErrNoRows", err)
	}
	detail, _, err := repository.GetUploadByTask(ctx, taskOne, userOne)
	if err != nil {
		t.Fatalf("query task detail: %v", err)
	}
	if detail.ID != uploadOne || detail.TaskID != taskOne {
		t.Fatalf("unexpected task detail: %#v", detail)
	}
	if _, _, err := repository.GetUploadByTask(ctx, taskTwo, userOne); err != sql.ErrNoRows {
		t.Fatalf("cross-user task detail error = %v, want sql.ErrNoRows", err)
	}
	service := logservice.NewServiceWithAgent(config.Config{}, repository, nil)
	activeUser := userOne
	service.SetCurrentUserResolver(func(*http.Request) (string, bool) { return activeUser, true })
	mux := http.NewServeMux()
	service.RegisterRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskOne, nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("task detail endpoint status = %d, body = %s", response.Code, response.Body.String())
	}
	previewRequest := httptest.NewRequest(http.MethodGet, "/api/logs/"+uploadOne+"/preview", nil)
	previewResponse := httptest.NewRecorder()
	mux.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || !strings.Contains(previewResponse.Body.String(), "ERROR preview failure") {
		t.Fatalf("preview endpoint status = %d, body = %s", previewResponse.Code, previewResponse.Body.String())
	}
	activeUser = userTwo
	forbiddenPreview := httptest.NewRecorder()
	mux.ServeHTTP(forbiddenPreview, httptest.NewRequest(http.MethodGet, "/api/logs/"+uploadOne+"/preview", nil))
	if forbiddenPreview.Code != http.StatusNotFound {
		t.Fatalf("cross-user preview status = %d, want %d", forbiddenPreview.Code, http.StatusNotFound)
	}
	activeUser = userOne
	stats, err := repository.Dashboard(ctx, userOne, 7)
	if err != nil {
		t.Fatalf("query user dashboard: %v", err)
	}
	if stats.TaskCount != 1 || len(stats.RecentTasks) != 1 || stats.RecentTasks[0].ID != uploadOne {
		t.Fatalf("user one dashboard contains unexpected tasks: %#v", stats)
	}

	created, err := repository.SaveRule(ctx, userOne, logservice.ParseRule{
		Name: "private rule", Category: "system", Keyword: "private-keyword",
		Level: "warning", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Enabled {
		t.Fatal("new warning rule must be enabled")
	}
	if containsRule(t, repository, ctx, userTwo, created.ID) {
		t.Fatal("user-created rule is visible to another user")
	}
	infoRule, err := repository.SaveRule(ctx, userOne, logservice.ParseRule{
		Name: "private info rule", Category: "system", Keyword: "private-info-keyword",
		Level: "info", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if infoRule.Enabled {
		t.Fatal("new info rule must be disabled")
	}

	rules, err := repository.ListRules(ctx, userOne)
	if err != nil {
		t.Fatal(err)
	}
	var systemRule *logservice.ParseRule
	for index := range rules {
		if rules[index].ID != created.ID && rules[index].ID != infoRule.ID && rules[index].Level == "info" {
			systemRule = &rules[index]
			break
		}
	}
	if systemRule == nil {
		t.Skip("no system rule is configured")
	}
	systemRule.Enabled = true
	if _, err := repository.SaveRule(ctx, userOne, *systemRule); err != nil {
		t.Fatal(err)
	}
	if !ruleEnabled(t, repository, ctx, userOne, systemRule.ID) {
		t.Fatal("system rule was not enabled for user one")
	}
	if ruleEnabled(t, repository, ctx, userTwo, systemRule.ID) {
		t.Fatal("user one's rule setting affected user two")
	}
}

func testUUID(t *testing.T) string {
	t.Helper()
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}

func containsRule(t *testing.T, repository *logservice.Repository, ctx context.Context, user string, ruleID int64) bool {
	t.Helper()
	rules, err := repository.ListRules(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.ID == ruleID {
			return true
		}
	}
	return false
}

func ruleEnabled(t *testing.T, repository *logservice.Repository, ctx context.Context, user string, ruleID int64) bool {
	t.Helper()
	rules, err := repository.ListRules(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.ID == ruleID {
			return rule.Enabled
		}
	}
	t.Fatalf("rule %d is not visible to %s", ruleID, user)
	return false
}
