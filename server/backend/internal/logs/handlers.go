package logs

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"logmaster-agent/internal/response"
)

type storedUploadItem struct {
	index      int
	storedPath string
	itemRoot   string
}

func (s *Service) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	creatorOpenID, ok := s.uploadUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	capacity, err := s.repo.UploadCapacity(r.Context(), s.config.MaxUploadBytes, s.config.MaxFilesPerUpload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query upload capacity failed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, capacity.MaxUploadBytes+(16<<20))
	var gzipBody *gzip.Reader
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
		var err error
		gzipBody, err = gzip.NewReader(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid gzip upload")
			return
		}
		defer gzipBody.Close()
		r.Body = http.MaxBytesReader(w, gzipBody, capacity.MaxUploadBytes+(16<<20))
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload or upload size exceeded")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	headers := r.MultipartForm.File["file"]
	headers = append(headers, r.MultipartForm.File["files"]...)
	if len(headers) == 0 {
		writeError(w, http.StatusBadRequest, "at least one file is required")
		return
	}
	if len(headers) > capacity.MaxFilesPerUpload {
		writeError(w, http.StatusBadRequest, "too many files in one upload")
		return
	}
	projectName := strings.TrimSpace(r.FormValue("project_name"))
	if projectName == "" {
		writeError(w, http.StatusBadRequest, "project_name is required")
		return
	}
	if len(projectName) > 128 {
		writeError(w, http.StatusBadRequest, "project_name is too long")
		return
	}
	version := strings.TrimSpace(r.FormValue("version"))
	if version == "" || len(version) > 64 {
		writeError(w, http.StatusBadRequest, "version is required and must not exceed 64 characters")
		return
	}
	requestedQueryCode := strings.ToUpper(strings.TrimSpace(r.FormValue("query_code")))
	uploadSessionID := strings.TrimSpace(r.FormValue("upload_session_id"))
	metadata, err := uploadMetadataFromForm(r, creatorOpenID, projectName, version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if metadata.ClientRequestID != "" {
		existingUploadID, existingTaskID, existingQueryCode, existingStatus, existingFileCount, findErr := s.repo.FindUploadByClientRequestID(r.Context(), creatorOpenID, metadata.ClientRequestID)
		if findErr == nil {
			response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "upload already accepted", Data: map[string]any{
				"upload_id": existingUploadID, "task_id": existingTaskID, "query_code": existingQueryCode, "status": existingStatus,
				"file_count": existingFileCount, "client_request_id": metadata.ClientRequestID,
			}})
			return
		}
		if !errors.Is(findErr, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "check upload idempotency failed")
			return
		}
	}
	var scenarioIDs []string
	if encoded := strings.TrimSpace(r.FormValue("scenario_ids")); encoded != "" {
		if err := json.Unmarshal([]byte(encoded), &scenarioIDs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid scenario_ids")
			return
		}
	} else if legacyID := strings.TrimSpace(r.FormValue("scenario_id")); legacyID != "" {
		scenarioIDs = []string{legacyID}
	}
	if len(scenarioIDs) > 20 {
		writeError(w, http.StatusBadRequest, "too many test scenarios")
		return
	}
	seenScenarioIDs := make(map[string]struct{}, len(scenarioIDs))
	uniqueScenarioIDs := make([]string, 0, len(scenarioIDs))
	for _, id := range scenarioIDs {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 64 {
			writeError(w, http.StatusBadRequest, "invalid scenario id")
			return
		}
		if _, exists := seenScenarioIDs[id]; exists {
			continue
		}
		seenScenarioIDs[id] = struct{}{}
		uniqueScenarioIDs = append(uniqueScenarioIDs, id)
	}
	scenarioIDs = uniqueScenarioIDs

	uploadID, taskID := newID(), newID()
	queryCode := requestedQueryCode
	uploadTime := time.Now()
	var uploadRoot string
	if uploadSessionID != "" {
		if requestedQueryCode == "" {
			writeError(w, http.StatusBadRequest, "query_code is required with upload_session_id")
			return
		}
		session, sessionErr := s.repo.GetUploadSessionForUpload(r.Context(), uploadSessionID, requestedQueryCode, creatorOpenID)
		if errors.Is(sessionErr, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "upload session or query_code is invalid")
			return
		}
		if sessionErr != nil {
			writeError(w, http.StatusInternalServerError, "validate upload session failed")
			return
		}
		if session.ProjectName != metadata.ProjectName || session.Version != metadata.Version ||
			session.TestTaskID != metadata.TestTaskID || session.TestTaskName != metadata.TestTaskName ||
			session.UploaderName != metadata.UploaderName {
			writeError(w, http.StatusBadRequest, "upload metadata does not match upload session")
			return
		}
		if !sameJSON(session.ConfigSnapshot, []byte(r.FormValue("config_snapshot"))) {
			writeError(w, http.StatusBadRequest, "config_snapshot does not match upload session")
			return
		}
		queryCode = session.QueryCode
		uploadRoot = filepath.Join(session.StorageRoot, uploadID)
	} else {
		if requestedQueryCode != "" {
			writeError(w, http.StatusBadRequest, "upload_session_id is required with query_code")
			return
		}
		queryCode = newQueryCode(projectName)
		userSegment, userErr := s.repo.UserStorageName(r.Context(), creatorOpenID)
		if userErr != nil {
			if !errors.Is(userErr, sql.ErrNoRows) {
				writeError(w, http.StatusInternalServerError, "resolve upload user failed")
				return
			}
			userSegment = creatorOpenID
		}
		uploadRoot = filepath.Join(
			s.config.StorageDir,
			safeStorageSegment(projectName, "unknown-project"),
			safeStorageSegment(userSegment, "unknown-user"),
			uploadTime.Format("2006-01-02"),
			uploadID,
		)
	}
	if err := os.MkdirAll(uploadRoot, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "create upload storage failed")
		return
	}
	metadata.QueryCode = queryCode
	metadata.UploadSessionID = uploadSessionID
	if err := s.repo.CreateUploadWithMetadata(r.Context(), uploadID, taskID, metadata, scenarioIDs, uploadRoot, creatorOpenID); err != nil {
		os.RemoveAll(uploadRoot)
		s.repo.RecordRuntimeLog(r.Context(), creatorOpenID, "upload", "create upload task", "failed", err.Error(), taskID, queryCode)
		if errors.Is(err, ErrProjectNotFound) {
			writeError(w, http.StatusBadRequest, "project does not exist")
			return
		}
		if errors.Is(err, ErrScenarioNotApplicable) {
			writeError(w, http.StatusBadRequest, "test scenario is unavailable or does not apply to this project")
			return
		}
		if errors.Is(err, ErrScenarioRuleUnavailable) {
			writeError(w, http.StatusBadRequest, "test scenario contains an unavailable parsing rule")
			return
		}
		log.Printf("create upload record failed: %v", err)
		writeError(w, http.StatusInternalServerError, "create upload record failed")
		return
	}

	var totalSize int64
	var originalNames []string
	var storedItems []storedUploadItem
	usedNames := make(map[string]struct{}, len(headers))
	for index, header := range headers {
		name := uniqueFileName(filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/")), func(candidate string) bool {
			key := strings.ToLower(candidate)
			if _, exists := usedNames[key]; exists {
				return true
			}
			usedNames[key] = struct{}{}
			return false
		}, uploadTime)
		itemRoot := filepath.Join(uploadRoot, "items", strconv.Itoa(index+1))
		storedPath, size, err := saveUploadedFile(header, name, itemRoot, capacity.MaxUploadBytes-totalSize)
		if err != nil {
			s.failUpload(w, uploadID, uploadRoot, err)
			return
		}
		totalSize += size
		originalNames = append(originalNames, name)
		storedItems = append(storedItems, storedUploadItem{index: index + 1, storedPath: storedPath, itemRoot: itemRoot})
	}
	originalName := strings.Join(originalNames, ", ")
	if err := s.repo.StoreUploadMetadata(r.Context(), uploadID, originalName, totalSize); err != nil {
		os.RemoveAll(uploadRoot)
		s.repo.MarkFailed(context.Background(), uploadID, "queue upload failed")
		writeError(w, http.StatusInternalServerError, "save upload metadata failed")
		return
	}
	go s.prepareUpload(uploadID, originalName, totalSize, storedItems)
	response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "upload accepted", Data: map[string]any{
		"upload_id": uploadID, "task_id": taskID, "query_code": queryCode, "status": "queued", "file_count": len(headers), "scenario_ids": scenarioIDs,
		"client_request_id": metadata.ClientRequestID,
	}})
	s.repo.RecordRuntimeLog(r.Context(), creatorOpenID, "upload", "upload accepted", "success", "upload queued for parsing", taskID, queryCode)
}

func sameJSON(left, right []byte) bool {
	if len(strings.TrimSpace(string(left))) == 0 || len(strings.TrimSpace(string(right))) == 0 {
		return false
	}
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func uploadMetadataFromForm(r *http.Request, creatorOpenID, projectName, version string) (UploadMetadata, error) {
	metadata := UploadMetadata{
		ProjectID:           strings.TrimSpace(r.FormValue("project_id")),
		ProjectName:         projectName,
		Version:             version,
		TestTaskID:          strings.TrimSpace(r.FormValue("test_task_id")),
		TestTaskName:        strings.TrimSpace(r.FormValue("test_task_name")),
		UploaderName:        strings.TrimSpace(r.FormValue("uploader_name")),
		UploaderID:          creatorOpenID,
		Remark:              strings.TrimSpace(r.FormValue("remark")),
		ClientRequestID:     strings.TrimSpace(firstNonEmptyValue(r.Header.Get("Idempotency-Key"), r.FormValue("client_request_id"))),
		CollectorVersion:    strings.TrimSpace(r.FormValue("collector_version")),
		Timezone:            strings.TrimSpace(r.FormValue("timezone")),
		DisableParsingRules: true,
	}
	if metadata.UploaderName == "" {
		return UploadMetadata{}, errors.New("uploader_name is required")
	}
	if value := strings.TrimSpace(r.FormValue("disable_parsing_rules")); value != "" {
		disableParsingRules, err := strconv.ParseBool(value)
		if err != nil {
			return UploadMetadata{}, fmt.Errorf("disable_parsing_rules must be a boolean")
		}
		metadata.DisableParsingRules = disableParsingRules
	}
	lengths := []struct {
		name  string
		value string
		max   int
	}{
		{"project_id", metadata.ProjectID, 128},
		{"test_task_id", metadata.TestTaskID, 128},
		{"test_task_name", metadata.TestTaskName, 256},
		{"uploader_name", metadata.UploaderName, 128},
		{"remark", metadata.Remark, 4000},
		{"client_request_id", metadata.ClientRequestID, 128},
		{"collector_version", metadata.CollectorVersion, 64},
		{"timezone", metadata.Timezone, 64},
	}
	for _, field := range lengths {
		if len([]rune(field.value)) > field.max {
			return UploadMetadata{}, fmt.Errorf("%s is too long", field.name)
		}
	}
	for _, field := range []struct {
		name   string
		value  string
		target **time.Time
	}{
		{"created_at", r.FormValue("created_at"), &metadata.CreatedAt},
		{"started_at", r.FormValue("started_at"), &metadata.StartedAt},
		{"ended_at", r.FormValue("ended_at"), &metadata.EndedAt},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return UploadMetadata{}, fmt.Errorf("%s must be RFC 3339", field.name)
		}
		*field.target = &parsed
	}
	return metadata, nil
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safeStorageSegment(value, fallback string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '_'
		}
		return r
	}, value)
	value = strings.Trim(value, ". ")
	if value == "" {
		value = fallback
	}
	upper := strings.ToUpper(value)
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" ||
		(len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9') {
		value += "_"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	return value
}

func (s *Service) prepareUpload(uploadID, originalName string, totalSize int64, items []storedUploadItem) {
	ctx := context.Background()
	done := make(chan struct{})
	go s.keepTaskAlive(uploadID, done)
	defer close(done)
	var logFiles []LogFile
	var extractedSize int64
	for _, item := range items {
		files, err := collectLogFiles(item.storedPath, item.itemRoot, s.config.MaxExtractBytes-extractedSize)
		if err != nil {
			s.repo.MarkFailed(ctx, uploadID, err.Error())
			s.repo.RecordUploadRuntimeLog(ctx, uploadID, "archive", "extract uploaded archive", "failed", err.Error())
			return
		}
		for i := range files {
			path := filepath.ToSlash(files[i].RelativePath)
			if strings.Contains(path, "/extracted/") || strings.HasPrefix(path, "extracted/") {
				extractedSize += files[i].SizeBytes
			}
			files[i].RelativePath = filepath.ToSlash(filepath.Join("items", strconv.Itoa(item.index), filepath.FromSlash(files[i].RelativePath)))
		}
		logFiles = append(logFiles, files...)
	}
	if err := s.repo.QueueUpload(ctx, uploadID, originalName, totalSize, logFiles); err != nil {
		s.repo.MarkFailed(ctx, uploadID, fmt.Sprintf("queue upload: %v", err))
		s.repo.RecordUploadRuntimeLog(ctx, uploadID, "upload", "queue upload", "failed", err.Error())
		return
	}
	s.repo.RecordUploadRuntimeLog(ctx, uploadID, "archive", "prepare uploaded files", "success", fmt.Sprintf("prepared %d log files", len(logFiles)))
	s.processUpload(uploadID)
}

func (s *Service) failUpload(w http.ResponseWriter, uploadID, uploadRoot string, err error) {
	os.RemoveAll(uploadRoot)
	s.repo.MarkFailed(context.Background(), uploadID, err.Error())
	s.repo.RecordUploadRuntimeLog(context.Background(), uploadID, "upload", "save uploaded file", "failed", err.Error())
	writeError(w, http.StatusBadRequest, err.Error())
}

func saveUploadedFile(header *multipart.FileHeader, name, itemRoot string, remaining int64) (string, int64, error) {
	if remaining <= 0 {
		return "", 0, fmt.Errorf("upload size exceeded")
	}
	if name == "." || name == "" {
		return "", 0, fmt.Errorf("invalid file name")
	}
	if !supportedUpload(name) {
		return "", 0, fmt.Errorf("unsupported file type: %s", name)
	}
	input, err := header.Open()
	if err != nil {
		return "", 0, err
	}
	defer input.Close()
	directory := filepath.Join(itemRoot, "original")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", 0, err
	}
	path := filepath.Join(directory, name)
	output, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, err
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, remaining+1))
	closeErr := output.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	if written > remaining {
		os.Remove(path)
		return "", 0, fmt.Errorf("upload size exceeded")
	}
	return path, written, nil
}

func supportedUpload(name string) bool {
	lower := strings.ToLower(name)
	return isLogFile(lower) || strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz")
}

func (s *Service) listUploadsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	page, pageSize := pagination(r)
	sourceType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source_type")))
	if sourceType != "" && sourceType != "collector" && sourceType != "uploaded" {
		writeError(w, http.StatusBadRequest, "invalid source_type")
		return
	}
	items, total, err := s.repo.ListUploads(r.Context(), ownerOpenID, sourceType, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query uploads failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"total": total, "list": items}})
}

func (s *Service) logDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/logs/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "preview" {
		s.logPreviewHandler(w, r, parts[0], ownerOpenID)
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	id := parts[0]
	upload, files, err := s.repo.GetUpload(r.Context(), id, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"upload": upload, "files": files}})
}

type logFilePreview struct {
	FileID       int64    `json:"file_id"`
	RelativePath string   `json:"relative_path"`
	Lines        []string `json:"lines"`
	Truncated    bool     `json:"truncated"`
}

func (s *Service) logPreviewHandler(w http.ResponseWriter, r *http.Request, uploadID, ownerOpenID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	_, files, err := s.repo.GetUpload(r.Context(), uploadID, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	if len(files) == 0 {
		writeError(w, http.StatusNotFound, "log file not found")
		return
	}
	selected := files[0]
	if rawID := strings.TrimSpace(r.URL.Query().Get("file_id")); rawID != "" {
		fileID, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid file_id")
			return
		}
		found := false
		for _, file := range files {
			if file.ID == fileID {
				selected, found = file, true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "log file not found")
			return
		}
	}
	storageRoot, err := s.repo.UploadStoragePath(r.Context(), uploadID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	cleanRelative := filepath.Clean(filepath.FromSlash(selected.RelativePath))
	if filepath.IsAbs(cleanRelative) || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		writeError(w, http.StatusInternalServerError, "invalid stored log path")
		return
	}
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve upload storage failed")
		return
	}
	fullPath, err := filepath.Abs(filepath.Join(root, cleanRelative))
	if err != nil || (fullPath != root && !strings.HasPrefix(fullPath, root+string(filepath.Separator))) {
		writeError(w, http.StatusInternalServerError, "invalid stored log path")
		return
	}
	input, err := os.Open(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "stored log file unavailable")
		return
	}
	defer input.Close()

	const maxPreviewBytes = 2 * 1024 * 1024
	const maxPreviewLines = 500
	scanner := bufio.NewScanner(io.LimitReader(input, maxPreviewBytes+1))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make([]string, 0, maxPreviewLines)
	truncated := false
	for scanner.Scan() {
		if len(lines) == maxPreviewLines {
			truncated = true
			break
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "read stored log failed")
		return
	}
	if !truncated {
		if info, statErr := input.Stat(); statErr == nil && info.Size() > maxPreviewBytes {
			truncated = true
		}
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: logFilePreview{
		FileID: selected.ID, RelativePath: selected.RelativePath, Lines: lines, Truncated: truncated,
	}})
}

func (s *Service) listTasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	page, pageSize := pagination(r)
	items, total, err := s.repo.ListTasks(r.Context(), ownerOpenID, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query tasks failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"total": total, "list": items}})
}

func (s *Service) taskHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	trimmed := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 3 && parts[0] == "api" {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	taskID := parts[1]
	if r.Method == http.MethodDelete && len(parts) == 2 {
		storagePath, err := s.repo.DeleteTask(r.Context(), taskID, ownerOpenID)
		if err != nil {
			handleQueryError(w, err)
			return
		}
		if err := removeUploadStorage(s.config.StorageDir, storagePath); err != nil {
			writeError(w, http.StatusInternalServerError, "task deleted but storage cleanup failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if len(parts) == 3 && parts[2] == "results" {
		page, pageSize := pagination(r)
		results, err := s.repo.Results(r.Context(), taskID, ownerOpenID, pageSize, (page-1)*pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query task results failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: results})
		return
	}
	if len(parts) == 3 && parts[2] == "agent-results" {
		results, err := s.repo.AgentResults(r.Context(), taskID, ownerOpenID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query agent results failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: results})
		return
	}
	upload, files, err := s.repo.GetUploadByTask(r.Context(), taskID, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{
		"task": upload, "files": files, "agent_enabled": s.agent != nil,
	}})
}

func removeUploadStorage(storageRoot, target string) error {
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		return err
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, absoluteTarget)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe storage path")
	}
	return os.RemoveAll(absoluteTarget)
}

func pagination(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func lastPathPart(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return parts[len(parts)-1]
}
func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
func handleQueryError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "not found")
	} else {
		writeError(w, http.StatusInternalServerError, "query failed")
	}
}
func writeError(w http.ResponseWriter, status int, message string) {
	response.JSONStatus(w, status, response.APIResponse{Code: status, Message: message, Data: nil})
}

func (s *Service) requireCurrentUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.currentUserResolver == nil {
		writeError(w, http.StatusUnauthorized, "login required")
		return "", false
	}
	openID, ok := s.currentUserResolver(r)
	if !ok || strings.TrimSpace(openID) == "" {
		writeError(w, http.StatusUnauthorized, "login required")
		return "", false
	}
	return openID, true
}

func (s *Service) keywordSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.uploadUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	keywords, err := s.repo.ListStandardKeywords(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query standard keywords failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: keywords})
}

func (s *Service) queryHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/query/"), "/")
	collect := strings.HasSuffix(path, "/collect")
	if collect {
		path = strings.TrimSuffix(path, "/collect")
	}
	queryCode := strings.ToUpper(strings.TrimSpace(path))
	if queryCode == "" || strings.Contains(queryCode, "/") {
		writeError(w, http.StatusBadRequest, "invalid query code")
		return
	}
	if collect {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		ownerOpenID, ok := s.requireCurrentUser(w, r)
		if !ok {
			return
		}
		batchCount, err := s.repo.LinkCollectedUploadSession(r.Context(), ownerOpenID, queryCode)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "query code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "collect upload session failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "collected upload session linked", Data: map[string]any{
			"query_code": queryCode, "batch_count": batchCount, "source_type": "collector",
		}})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	result, err := s.repo.GetPublicUploadByQueryCode(r.Context(), queryCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "query code not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query upload failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: result})
}

func (s *Service) uploadUser(r *http.Request) (string, bool) {
	if s.currentUserResolver != nil {
		if openID, ok := s.currentUserResolver(r); ok && strings.TrimSpace(openID) != "" {
			return strings.TrimSpace(openID), true
		}
	}
	credentials := strings.TrimSpace(r.Header.Get("Authorization"))
	if subtle.ConstantTimeCompare([]byte(credentials), []byte("Bearer "+builtinUploadToken)) == 1 {
		return builtinUploadOwnerOpenID, true
	}
	if s.uploadToken != "" && s.uploadOwnerOpenID != "" &&
		subtle.ConstantTimeCompare([]byte(credentials), []byte("Bearer "+s.uploadToken)) == 1 {
		return s.uploadOwnerOpenID, true
	}
	return "", false
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes)
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
}

func newQueryCode(projectName string) string {
	bytes := make([]byte, 5)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	suffix := strings.ToUpper(hex.EncodeToString(bytes))
	prefix := queryCodePrefix(projectName)
	if prefix == "" {
		return suffix
	}
	return prefix + "-" + suffix
}

// queryCodePrefix 将项目名规范化为上传码前缀：仅保留字母数字，截断到 16 字符。
func queryCodePrefix(projectName string) string {
	var builder strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(projectName)) {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
		if builder.Len() >= 16 {
			break
		}
	}
	return builder.String()
}
