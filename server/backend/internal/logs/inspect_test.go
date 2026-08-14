package logs

import (
	"os"
	"path/filepath"
	"testing"

	zip "github.com/yeka/zip"
)

func TestInspectExtensionlessLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logfile_0")
	if err := os.WriteFile(path, []byte("INFO started\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, archive, err := inspectLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if archive || len(entries) != 1 || entries[0].Path != "logfile_0" || entries[0].SizeBytes == 0 {
		t.Fatalf("archive=%v entries=%+v", archive, entries)
	}
}

func TestInspectZIPNormalizesRootedEntry(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "logs.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Encrypt("/logfile_0", defaultArchivePassword, zip.AES256Encryption)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("ERROR failed\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	entries, archive, err := inspectLogFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !archive || len(entries) != 1 || entries[0].Path != "logfile_0" || !entries[0].Encrypted {
		t.Fatalf("archive=%v entries=%+v", archive, entries)
	}
}
