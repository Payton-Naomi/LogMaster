package logservice

import (
	"os"
	"path/filepath"
	"strings"
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

func TestExtractUnencryptedZIPWithoutPassword(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "logs.zip")
	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("device/system.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("INFO no password needed\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	files, err := collectLogFilesWithPasswords(archivePath, filepath.Join(root, "upload"), 1024*1024, []string{"unused-password"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	content, err := os.ReadFile(filepath.Join(root, "upload", filepath.FromSlash(files[0].RelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "INFO no password needed\n" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestSafeArchiveNameAcceptsRootedEntry(t *testing.T) {
	clean, err := safeArchiveName("/logfile_0")
	if err != nil {
		t.Fatalf("safeArchiveName() error = %v", err)
	}
	if clean != "logfile_0" {
		t.Fatalf("safeArchiveName() = %q, want logfile_0", clean)
	}
}

func TestSafeTargetRenamesExistingFile(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "device.log")
	if err := os.WriteFile(existing, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}

	target, relative, err := safeTarget(root, "device.log")
	if err != nil {
		t.Fatal(err)
	}
	if target == existing {
		t.Fatal("safeTarget returned the existing file")
	}
	if !strings.HasPrefix(relative, "device_") || !strings.HasSuffix(relative, ".log") {
		t.Fatalf("renamed path = %q", relative)
	}
}
