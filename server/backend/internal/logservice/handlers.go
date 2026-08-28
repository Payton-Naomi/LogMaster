package logservice

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
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
	if uploadSessionID == "" && metadata.UploaderName == "" {
		writeError(w, http.StatusBadRequest, "uploader_name is required")
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
		if metadata.UploaderName != "" && session.UploaderName != metadata.UploaderName {
			writeError(w, http.StatusBadRequest, "upload metadata does not match upload session")
			return
		}
		if session.ProjectName != metadata.ProjectName || session.Version != metadata.Version || session.TestTaskID != metadata.TestTaskID || session.TestTaskName != metadata.TestTaskName {
			writeError(w, http.StatusBadRequest, "upload metadata does not match upload session")
			return
		}
		metadata.UploaderName, metadata.UploaderID = session.UploaderName, session.UploaderID
		metadata.AIAnalysisEnabled = session.AIAnalysisEnabled
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
			writeError(w, http.StatusBadRequest, "test task is not synchronized to a server test scenario")
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
		_, size, err := saveUploadedFile(header, name, itemRoot, capacity.MaxUploadBytes-totalSize)
		if err != nil {
			s.failUpload(w, uploadID, uploadRoot, err)
			return
		}
		totalSize += size
		originalNames = append(originalNames, name)
	}
	originalName := strings.Join(originalNames, ", ")
	if err := s.repo.StoreUploadMetadata(r.Context(), uploadID, originalName, totalSize); err != nil {
		os.RemoveAll(uploadRoot)
		s.repo.MarkFailed(context.Background(), uploadID, "queue upload failed")
		writeError(w, http.StatusInternalServerError, "save upload metadata failed")
		return
	}
	s.signalParseQueue()
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
		AIAnalysisEnabled:   true,
	}
	if value := strings.TrimSpace(r.FormValue("ai_analysis_enabled")); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return UploadMetadata{}, fmt.Errorf("ai_analysis_enabled must be a boolean")
		}
		metadata.AIAnalysisEnabled = enabled
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
	filters := UploadFilters{
		Keyword:     strings.TrimSpace(r.URL.Query().Get("keyword")),
		Project:     strings.TrimSpace(r.URL.Query().Get("project")),
		StatusGroup: strings.TrimSpace(r.URL.Query().Get("status_group")),
		Sort:        strings.TrimSpace(r.URL.Query().Get("sort")),
	}
	if raw := r.URL.Query().Get("start"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filters.Start = &t
		}
	}
	if raw := r.URL.Query().Get("end"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			filters.End = &t
		}
	}
	items, total, err := s.repo.ListUploads(r.Context(), ownerOpenID, sourceType, filters, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query uploads failed")
		return
	}
	summary, err := s.repo.UploadSummary(r.Context(), ownerOpenID, sourceType, filters)
	if err != nil {
		summary = UploadSummary{}
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{
		"total": total, "page": page, "page_size": pageSize, "list": items, "summary": summary,
	}})
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
	if len(parts) == 2 && parts[1] == "search" {
		s.logSearchHandler(w, r, parts[0], ownerOpenID)
		return
	}
	if len(parts) == 2 && parts[1] == "download" {
		s.logDownloadHandler(w, r, parts[0], ownerOpenID)
		return
	}
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

func (s *Service) resultHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "results" && parts[2] == "batch" && parts[3] == "assignment" && r.Method == http.MethodPut {
		var input struct {
			ResultIDs  []int64 `json:"result_ids"`
			AssignedTo string  `json:"assigned_to"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input) != nil || len(input.ResultIDs) == 0 || len(input.ResultIDs) > 500 || len([]rune(input.AssignedTo)) > 128 {
			writeError(w, http.StatusBadRequest, "批量负责人请求无效，结果数量必须为 1 到 500")
			return
		}
		items := make([]map[string]any, 0, len(input.ResultIDs))
		succeeded := 0
		for _, id := range input.ResultIDs {
			item := map[string]any{"result_id": id, "success": false}
			_, err := s.repo.UpdateResultAssignment(r.Context(), id, ownerOpenID, strings.TrimSpace(input.AssignedTo))
			switch {
			case err == nil:
				item["success"] = true
				item["message"] = "负责人已更新"
				succeeded++
				if notifyErr := s.repo.CreateResultAssignmentNotification(r.Context(), id, ownerOpenID); notifyErr != nil {
					log.Printf("create assignment notification for result %d: %v", id, notifyErr)
				}
			case errors.Is(err, ErrAssignedUserNotFound):
				item["message"] = "负责人用户不存在"
			case errors.Is(err, sql.ErrNoRows):
				item["message"] = "异常结果不存在或无权访问"
			default:
				item["message"] = "更新负责人失败"
			}
			items = append(items, item)
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "批量负责人操作完成", Data: map[string]any{"total": len(items), "succeeded": succeeded, "failed": len(items) - succeeded, "items": items}})
		return
	}
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "results" || parts[3] != "status" {
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "results" && parts[3] == "history" && r.Method == http.MethodGet {
			resultID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || resultID <= 0 {
				writeError(w, http.StatusBadRequest, "结果编号无效")
				return
			}
			items, historyErr := s.repo.ResultHistory(r.Context(), resultID, ownerOpenID)
			if errors.Is(historyErr, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "异常结果不存在或无权访问")
				return
			}
			if historyErr != nil {
				writeError(w, http.StatusInternalServerError, "查询异常操作历史失败")
				return
			}
			response.JSON(w, response.APIResponse{Code: 0, Message: "查询成功", Data: items})
			return
		}
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "results" && parts[3] == "assignment" && r.Method == http.MethodPut {
			resultID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || resultID <= 0 {
				writeError(w, http.StatusBadRequest, "结果编号无效")
				return
			}
			var input struct {
				AssignedTo string `json:"assigned_to"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || len([]rune(input.AssignedTo)) > 128 {
				writeError(w, http.StatusBadRequest, "负责人编号无效")
				return
			}
			result, assignmentErr := s.repo.UpdateResultAssignment(r.Context(), resultID, ownerOpenID, strings.TrimSpace(input.AssignedTo))
			if assignmentErr != nil {
				if errors.Is(assignmentErr, ErrAssignedUserNotFound) {
					writeError(w, http.StatusBadRequest, "负责人用户不存在")
				} else if errors.Is(assignmentErr, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "异常结果不存在或无权访问")
				} else {
					writeError(w, http.StatusInternalServerError, "更新结果负责人失败")
				}
				return
			}
			if notifyErr := s.repo.CreateResultAssignmentNotification(r.Context(), resultID, ownerOpenID); notifyErr != nil {
				log.Printf("create assignment notification for result %d: %v", resultID, notifyErr)
			}
			response.JSON(w, response.APIResponse{Code: 0, Message: "结果负责人已更新", Data: result})
			return
		}
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "results" && parts[3] == "comments" && r.Method == http.MethodPost {
			resultID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || resultID <= 0 {
				writeError(w, http.StatusBadRequest, "结果编号无效")
				return
			}
			var input struct {
				Comment  string `json:"comment"`
				DefectID string `json:"defect_id"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil || strings.TrimSpace(input.Comment) == "" {
				writeError(w, http.StatusBadRequest, "备注内容不能为空")
				return
			}
			if len([]rune(input.Comment)) > 5000 || len([]rune(input.DefectID)) > 128 {
				writeError(w, http.StatusBadRequest, "备注或缺陷单号过长")
				return
			}
			item, commentErr := s.repo.AddResultComment(r.Context(), resultID, ownerOpenID, strings.TrimSpace(input.Comment), strings.TrimSpace(input.DefectID))
			if commentErr != nil {
				if errors.Is(commentErr, sql.ErrNoRows) {
					writeError(w, http.StatusNotFound, "异常结果不存在或无权访问")
				} else {
					log.Printf("save result comment %d: %v", resultID, commentErr)
					writeError(w, http.StatusInternalServerError, "保存结果备注失败")
				}
				return
			}
			if notifyErr := s.repo.CreateResultCommentNotifications(r.Context(), resultID, ownerOpenID); notifyErr != nil {
				log.Printf("create comment notification for result %d: %v", resultID, notifyErr)
			}
			response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "结果备注已添加", Data: item})
			return
		}
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "results" && parts[3] == "comments" && r.Method == http.MethodGet {
			resultID, parseErr := strconv.ParseInt(parts[2], 10, 64)
			if parseErr != nil || resultID <= 0 {
				writeError(w, http.StatusBadRequest, "结果编号无效")
				return
			}
			items, commentErr := s.repo.ResultComments(r.Context(), resultID, ownerOpenID)
			if commentErr != nil {
				writeError(w, http.StatusInternalServerError, "查询结果备注失败")
				return
			}
			response.JSON(w, response.APIResponse{Code: 0, Message: "查询成功", Data: items})
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPatch {
		methodNotAllowed(w)
		return
	}
	resultID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || resultID <= 0 {
		writeError(w, http.StatusBadRequest, "结果编号无效")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || !validResultStatuses[input.Status] {
		writeError(w, http.StatusBadRequest, "结果状态无效")
		return
	}
	result, err := s.repo.UpdateResultStatus(r.Context(), resultID, ownerOpenID, input.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "异常结果不存在或无权访问")
			return
		}
		writeError(w, http.StatusInternalServerError, "更新结果状态失败")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "结果状态已更新", Data: result})
}

func (s *Service) compareHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	baseline := strings.TrimSpace(r.URL.Query().Get("baseline_task_id"))
	current := strings.TrimSpace(r.URL.Query().Get("current_task_id"))
	if baseline == "" || current == "" || baseline == current {
		writeError(w, http.StatusBadRequest, "基准任务和当前任务必须有效且不能相同")
		return
	}
	if _, _, err := s.repo.GetUploadByTask(r.Context(), baseline, ownerOpenID); err != nil {
		handleQueryError(w, err)
		return
	}
	if _, _, err := s.repo.GetUploadByTask(r.Context(), current, ownerOpenID); err != nil {
		handleQueryError(w, err)
		return
	}
	comparison, err := s.repo.CompareTasks(r.Context(), baseline, current, ownerOpenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询分析对比失败")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "分析对比完成", Data: comparison})
}

func (s *Service) notificationsHandler(w http.ResponseWriter, r *http.Request) {
	recipient, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/notifications"), "/")
	if r.Method == http.MethodGet && path == "stream" {
		s.notificationStreamHandler(w, r, recipient)
		return
	}
	if r.Method == http.MethodGet && path == "" {
		page, pageSize := pagination(r)
		unreadOnly := false
		if raw := strings.TrimSpace(r.URL.Query().Get("unread_only")); raw != "" {
			var err error
			unreadOnly, err = strconv.ParseBool(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "unread_only 参数无效")
				return
			}
		}
		items, total, unread, err := s.repo.ListNotifications(r.Context(), recipient, unreadOnly, pageSize, (page-1)*pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询通知失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "查询成功", Data: map[string]any{"total": total, "unread": unread, "page": page, "page_size": pageSize, "list": items}})
		return
	}
	if r.Method == http.MethodPost && path == "read-all" {
		count, err := s.repo.MarkAllNotificationsRead(r.Context(), recipient)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "全部标记已读失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "通知已全部标记为已读", Data: map[string]any{"updated": count}})
		return
	}
	parts := strings.Split(path, "/")
	if r.Method == http.MethodPatch && len(parts) == 2 && parts[1] == "read" {
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "通知编号无效")
			return
		}
		item, err := s.repo.MarkNotificationRead(r.Context(), id, recipient)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "通知不存在")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "更新通知失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "通知已读", Data: item})
		return
	}
	methodNotAllowed(w)
}

func (s *Service) notificationSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		settings, err := s.repo.GetNotificationSettings(r.Context(), userOpenID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询通知设置失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "查询成功", Data: settings})
		return
	}
	if r.Method == http.MethodPut {
		var input struct {
			TaskCompleted   *bool `json:"task_completed"`
			TaskFailed      *bool `json:"task_failed"`
			TaskCancelled   *bool `json:"task_cancelled"`
			AICompleted     *bool `json:"ai_completed"`
			AIFailed        *bool `json:"ai_failed"`
			ResultAssigned  *bool `json:"result_assigned"`
			ResultCommented *bool `json:"result_commented"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || input.TaskCompleted == nil || input.TaskFailed == nil ||
			input.TaskCancelled == nil || input.AICompleted == nil || input.AIFailed == nil ||
			input.ResultAssigned == nil || input.ResultCommented == nil {
			writeError(w, http.StatusBadRequest, "通知设置格式无效")
			return
		}
		settings := NotificationSettings{
			TaskCompleted: *input.TaskCompleted, TaskFailed: *input.TaskFailed, TaskCancelled: *input.TaskCancelled,
			AICompleted: *input.AICompleted, AIFailed: *input.AIFailed,
			ResultAssigned: *input.ResultAssigned, ResultCommented: *input.ResultCommented,
		}
		saved, err := s.repo.SaveNotificationSettings(r.Context(), userOpenID, settings)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "保存通知设置失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "通知设置已保存", Data: saved})
		return
	}
	methodNotAllowed(w)
}

func (s *Service) notificationStreamHandler(w http.ResponseWriter, r *http.Request, recipient string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "当前服务器不支持通知流")
		return
	}
	afterID := int64(0)
	rawAfter := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if rawAfter == "" {
		rawAfter = strings.TrimSpace(r.URL.Query().Get("after_id"))
	}
	if rawAfter != "" {
		parsed, err := strconv.ParseInt(rawAfter, 10, 64)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "通知游标无效")
			return
		}
		afterID = parsed
	} else {
		latest, err := s.repo.LatestNotificationID(r.Context(), recipient)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "初始化通知流失败")
			return
		}
		afterID = latest
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	poll := time.NewTicker(2 * time.Second)
	heartbeat := time.NewTicker(15 * time.Second)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-poll.C:
			items, err := s.repo.NotificationsAfter(r.Context(), recipient, afterID, 100)
			if err != nil {
				fmt.Fprint(w, "event: error\ndata: {\"message\":\"query failed\"}\n\n")
				flusher.Flush()
				continue
			}
			for _, item := range items {
				payload, err := json.Marshal(item)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "id: %d\nevent: notification\ndata: %s\n\n", item.ID, payload)
				afterID = item.ID
			}
			if len(items) > 0 {
				flusher.Flush()
			}
		}
	}
}

type logFilePreview struct {
	FileID       int64    `json:"file_id"`
	RelativePath string   `json:"relative_path"`
	Lines        []string `json:"lines"`
	Truncated    bool     `json:"truncated"`
}

type logSearchMatch struct {
	FileID       int64  `json:"file_id"`
	RelativePath string `json:"relative_path"`
	LineNumber   int    `json:"line_number"`
	Content      string `json:"content"`
}

func (s *Service) logSearchHandler(w http.ResponseWriter, r *http.Request, uploadID, ownerOpenID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	_, files, err := s.repo.GetUpload(r.Context(), uploadID, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	keyword := r.URL.Query().Get("keyword")
	if strings.TrimSpace(keyword) == "" {
		writeError(w, http.StatusBadRequest, "keyword is required")
		return
	}
	if len(keyword) > 1000 {
		writeError(w, http.StatusBadRequest, "keyword is too long")
		return
	}
	caseSensitive := false
	if raw := strings.TrimSpace(r.URL.Query().Get("case_sensitive")); raw != "" {
		caseSensitive, err = strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid case_sensitive")
			return
		}
	}
	page, pageSize, err := parsePageParams(r, 100, 500)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	selectedFiles := files
	if rawID := strings.TrimSpace(r.URL.Query().Get("file_id")); rawID != "" {
		fileID, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid file_id")
			return
		}
		selectedFiles = nil
		for _, file := range files {
			if file.ID == fileID {
				selectedFiles = []LogFile{file}
				break
			}
		}
		if len(selectedFiles) == 0 {
			writeError(w, http.StatusNotFound, "log file not found")
			return
		}
	}
	storageRoot, err := s.repo.UploadStoragePath(r.Context(), uploadID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve upload storage failed")
		return
	}
	needle := keyword
	if !caseSensitive {
		needle = strings.ToLower(keyword)
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	cacheFiles := make([]string, 0, len(selectedFiles))
	for _, file := range selectedFiles {
		cacheFiles = append(cacheFiles, fmt.Sprintf("%d:%s", file.ID, file.SHA256))
	}
	cacheKey := logSearchCacheKey{UploadID: uploadID, Files: strings.Join(cacheFiles, ","), Keyword: keyword, CaseSensitive: caseSensitive}
	loadMatches := func() ([]logSearchMatch, error) {
		allMatches := make([]logSearchMatch, 0)
		for _, file := range selectedFiles {
			cleanRelative := filepath.Clean(filepath.FromSlash(file.RelativePath))
			if filepath.IsAbs(cleanRelative) || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("invalid stored log path")
			}
			fullPath, pathErr := filepath.Abs(filepath.Join(root, cleanRelative))
			if pathErr != nil || (fullPath != root && !strings.HasPrefix(fullPath, root+string(filepath.Separator))) {
				return nil, fmt.Errorf("invalid stored log path")
			}
			input, openErr := os.Open(fullPath)
			if openErr != nil {
				return nil, fmt.Errorf("stored log file unavailable")
			}
			scanner := bufio.NewScanner(input)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			lineNumber := 0
			for scanner.Scan() {
				lineNumber++
				content := scanner.Text()
				candidate := content
				if !caseSensitive {
					candidate = strings.ToLower(content)
				}
				if !strings.Contains(candidate, needle) {
					continue
				}
				allMatches = append(allMatches, logSearchMatch{FileID: file.ID, RelativePath: file.RelativePath, LineNumber: lineNumber, Content: content})
			}
			closeErr := input.Close()
			if scanErr := scanner.Err(); scanErr != nil {
				return nil, fmt.Errorf("read stored log failed")
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close stored log failed")
			}
		}
		return allMatches, nil
	}
	var allMatches []logSearchMatch
	if s.searchCache != nil {
		allMatches, err = s.searchCache.getOrLoad(cacheKey, loadMatches)
	} else {
		allMatches, err = loadMatches()
	}
	if err != nil {
		if strings.Contains(err.Error(), "stored log file unavailable") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	total := len(allMatches)
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	matches := allMatches[start:end]
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{
		"total": total, "page": page, "page_size": pageSize, "keyword": keyword, "case_sensitive": caseSensitive, "matches": matches,
	}})
}

func (s *Service) logDownloadHandler(w http.ResponseWriter, r *http.Request, uploadID, ownerOpenID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	upload, files, err := s.repo.GetUpload(r.Context(), uploadID, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	rawFileID := strings.TrimSpace(r.URL.Query().Get("file_id"))
	if kind == "" {
		if rawFileID != "" {
			kind = "file"
		} else {
			kind = "batch"
		}
	}
	if kind != "file" && kind != "batch" && kind != "original" && kind != "results" {
		writeError(w, http.StatusBadRequest, "下载类型必须为 file、batch、original 或 results")
		return
	}
	if kind == "results" {
		artifacts, buildErr := s.buildTaskResultArtifacts(r.Context(), upload.TaskID, ownerOpenID)
		if buildErr != nil {
			writeError(w, http.StatusInternalServerError, "生成分析结果包失败")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="results-`+uploadID+`.zip"`)
		archive := zip.NewWriter(w)
		for _, artifact := range artifacts {
			entry, createErr := archive.Create(artifact.Name)
			if createErr != nil {
				_ = archive.Close()
				return
			}
			if _, writeErr := entry.Write(artifact.Data); writeErr != nil {
				_ = archive.Close()
				return
			}
		}
		_ = archive.Close()
		return
	}
	storageRoot, err := s.repo.UploadStoragePath(r.Context(), uploadID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "解析日志存储路径失败")
		return
	}
	if kind == "original" {
		items, itemErr := storedUploadItems(root)
		if itemErr != nil {
			writeError(w, http.StatusNotFound, "原始上传文件不存在")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="original-`+uploadID+`.zip"`)
		archive := zip.NewWriter(w)
		for _, item := range items {
			entry, createErr := archive.Create(fmt.Sprintf("items/%d/%s", item.index, filepath.Base(item.storedPath)))
			if createErr != nil {
				_ = archive.Close()
				return
			}
			input, openErr := os.Open(item.storedPath)
			if openErr != nil {
				_ = archive.Close()
				return
			}
			_, copyErr := io.Copy(entry, input)
			closeErr := input.Close()
			if copyErr != nil || closeErr != nil {
				_ = archive.Close()
				return
			}
		}
		_ = archive.Close()
		return
	}
	if len(files) == 0 {
		writeError(w, http.StatusNotFound, "日志文件不存在")
		return
	}
	selected := files
	if kind == "file" {
		if rawFileID == "" {
			writeError(w, http.StatusBadRequest, "下载单文件必须提供 file_id")
			return
		}
		fileID, parseErr := strconv.ParseInt(rawFileID, 10, 64)
		if parseErr != nil || fileID <= 0 {
			writeError(w, http.StatusBadRequest, "file_id 无效")
			return
		}
		selected = nil
		for _, file := range files {
			if file.ID == fileID {
				selected = []LogFile{file}
				break
			}
		}
		if len(selected) == 0 {
			writeError(w, http.StatusNotFound, "日志文件不存在")
			return
		}
	}
	paths := make([]string, 0, len(selected))
	for _, file := range selected {
		fullPath, pathErr := safeUploadFilePath(root, file.RelativePath)
		if pathErr != nil {
			writeError(w, http.StatusInternalServerError, "日志存储路径无效")
			return
		}
		if _, statErr := os.Stat(fullPath); statErr != nil {
			writeError(w, http.StatusNotFound, "服务端日志文件不存在")
			return
		}
		paths = append(paths, fullPath)
	}
	if kind == "file" {
		input, openErr := os.Open(paths[0])
		if openErr != nil {
			writeError(w, http.StatusNotFound, "服务端日志文件不存在")
			return
		}
		defer input.Close()
		info, statErr := input.Stat()
		if statErr != nil {
			writeError(w, http.StatusInternalServerError, "读取日志文件属性失败")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(selected[0].RelativePath)}))
		http.ServeContent(w, r, filepath.Base(selected[0].RelativePath), info.ModTime(), input)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="batch-`+uploadID+`.zip"`)
	archive := zip.NewWriter(w)
	for index, file := range selected {
		entryName := filepath.ToSlash(filepath.Clean(file.RelativePath))
		if entryName == "." || strings.HasPrefix(entryName, "../") || strings.HasPrefix(entryName, "/") {
			_ = archive.Close()
			return
		}
		if index > 0 {
			entryName = fmt.Sprintf("%d-%s", index+1, entryName)
		}
		entry, createErr := archive.Create(entryName)
		if createErr != nil {
			_ = archive.Close()
			return
		}
		input, openErr := os.Open(paths[index])
		if openErr != nil {
			_ = archive.Close()
			return
		}
		_, copyErr := io.Copy(entry, input)
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil {
			_ = archive.Close()
			return
		}
	}
	_ = archive.Close()
}

func safeUploadFilePath(root, relativePath string) (string, error) {
	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(cleanRelative) || cleanRelative == "." || cleanRelative == ".." || strings.HasPrefix(cleanRelative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid stored log path")
	}
	fullPath, err := filepath.Abs(filepath.Join(root, cleanRelative))
	if err != nil || (fullPath != root && !strings.HasPrefix(fullPath, root+string(filepath.Separator))) {
		return "", errors.New("invalid stored log path")
	}
	return fullPath, nil
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
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	aiStatus := strings.TrimSpace(r.URL.Query().Get("ai_status"))
	project := strings.TrimSpace(r.URL.Query().Get("project"))
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	sort := strings.TrimSpace(r.URL.Query().Get("sort"))
	items, total, err := s.repo.ListTasks(r.Context(), ownerOpenID, pageSize, (page-1)*pageSize, status, aiStatus, project, version, sort)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query tasks failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"total": total, "page": page, "page_size": pageSize, "list": items}})
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
	if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "batch" {
		var input struct {
			Action  string   `json:"action"`
			TaskIDs []string `json:"task_ids"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input) != nil || len(input.TaskIDs) == 0 || len(input.TaskIDs) > 100 || (input.Action != "retry" && input.Action != "cancel" && input.Action != "delete") {
			writeError(w, http.StatusBadRequest, "批量任务请求无效，操作必须为 retry、cancel 或 delete，任务数量必须为 1 到 100")
			return
		}
		items := make([]map[string]any, 0, len(input.TaskIDs))
		succeeded := 0
		for _, id := range input.TaskIDs {
			id = strings.TrimSpace(id)
			item := map[string]any{"task_id": id, "success": false}
			var err error
			switch input.Action {
			case "retry":
				_, err = s.repo.RequestTaskRetry(r.Context(), id, ownerOpenID)
			case "cancel":
				_, err = s.repo.CancelTask(r.Context(), id, ownerOpenID)
			case "delete":
				var storagePath string
				storagePath, err = s.repo.DeleteTask(r.Context(), id, ownerOpenID)
				if err == nil {
					err = removeUploadStorage(s.config.StorageDir, storagePath)
				}
			}
			if err == nil {
				item["success"] = true
				item["message"] = "操作成功"
				succeeded++
			} else if errors.Is(err, sql.ErrNoRows) {
				item["message"] = "任务不存在或无权访问"
			} else if errors.Is(err, ErrTaskNotRetryable) || errors.Is(err, ErrTaskNotCancellable) {
				item["message"] = "当前任务状态不允许该操作"
			} else {
				item["message"] = "操作失败"
			}
			items = append(items, item)
		}
		if input.Action == "retry" {
			s.signalParseQueue()
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "批量任务操作完成", Data: map[string]any{"action": input.Action, "total": len(items), "succeeded": succeeded, "failed": len(items) - succeeded, "items": items}})
		return
	}
	taskID := parts[1]
	if r.Method == http.MethodGet && len(parts) == 3 && parts[2] == "export" {
		s.taskExportHandler(w, r, taskID, ownerOpenID)
		return
	}
	if r.Method == http.MethodPut && len(parts) == 3 && parts[2] == "priority" {
		var input struct {
			Priority int `json:"priority"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || input.Priority < -100 || input.Priority > 100 {
			writeError(w, http.StatusBadRequest, "任务优先级必须在 -100 到 100 之间")
			return
		}
		err := s.repo.UpdateTaskPriority(r.Context(), taskID, ownerOpenID, input.Priority)
		if errors.Is(err, ErrTaskPriorityNotEditable) {
			writeError(w, http.StatusConflict, "只有排队或暂停任务可以修改优先级")
			return
		}
		if err != nil {
			handleQueryError(w, err)
			return
		}
		s.signalParseQueue()
		response.JSON(w, response.APIResponse{Code: 0, Message: "任务优先级已更新", Data: map[string]any{"task_id": taskID, "priority": input.Priority}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "pause" {
		alreadyPaused, err := s.repo.PauseTask(r.Context(), taskID, ownerOpenID)
		if errors.Is(err, ErrTaskNotPausable) {
			writeError(w, http.StatusConflict, "当前任务状态不能暂停")
			return
		}
		if err != nil {
			handleQueryError(w, err)
			return
		}
		message := "任务已暂停"
		if alreadyPaused {
			message = "任务已经处于暂停状态"
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: message, Data: map[string]any{"task_id": taskID, "status": "paused"}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "resume" {
		err := s.repo.ResumeTask(r.Context(), taskID, ownerOpenID)
		if errors.Is(err, ErrTaskNotResumable) {
			writeError(w, http.StatusConflict, "只有暂停任务可以恢复")
			return
		}
		if err != nil {
			handleQueryError(w, err)
			return
		}
		s.signalParseQueue()
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "任务已恢复并重新排队", Data: map[string]any{"task_id": taskID, "status": "queued"}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "retry" {
		alreadyQueued, err := s.repo.RequestTaskRetry(r.Context(), taskID, ownerOpenID)
		if err != nil {
			if errors.Is(err, ErrTaskNotRetryable) {
				writeError(w, http.StatusConflict, ErrTaskNotRetryable.Error())
				return
			}
			handleQueryError(w, err)
			return
		}
		s.signalParseQueue()
		message := "task retry queued"
		if alreadyQueued {
			message = "task retry already queued"
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: message, Data: map[string]any{
			"task_id": taskID, "status": "queued",
		}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "agent-retry" {
		if !s.analysisEnabled(r.Context()) {
			writeError(w, http.StatusServiceUnavailable, "AI analysis is not configured")
			return
		}
		input, err := s.repo.PrepareAgentRetry(r.Context(), taskID, ownerOpenID)
		if err != nil {
			if errors.Is(err, ErrAgentRetryNotReady) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			if errors.Is(err, ErrAgentRetryQueued) {
				response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: err.Error(), Data: map[string]any{"task_id": taskID, "status": "queued"}})
				return
			}
			handleQueryError(w, err)
			return
		}
		queued := 0
		for _, file := range input.Files {
			if s.enqueueAgentAnalysis(agentJob{
				taskID: input.TaskID, uploadID: input.UploadID, ownerOpenID: ownerOpenID,
				attemptNo: input.AttemptNo, file: file, totalLines: file.LineCount,
				matches: input.Matches[file.RelativePath],
			}) {
				queued++
			}
		}
		if queued == 0 {
			_ = s.repo.ClearAgentRetryRequested(r.Context(), input.TaskID, input.AttemptNo)
			writeError(w, http.StatusServiceUnavailable, "AI worker queue is full")
			return
		}
		if queued == len(input.Files) {
			if s.dynamicLLM {
				if !s.enqueueAgentAnalysis(agentJob{taskID: input.TaskID, uploadID: input.UploadID, ownerOpenID: ownerOpenID, attemptNo: input.AttemptNo, overview: true}) {
					_ = s.repo.ClearAgentRetryRequested(r.Context(), input.TaskID, input.AttemptNo)
					writeError(w, http.StatusServiceUnavailable, "AI worker queue is full")
					return
				}
			} else {
				_ = s.repo.ClearAgentRetryRequested(r.Context(), input.TaskID, input.AttemptNo)
			}
		} else {
			_ = s.repo.ClearAgentRetryRequested(r.Context(), input.TaskID, input.AttemptNo)
			writeError(w, http.StatusServiceUnavailable, "AI worker queue is full")
			return
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "AI retry queued", Data: map[string]any{
			"task_id": taskID, "status": "queued", "files_queued": queued,
		}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 4 && parts[2] == "agent-retry" {
		if !s.analysisEnabled(r.Context()) {
			writeError(w, http.StatusServiceUnavailable, "AI 分析尚未配置")
			return
		}
		fileID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || fileID <= 0 {
			writeError(w, http.StatusBadRequest, "文件 ID 格式错误")
			return
		}
		input, err := s.repo.PrepareAgentFileRetry(r.Context(), taskID, ownerOpenID, fileID)
		if err != nil {
			if errors.Is(err, ErrAgentRetryNotReady) {
				writeError(w, http.StatusConflict, "规则解析完成后才能重试 AI 分析")
				return
			}
			if errors.Is(err, ErrAgentRetryQueued) {
				response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "AI 重试已经在队列中", Data: map[string]any{"task_id": taskID, "file_id": fileID, "status": "queued"}})
				return
			}
			handleQueryError(w, err)
			return
		}
		file := input.Files[0]
		queued := s.enqueueAgentAnalysis(agentJob{
			taskID: input.TaskID, uploadID: input.UploadID, ownerOpenID: ownerOpenID,
			attemptNo: input.AttemptNo, file: file, totalLines: file.LineCount,
			matches: input.Matches[file.RelativePath], refreshOverview: true,
		})
		if !queued {
			_ = s.repo.ClearAgentRetryRequested(r.Context(), input.TaskID, input.AttemptNo)
			writeError(w, http.StatusServiceUnavailable, "AI 任务队列已满，请稍后重试")
			return
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "文件 AI 重试已进入队列", Data: map[string]any{
			"task_id": taskID, "file_id": fileID, "status": "queued",
		}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "agent-cancel" {
		alreadyCancelled, err := s.repo.CancelAgentAnalysis(r.Context(), taskID, ownerOpenID)
		if errors.Is(err, ErrAgentNotCancellable) {
			writeError(w, http.StatusConflict, "规则解析完成后才能取消 AI 分析")
			return
		}
		if err != nil {
			handleQueryError(w, err)
			return
		}
		message := "AI 分析取消请求已提交"
		if alreadyCancelled {
			message = "AI 分析已经取消"
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: message, Data: map[string]any{
			"task_id": taskID, "status": "cancelled",
		}})
		return
	}
	if r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "cancel" {
		alreadyCancelled, err := s.repo.CancelTask(r.Context(), taskID, ownerOpenID)
		if err != nil {
			if errors.Is(err, ErrTaskNotCancellable) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			handleQueryError(w, err)
			return
		}
		message := "task cancelled"
		if alreadyCancelled {
			message = "task already cancelled"
		}
		response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: message, Data: map[string]any{
			"task_id": taskID, "status": "cancelled",
		}})
		return
	}
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
		overview, overviewErr := s.repo.TaskOverview(r.Context(), taskID, ownerOpenID)
		if overviewErr == nil {
			results = append([]AgentAnalysisRecord{overview.AsAgentAnalysisRecord()}, results...)
		} else if overviewErr != sql.ErrNoRows {
			writeError(w, http.StatusInternalServerError, "query task AI overview failed")
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
		"task": upload, "files": files, "agent_enabled": s.analysisEnabled(r.Context()),
	}})
}

func (s *Service) taskExportHandler(w http.ResponseWriter, r *http.Request, taskID, ownerOpenID string) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "csv" && format != "json" && format != "report" {
		writeError(w, http.StatusBadRequest, "导出格式必须为 csv、json 或 report")
		return
	}
	upload, files, err := s.repo.GetUploadByTask(r.Context(), taskID, ownerOpenID)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	results, err := s.repo.Results(r.Context(), taskID, ownerOpenID, 1000000, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询导出结果失败")
		return
	}
	filename := "task-" + taskID
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
		writer := csv.NewWriter(w)
		defer writer.Flush()
		_ = writer.Write([]string{"结果ID", "状态", "负责人", "文件", "级别", "规则", "分类", "行号", "命中文本", "日志内容"})
		for _, item := range results {
			_ = writer.Write([]string{strconv.FormatInt(item.ID, 10), item.Status, item.AssignedTo, item.FilePath, item.Level, item.RuleName, item.Category, strconv.FormatInt(item.LineNumber, 10), item.MatchedText, item.Content})
		}
	case "json":
		agentResults, agentErr := s.repo.AgentResults(r.Context(), taskID, ownerOpenID)
		if agentErr != nil {
			writeError(w, http.StatusInternalServerError, "查询 AI 导出结果失败")
			return
		}
		if overview, overviewErr := s.repo.TaskOverview(r.Context(), taskID, ownerOpenID); overviewErr == nil {
			agentResults = append([]AgentAnalysisRecord{overview.AsAgentAnalysisRecord()}, agentResults...)
		} else if !errors.Is(overviewErr, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "查询任务 AI 总览失败")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.json"`)
		_ = json.NewEncoder(w).Encode(map[string]any{"task": upload, "files": files, "results": results, "agent_results": agentResults, "exported_at": time.Now()})
	case "report":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`.md"`)
		fmt.Fprintf(w, "# LogMaster 任务分析报告\n\n- 任务 ID：%s\n- 项目：%s\n- 版本：%s\n- 状态：%s\n- 文件数：%d\n- 错误数：%d\n- 警告数：%d\n- 导出时间：%s\n\n", taskID, upload.ProjectName, upload.Version, upload.Status, len(files), upload.ErrorCount, upload.WarningCount, time.Now().Format(time.RFC3339))
		if overview, overviewErr := s.repo.TaskOverview(r.Context(), taskID, ownerOpenID); overviewErr == nil {
			fmt.Fprintf(w, "## AI 总体结论\n\n%s\n\n风险等级：%s\n\n", overview.Summary, overview.RiskLevel)
		}
		fmt.Fprintln(w, "## 异常明细")
		fmt.Fprintln(w)
		for _, item := range results {
			fmt.Fprintf(w, "- `%s:%d` [%s] %s：%s\n", item.FilePath, item.LineNumber, item.Level, item.RuleName, item.MatchedText)
		}
	}
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
	message = chineseErrorMessage(message)
	response.JSONStatus(w, status, response.APIResponse{Code: status, Message: message, Data: nil})
}

func chineseErrorMessage(message string) string {
	translations := []struct{ prefix, text string }{
		{"invalid upload session request", "上传会话请求格式无效"},
		{"uploader email is not an active enterprise member", "上传邮箱不是有效的企业成员"},
		{"validate uploader email with Feishu failed", "上传邮箱企业身份校验失败"},
		{"project does not exist", "项目不存在"},
		{"invalid multipart upload or upload size exceeded", "上传文件格式无效或超过大小限制"},
		{"at least one file is required", "至少需要上传一个文件"},
		{"too many files in one upload", "单次上传文件数量超过限制"},
		{"save upload metadata failed", "保存上传信息失败"},
		{"queue upload failed", "上传任务入队失败"},
		{"parse log file", "解析日志文件失败"},
		{"extract uploaded archive", "解压上传压缩包失败"},
		{"unsupported file type", "不支持的文件类型"},
		{"archive contains no supported log files", "压缩包中没有可解析的日志文件"},
		{"archive exceeds extraction limit", "压缩包解压后超过大小限制"},
		{"archive contains too many files", "压缩包文件数量超过限制"},
	}
	for _, item := range translations {
		if strings.HasPrefix(message, item.prefix) {
			return item.text + "：" + strings.TrimSpace(strings.TrimPrefix(message, item.prefix))
		}
	}
	return message
}

func parsePageParams(r *http.Request, defaultSize, maxSize int) (int, int, error) {
	page, pageSize := 1, defaultSize
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("invalid page")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		pageSize, err = strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > maxSize {
			return 0, 0, fmt.Errorf("invalid page_size")
		}
	}
	return page, pageSize, nil
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

func (s *Service) collectorProjectsSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.uploadUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	projects, err := s.repo.ListCollectorProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query collector projects failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: projects})
}

func (s *Service) collectorScenariosSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.uploadUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	scenarios, err := s.repo.ListCollectorScenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query collector scenarios failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: scenarios})
}

type collectorSyncSnapshot struct {
	Projects  []CollectorProject `json:"projects"`
	Scenarios []TestScenario     `json:"scenarios"`
	Keywords  []StandardKeyword  `json:"keywords"`
	SyncedAt  time.Time          `json:"synced_at"`
}

func (s *Service) collectorSyncHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.uploadUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "login required or upload token invalid")
		return
	}
	ctx := r.Context()
	projects, err := s.repo.ListCollectorProjects(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query collector projects failed")
		return
	}
	scenarios, err := s.repo.ListCollectorScenarios(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query collector scenarios failed")
		return
	}
	keywords, err := s.repo.ListStandardKeywords(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query standard keywords failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: collectorSyncSnapshot{
		Projects: projects, Scenarios: scenarios, Keywords: keywords, SyncedAt: time.Now().UTC(),
	}})
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
