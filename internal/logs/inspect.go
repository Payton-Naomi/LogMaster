package logs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	zip "github.com/yeka/zip"

	"logmaster-agent/internal/response"
)

type InspectedEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Encrypted bool   `json:"encrypted"`
}

func (s *Service) inspectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxUploadBytes+(4<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid file or file size exceeded")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	headers := r.MultipartForm.File["file"]
	if len(headers) != 1 {
		writeError(w, http.StatusBadRequest, "exactly one file is required")
		return
	}

	root := filepath.Join(s.config.StorageDir, ".inspect", newID())
	defer os.RemoveAll(root)
	path, _, err := saveUploadedFile(headers[0], root, s.config.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, archive, err := inspectLogFile(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"archive": archive, "entries": entries}})
}

func inspectLogFile(sourcePath string) ([]InspectedEntry, bool, error) {
	lower := strings.ToLower(filepath.Base(sourcePath))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		entries, err := inspectZIP(sourcePath)
		return entries, true, err
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		entries, err := inspectTarGZ(sourcePath)
		return entries, true, err
	case strings.HasSuffix(lower, ".gz"):
		entries, err := inspectGZ(sourcePath)
		return entries, true, err
	default:
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, false, err
		}
		return []InspectedEntry{{Path: filepath.Base(sourcePath), SizeBytes: info.Size()}}, false, nil
	}
}

func inspectZIP(sourcePath string) ([]InspectedEntry, error) {
	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return nil, fmt.Errorf("archive contains too many files")
	}
	entries := make([]InspectedEntry, 0)
	for _, item := range reader.File {
		if item.FileInfo().IsDir() || !isLogFile(item.Name) {
			continue
		}
		name, err := safeArchiveName(item.Name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, InspectedEntry{Path: filepath.ToSlash(name), SizeBytes: int64(item.UncompressedSize64), Encrypted: item.IsEncrypted()})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("archive contains no supported log files")
	}
	return entries, nil
}

func inspectTarGZ(sourcePath string) ([]InspectedEntry, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	entries := make([]InspectedEntry, 0)
	for count := 0; ; count++ {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if count >= maxArchiveFiles {
			return nil, fmt.Errorf("archive contains too many files")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || !isLogFile(header.Name) {
			continue
		}
		name, err := safeArchiveName(header.Name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, InspectedEntry{Path: filepath.ToSlash(name), SizeBytes: header.Size})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("archive contains no supported log files")
	}
	return entries, nil
}

func inspectGZ(sourcePath string) ([]InspectedEntry, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	name := gz.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	clean, err := safeArchiveName(name)
	if err != nil {
		return nil, err
	}
	if !isLogFile(clean) {
		return nil, fmt.Errorf("gzip content is not a supported log file")
	}
	return []InspectedEntry{{Path: filepath.ToSlash(clean)}}, nil
}
