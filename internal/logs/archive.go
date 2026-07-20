package logs

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	zip "github.com/yeka/zip"
)

const defaultArchivePassword = "70M_dashcam_^"
const maxArchiveFiles = 10000

func collectLogFiles(sourcePath, uploadRoot string, maxExtractBytes int64) ([]LogFile, error) {
	lower := strings.ToLower(filepath.Base(sourcePath))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZIP(sourcePath, filepath.Join(uploadRoot, "extracted"), maxExtractBytes)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGZ(sourcePath, filepath.Join(uploadRoot, "extracted"), maxExtractBytes)
	case strings.HasSuffix(lower, ".gz"):
		return extractGZ(sourcePath, filepath.Join(uploadRoot, "extracted"), maxExtractBytes)
	default:
		if !isLogFile(lower) {
			return nil, fmt.Errorf("unsupported file type: %s", filepath.Base(sourcePath))
		}
		return []LogFile{fileMetadata(sourcePath, filepath.ToSlash(filepath.Join("original", filepath.Base(sourcePath))))}, nil
	}
}

func extractZIP(sourcePath, destination string, maxBytes int64) ([]LogFile, error) {
	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return nil, fmt.Errorf("archive contains too many files")
	}

	var declared uint64
	for _, entry := range reader.File {
		declared += entry.UncompressedSize64
		if declared > uint64(maxBytes) {
			return nil, fmt.Errorf("archive exceeds extraction limit")
		}
	}
	if err := os.MkdirAll(destination, 0o750); err != nil {
		return nil, err
	}
	var files []LogFile
	var written int64
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if !isLogFile(entry.Name) {
			continue
		}
		target, relative, err := safeTarget(destination, entry.Name)
		if err != nil {
			return nil, err
		}
		if entry.IsEncrypted() {
			entry.SetPassword(defaultArchivePassword)
		}
		source, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", entry.Name, err)
		}
		size, digest, err := writeExtracted(target, source, maxBytes-written)
		source.Close()
		if err != nil {
			return nil, fmt.Errorf("extract zip entry %q: %w", entry.Name, err)
		}
		written += size
		if isLogFile(relative) {
			files = append(files, LogFile{RelativePath: filepath.ToSlash(filepath.Join("extracted", relative)), SizeBytes: size, SHA256: digest})
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("archive contains no supported log files")
	}
	return files, nil
}

func extractTarGZ(sourcePath, destination string, maxBytes int64) ([]LogFile, error) {
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
	if err := os.MkdirAll(destination, 0o750); err != nil {
		return nil, err
	}
	reader := tar.NewReader(gz)
	var files []LogFile
	var written int64
	var count int
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if !isLogFile(header.Name) {
			continue
		}
		count++
		if count > maxArchiveFiles {
			return nil, fmt.Errorf("archive contains too many files")
		}
		target, relative, err := safeTarget(destination, header.Name)
		if err != nil {
			return nil, err
		}
		size, digest, err := writeExtracted(target, io.LimitReader(reader, header.Size), maxBytes-written)
		if err != nil {
			return nil, fmt.Errorf("extract tar entry %q: %w", header.Name, err)
		}
		written += size
		if isLogFile(relative) {
			files = append(files, LogFile{RelativePath: filepath.ToSlash(filepath.Join("extracted", relative)), SizeBytes: size, SHA256: digest})
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("archive contains no supported log files")
	}
	return files, nil
}

func extractGZ(sourcePath, destination string, maxBytes int64) ([]LogFile, error) {
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
	if !isLogFile(name) {
		return nil, fmt.Errorf("gzip content is not a supported log file")
	}
	target, relative, err := safeTarget(destination, name)
	if err != nil {
		return nil, err
	}
	size, digest, err := writeExtracted(target, gz, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("extract gzip: %w", err)
	}
	return []LogFile{{RelativePath: filepath.ToSlash(filepath.Join("extracted", relative)), SizeBytes: size, SHA256: digest}}, nil
}

func safeTarget(root, name string) (string, string, error) {
	clean, err := safeArchiveName(name)
	if err != nil {
		return "", "", err
	}
	target := filepath.Join(root, clean)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("unsafe archive path %q", name)
	}
	return target, relative, nil
}

func safeArchiveName(name string) (string, error) {
	normalized := strings.TrimLeft(strings.ReplaceAll(name, "\\", "/"), "/")
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	if filepath.VolumeName(clean) != "" || strings.Contains(strings.Split(filepath.ToSlash(clean), "/")[0], ":") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return clean, nil
}

func writeExtracted(target string, source io.Reader, remaining int64) (int64, string, error) {
	if remaining <= 0 {
		return 0, "", fmt.Errorf("archive exceeds extraction limit")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return 0, "", err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(source, remaining+1))
	closeErr := output.Close()
	if copyErr != nil {
		return 0, "", copyErr
	}
	if closeErr != nil {
		return 0, "", closeErr
	}
	if written > remaining {
		os.Remove(target)
		return 0, "", fmt.Errorf("archive exceeds extraction limit")
	}
	return written, fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func fileMetadata(path, relative string) LogFile {
	file, err := os.Open(path)
	if err != nil {
		return LogFile{RelativePath: relative}
	}
	defer file.Close()
	hash := sha256.New()
	size, _ := io.Copy(hash, file)
	return LogFile{RelativePath: relative, SizeBytes: size, SHA256: fmt.Sprintf("%x", hash.Sum(nil))}
}

func isLogFile(name string) bool {
	lower := strings.ToLower(name)
	ext := filepath.Ext(lower)
	return ext == ".log" || ext == ".txt" || ext == ".out" || ext == ".csv" || ext == ""
}
