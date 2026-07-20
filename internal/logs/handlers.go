package logs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"logmaster-agent/internal/response"
)

func (s *Service) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes+(16<<20))
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
	projectName := strings.TrimSpace(r.FormValue("project_name"))
	if projectName == "" {
		projectName = "default"
	}
	if len(projectName) > 128 {
		writeError(w, http.StatusBadRequest, "project_name is too long")
		return
	}
	version := strings.TrimSpace(r.FormValue("version"))
	if len(version) > 64 {
		writeError(w, http.StatusBadRequest, "version is too long")
		return
	}

	uploadID, taskID := newID(), newID()
	uploadRoot := filepath.Join(s.config.StorageDir, uploadID)
	if err := os.MkdirAll(uploadRoot, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "create upload storage failed")
		return
	}
	if err := s.repo.CreateUpload(r.Context(), uploadID, taskID, projectName, version, uploadRoot); err != nil {
		os.RemoveAll(uploadRoot)
		writeError(w, http.StatusInternalServerError, "create upload record failed")
		return
	}

	var totalSize int64
	var originalNames []string
	var logFiles []LogFile
	var extractedSize int64
	for index, header := range headers {
		itemRoot := filepath.Join(uploadRoot, "items", strconv.Itoa(index+1))
		storedPath, size, err := saveUploadedFile(header, itemRoot, s.config.MaxUploadBytes-totalSize)
		if err != nil {
			s.failUpload(w, uploadID, uploadRoot, err)
			return
		}
		totalSize += size
		originalNames = append(originalNames, filepath.Base(header.Filename))
		files, err := collectLogFiles(storedPath, itemRoot, s.config.MaxExtractBytes-extractedSize)
		if err != nil {
			s.failUpload(w, uploadID, uploadRoot, err)
			return
		}
		for i := range files {
			if strings.Contains(filepath.ToSlash(files[i].RelativePath), "/extracted/") || strings.HasPrefix(filepath.ToSlash(files[i].RelativePath), "extracted/") {
				extractedSize += files[i].SizeBytes
			}
			files[i].RelativePath = filepath.ToSlash(filepath.Join("items", strconv.Itoa(index+1), filepath.FromSlash(files[i].RelativePath)))
		}
		logFiles = append(logFiles, files...)
	}
	if err := s.repo.QueueUpload(r.Context(), uploadID, strings.Join(originalNames, ", "), totalSize, logFiles); err != nil {
		os.RemoveAll(uploadRoot)
		s.repo.MarkFailed(context.Background(), uploadID, "queue upload failed")
		writeError(w, http.StatusInternalServerError, "save upload metadata failed")
		return
	}
	go s.processUpload(uploadID)
	response.JSONStatus(w, http.StatusAccepted, response.APIResponse{Code: 0, Message: "upload accepted", Data: map[string]any{
		"upload_id": uploadID, "task_id": taskID, "status": "queued", "file_count": len(logFiles),
	}})
}

func (s *Service) failUpload(w http.ResponseWriter, uploadID, uploadRoot string, err error) {
	os.RemoveAll(uploadRoot)
	s.repo.MarkFailed(context.Background(), uploadID, err.Error())
	writeError(w, http.StatusBadRequest, err.Error())
}

func saveUploadedFile(header *multipart.FileHeader, itemRoot string, remaining int64) (string, int64, error) {
	if remaining <= 0 {
		return "", 0, fmt.Errorf("upload size exceeded")
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
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
	page, pageSize := pagination(r)
	items, total, err := s.repo.ListUploads(r.Context(), pageSize, (page-1)*pageSize)
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
	id := lastPathPart(r.URL.Path)
	upload, files, err := s.repo.GetUpload(r.Context(), id)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"upload": upload, "files": files}})
}

func (s *Service) listTasksHandler(w http.ResponseWriter, r *http.Request) {
	s.listUploadsHandler(w, r)
}

func (s *Service) taskHandler(w http.ResponseWriter, r *http.Request) {
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
		storagePath, err := s.repo.DeleteTask(r.Context(), taskID)
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
		results, err := s.repo.Results(r.Context(), taskID, pageSize, (page-1)*pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query task results failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: results})
		return
	}
	if len(parts) == 3 && parts[2] == "agent-results" {
		results, err := s.repo.AgentResults(r.Context(), taskID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query agent results failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: results})
		return
	}
	upload, files, err := s.repo.GetUploadByTask(r.Context(), taskID)
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
