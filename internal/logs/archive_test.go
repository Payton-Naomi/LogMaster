package logs

import (
	"os"
	"path/filepath"
	"testing"

	zip "github.com/yeka/zip"
)

func TestExtractEncryptedZIPWithDefaultPassword(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "logs.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Encrypt("device/system.log", defaultArchivePassword, zip.AES256Encryption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("INFO started\nERROR failed\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	files, err := collectLogFiles(archivePath, filepath.Join(root, "upload"), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].SizeBytes == 0 {
		t.Fatalf("unexpected files: %+v", files)
	}
	content, err := os.ReadFile(filepath.Join(root, "upload", filepath.FromSlash(files[0].RelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "INFO started\nERROR failed\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}
