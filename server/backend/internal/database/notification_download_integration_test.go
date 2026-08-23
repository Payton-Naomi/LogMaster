package database_test

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
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

func TestNotificationAndDownloadEndpoints(t *testing.T) {
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
	owner := "ou_feature_owner_" + suffix
	otherUser := "ou_feature_other_" + suffix
	project := "feature-project-" + suffix
	uploadID := testUUID(t)
	taskID := testUUID(t)
	storageRoot := t.TempDir()
	parsedRelativePath := filepath.ToSlash(filepath.Join("items", "1", "extracted", "app.log"))
	parsedContent := []byte("0123456789\nFATAL integration failure\n")
	originalContent := []byte("original upload content")

	writeTestFile(t, filepath.Join(storageRoot, filepath.FromSlash(parsedRelativePath)), parsedContent)
	writeTestFile(t, filepath.Join(storageRoot, "items", "1", "original", "source.log"), originalContent)

	if _, err = db.ExecContext(ctx, `INSERT INTO logmaster_api.users (feishu_open_id,name,email)
		VALUES ($1,'feature owner','owner@example.com'),($2,'feature other','other@example.com')`, owner, otherUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO logmaster_api.projects (name,is_active) VALUES ($1,TRUE)`, project); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx := context.Background()
		if _, cleanupErr := db.ExecContext(cleanupCtx, `DELETE FROM logmaster_api.log_uploads WHERE id=$1`, uploadID); cleanupErr != nil {
			t.Errorf("clean test upload: %v", cleanupErr)
		}
		if _, cleanupErr := db.ExecContext(cleanupCtx, `DELETE FROM logmaster_api.users WHERE feishu_open_id IN ($1,$2)`, owner, otherUser); cleanupErr != nil {
			t.Errorf("clean test users: %v", cleanupErr)
		}
		if _, cleanupErr := db.ExecContext(cleanupCtx, `DELETE FROM logmaster_api.projects WHERE name=$1`, project); cleanupErr != nil {
			t.Errorf("clean test project: %v", cleanupErr)
		}
	}()

	repository := logservice.NewRepository(db)
	if err = repository.CreateUpload(ctx, uploadID, taskID, project, "v-test", nil, storageRoot, owner); err != nil {
		t.Fatal(err)
	}
	if err = repository.QueueUpload(ctx, uploadID, "source.log", int64(len(originalContent)), []logservice.LogFile{{
		RelativePath: parsedRelativePath, SizeBytes: int64(len(parsedContent)), SHA256: strings.Repeat("0", 64),
	}}); err != nil {
		t.Fatal(err)
	}
	_, files, err := repository.GetUpload(ctx, uploadID, owner)
	if err != nil || len(files) != 1 {
		t.Fatalf("load test upload: files=%d err=%v", len(files), err)
	}
	fileID := files[0].ID
	if err = repository.SaveFileResults(ctx, taskID, fileID, 2, 1, 0, []logservice.ParseResult{{
		Level: "critical", MatchedText: "FATAL", LineNumber: 2, Content: "FATAL integration failure",
		FilePath: parsedRelativePath, RuleName: "integration rule", Category: "system",
	}}); err != nil {
		t.Fatal(err)
	}
	var resultID int64
	if err = db.QueryRowContext(ctx, `SELECT id FROM logmaster_api.parse_results WHERE task_id=$1`, taskID).Scan(&resultID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE logmaster_api.parse_tasks SET status='completed',attempt_no=1 WHERE id=$1`, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE logmaster_api.log_uploads SET status='completed' WHERE id=$1`, uploadID); err != nil {
		t.Fatal(err)
	}

	activeUser := owner
	service := logservice.NewServiceWithAgent(config.Config{}, repository, nil)
	service.SetCurrentUserResolver(func(*http.Request) (string, bool) { return activeUser, true })
	mux := http.NewServeMux()
	service.RegisterRoutes(mux)

	t.Run("download types and range", func(t *testing.T) {
		invalidType := httptest.NewRecorder()
		mux.ServeHTTP(invalidType, httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/logs/%s/download?type=unknown", uploadID), nil))
		if invalidType.Code != http.StatusBadRequest {
			t.Fatalf("invalid download type status=%d", invalidType.Code)
		}
		missingFileID := httptest.NewRecorder()
		mux.ServeHTTP(missingFileID, httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/logs/%s/download?type=file", uploadID), nil))
		if missingFileID.Code != http.StatusBadRequest {
			t.Fatalf("missing file_id status=%d", missingFileID.Code)
		}

		rangeRequest := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/logs/%s/download?type=file&file_id=%d", uploadID, fileID), nil)
		rangeRequest.Header.Set("Range", "bytes=2-5")
		rangeResponse := httptest.NewRecorder()
		mux.ServeHTTP(rangeResponse, rangeRequest)
		if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.String() != "2345" {
			t.Fatalf("range download status=%d body=%q", rangeResponse.Code, rangeResponse.Body.String())
		}
		if rangeResponse.Header().Get("Content-Range") != "bytes 2-5/37" {
			t.Fatalf("Content-Range = %q", rangeResponse.Header().Get("Content-Range"))
		}
		legacyFile := httptest.NewRecorder()
		mux.ServeHTTP(legacyFile, httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/logs/%s/download?file_id=%d", uploadID, fileID), nil))
		if legacyFile.Code != http.StatusOK || !bytes.Equal(legacyFile.Body.Bytes(), parsedContent) {
			t.Fatalf("legacy file download status=%d body=%q", legacyFile.Code, legacyFile.Body.Bytes())
		}

		batch := downloadZIP(t, mux, fmt.Sprintf("/api/logs/%s/download?type=batch", uploadID))
		if got := batch[parsedRelativePath]; !bytes.Equal(got, parsedContent) {
			t.Fatalf("batch content = %q", got)
		}
		original := downloadZIP(t, mux, fmt.Sprintf("/api/logs/%s/download?type=original", uploadID))
		if got := original["items/1/source.log"]; !bytes.Equal(got, originalContent) {
			t.Fatalf("original content = %q", got)
		}
		results := downloadZIP(t, mux, fmt.Sprintf("/api/logs/%s/download?type=results", uploadID))
		for _, name := range []string{"results.csv", "data.json", "report.md"} {
			if len(results[name]) == 0 {
				t.Fatalf("result archive is missing %s", name)
			}
		}
		if !bytes.Contains(results["results.csv"], []byte("FATAL")) || !bytes.Contains(results["report.md"], []byte("integration rule")) {
			t.Fatal("result archive does not contain the parsed finding")
		}

		activeUser = otherUser
		forbidden := httptest.NewRecorder()
		mux.ServeHTTP(forbidden, httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/api/logs/%s/download?type=batch", uploadID), nil))
		activeUser = owner
		if forbidden.Code != http.StatusNotFound {
			t.Fatalf("cross-user download status=%d, want %d", forbidden.Code, http.StatusNotFound)
		}
	})

	t.Run("AI aggregate status and notification dedupe", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `INSERT INTO logmaster_api.ai_jobs
			(task_id,attempt_no,job_type,log_file_id,owner_open_id,status,error_message)
			VALUES ($1,1,'file',$2,$3,'completed',''),($1,1,'overview',NULL,$3,'failed','总览失败')`,
			taskID, fileID, owner); err != nil {
			t.Fatal(err)
		}
		if err := repository.ReconcileAIQueue(ctx); err != nil {
			t.Fatal(err)
		}
		assertTaskAIStatus(t, db, taskID, "partial_failed", "总览失败")
		if err := repository.CreatePendingAINotifications(ctx, taskID); err != nil {
			t.Fatal(err)
		}
		if err := repository.CreatePendingAINotifications(ctx, taskID); err != nil {
			t.Fatal(err)
		}
		assertNotificationCount(t, db, owner, taskID, "ai_failed", 1)

		if _, err := db.ExecContext(ctx, `UPDATE logmaster_api.ai_jobs SET status='completed',error_message='' WHERE task_id=$1`, taskID); err != nil {
			t.Fatal(err)
		}
		if err := repository.ReconcileAIQueue(ctx); err != nil {
			t.Fatal(err)
		}
		assertTaskAIStatus(t, db, taskID, "completed", "")
		if err := repository.CreatePendingAINotifications(ctx, taskID); err != nil {
			t.Fatal(err)
		}
		assertNotificationCount(t, db, owner, taskID, "ai_completed", 1)

		taskResponse := httptest.NewRecorder()
		mux.ServeHTTP(taskResponse, httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID, nil))
		if taskResponse.Code != http.StatusOK || !strings.Contains(taskResponse.Body.String(), `"ai_status":"completed"`) {
			t.Fatalf("task AI response status=%d body=%s", taskResponse.Code, taskResponse.Body.String())
		}
	})

	t.Run("notification settings and read endpoints", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `DELETE FROM logmaster_api.notifications WHERE recipient_open_id=$1`, owner); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO logmaster_api.notifications
			(recipient_open_id,task_id,upload_id,type,title,message)
			VALUES ($1,$2,$3,'task_completed','测试通知一','one'),
			       ($1,$2,$3,'task_cancelled','测试通知二','two')`, owner, taskID, uploadID); err != nil {
			t.Fatal(err)
		}
		settingsBody := `{"task_completed":true,"task_failed":true,"task_cancelled":false,"ai_completed":true,"ai_failed":false,"result_assigned":true,"result_commented":false}`
		settingsResponse := httptest.NewRecorder()
		mux.ServeHTTP(settingsResponse, httptest.NewRequest(http.MethodPut, "/api/notification-settings", strings.NewReader(settingsBody)))
		if settingsResponse.Code != http.StatusOK || !strings.Contains(settingsResponse.Body.String(), `"ai_failed":false`) {
			t.Fatalf("save settings status=%d body=%s", settingsResponse.Code, settingsResponse.Body.String())
		}
		getSettings := httptest.NewRecorder()
		mux.ServeHTTP(getSettings, httptest.NewRequest(http.MethodGet, "/api/notification-settings", nil))
		if getSettings.Code != http.StatusOK || !strings.Contains(getSettings.Body.String(), `"result_commented":false`) {
			t.Fatalf("get settings status=%d body=%s", getSettings.Code, getSettings.Body.String())
		}

		listResponse := httptest.NewRecorder()
		mux.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/notifications?unread_only=true", nil))
		if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"unread":2`) {
			t.Fatalf("list notifications status=%d body=%s", listResponse.Code, listResponse.Body.String())
		}
		var notificationID int64
		if err := db.QueryRowContext(ctx, `SELECT MIN(id) FROM logmaster_api.notifications WHERE recipient_open_id=$1`, owner).Scan(&notificationID); err != nil {
			t.Fatal(err)
		}
		activeUser = otherUser
		forbidden := httptest.NewRecorder()
		mux.ServeHTTP(forbidden, httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/api/notifications/%d/read", notificationID), nil))
		activeUser = owner
		if forbidden.Code != http.StatusNotFound {
			t.Fatalf("cross-user mark-read status=%d", forbidden.Code)
		}
		markRead := httptest.NewRecorder()
		mux.ServeHTTP(markRead, httptest.NewRequest(http.MethodPatch,
			fmt.Sprintf("/api/notifications/%d/read", notificationID), nil))
		if markRead.Code != http.StatusOK {
			t.Fatalf("mark-read status=%d body=%s", markRead.Code, markRead.Body.String())
		}
		readAll := httptest.NewRecorder()
		mux.ServeHTTP(readAll, httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil))
		if readAll.Code != http.StatusOK || !strings.Contains(readAll.Body.String(), `"updated":1`) {
			t.Fatalf("read-all status=%d body=%s", readAll.Code, readAll.Body.String())
		}
	})

	t.Run("notification event settings are enforced", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, `DELETE FROM logmaster_api.notifications
			WHERE recipient_open_id=$1 AND task_id=$2 AND type IN ('task_completed','task_failed')`, owner, taskID); err != nil {
			t.Fatal(err)
		}
		ownerSettings, err := repository.GetNotificationSettings(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		ownerSettings.TaskFailed = false
		if _, err := repository.SaveNotificationSettings(ctx, owner, ownerSettings); err != nil {
			t.Fatal(err)
		}
		if err := repository.CreateTaskNotifications(ctx, taskID, "task_failed", "不应出现", "通知已关闭"); err != nil {
			t.Fatal(err)
		}
		assertNotificationCount(t, db, owner, taskID, "task_failed", 0)
		if err := repository.CreateTaskNotifications(ctx, taskID, "task_completed", "任务完成", "测试通知去重"); err != nil {
			t.Fatal(err)
		}
		if err := repository.CreateTaskNotifications(ctx, taskID, "task_completed", "任务完成", "测试通知去重"); err != nil {
			t.Fatal(err)
		}
		assertNotificationCount(t, db, owner, taskID, "task_completed", 1)

		assignment := httptest.NewRecorder()
		assignmentBody := fmt.Sprintf(`{"assigned_to":%q}`, otherUser)
		mux.ServeHTTP(assignment, httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/api/results/%d/assignment", resultID), strings.NewReader(assignmentBody)))
		if assignment.Code != http.StatusOK {
			t.Fatalf("assignment status=%d body=%s", assignment.Code, assignment.Body.String())
		}
		assertNotificationCount(t, db, otherUser, taskID, "result_assigned", 1)

		otherSettings := logservice.NotificationSettings{
			TaskCompleted: true, TaskFailed: true, TaskCancelled: true,
			AICompleted: true, AIFailed: true, ResultAssigned: true, ResultCommented: false,
		}
		if _, err := repository.SaveNotificationSettings(ctx, otherUser, otherSettings); err != nil {
			t.Fatal(err)
		}
		addResultComment(t, mux, resultID, "suppressed comment")
		assertNotificationCount(t, db, otherUser, taskID, "result_commented", 0)
		otherSettings.ResultCommented = true
		if _, err := repository.SaveNotificationSettings(ctx, otherUser, otherSettings); err != nil {
			t.Fatal(err)
		}
		addResultComment(t, mux, resultID, "visible comment")
		assertNotificationCount(t, db, otherUser, taskID, "result_commented", 1)
	})

	t.Run("SSE delivers a new database notification", func(t *testing.T) {
		activeUser = owner
		server := httptest.NewServer(mux)
		defer server.Close()
		streamCtx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, server.URL+"/api/notifications/stream", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" {
			t.Fatalf("SSE status=%d content-type=%q", response.StatusCode, response.Header.Get("Content-Type"))
		}
		reader := bufio.NewReader(response.Body)
		if line, err := reader.ReadString('\n'); err != nil || line != ": connected\n" {
			t.Fatalf("SSE greeting=%q err=%v", line, err)
		}
		var expectedID int64
		if err := db.QueryRowContext(ctx, `INSERT INTO logmaster_api.notifications
			(recipient_open_id,task_id,upload_id,result_id,type,title,message)
			VALUES ($1,$2,$3,$4,'result_commented','集成测试通知','SSE notification') RETURNING id`,
			owner, taskID, uploadID, resultID).Scan(&expectedID); err != nil {
			t.Fatal(err)
		}
		var event strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("read SSE event: %v; event=%q", err, event.String())
			}
			event.WriteString(line)
			if line == "\n" && strings.Contains(event.String(), "event: notification") {
				break
			}
		}
		if !strings.Contains(event.String(), fmt.Sprintf("id: %d", expectedID)) ||
			!strings.Contains(event.String(), "SSE notification") {
			t.Fatalf("unexpected SSE event: %s", event.String())
		}
	})
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func downloadZIP(t *testing.T, handler http.Handler, target string) map[string][]byte {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("download %s status=%d body=%s", target, response.Code, response.Body.String())
	}
	reader, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatalf("open ZIP %s: %v", target, err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(input)
		closeErr := input.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read ZIP entry %s: read=%v close=%v", file.Name, readErr, closeErr)
		}
		files[file.Name] = content
	}
	return files
}

func assertTaskAIStatus(t *testing.T, db *sql.DB, taskID, wantStatus, wantError string) {
	t.Helper()
	var status, errorMessage string
	if err := db.QueryRowContext(context.Background(),
		`SELECT ai_status,ai_error_message FROM logmaster_api.parse_tasks WHERE id=$1`, taskID).
		Scan(&status, &errorMessage); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || errorMessage != wantError {
		t.Fatalf("AI state=(%q,%q), want (%q,%q)", status, errorMessage, wantStatus, wantError)
	}
}

func assertNotificationCount(t *testing.T, db *sql.DB, recipient, taskID, notificationType string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM logmaster_api.notifications
		WHERE recipient_open_id=$1 AND task_id=$2 AND type=$3`, recipient, taskID, notificationType).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s notification count=%d, want %d", notificationType, count, want)
	}
}

func addResultComment(t *testing.T, handler http.Handler, resultID int64, comment string) {
	t.Helper()
	response := httptest.NewRecorder()
	body := fmt.Sprintf(`{"comment":%q}`, comment)
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/results/%d/comments", resultID), strings.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("add comment status=%d body=%s", response.Code, response.Body.String())
	}
}
