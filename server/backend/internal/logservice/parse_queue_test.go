package logservice

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestStoredUploadItemsRebuildsOrderedSources(t *testing.T) {
	root := t.TempDir()
	writeStoredUploadSource(t, root, 2, "second.log")
	writeStoredUploadSource(t, root, 1, "first.zip")

	items, err := storedUploadItems(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].index != 1 || filepath.Base(items[0].storedPath) != "first.zip" {
		t.Fatalf("first item = %#v", items[0])
	}
	if items[1].index != 2 || filepath.Base(items[1].storedPath) != "second.log" {
		t.Fatalf("second item = %#v", items[1])
	}
}

func TestStoredUploadItemsRejectsMissingSource(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "items", "1", "original"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := storedUploadItems(root); err == nil {
		t.Fatal("expected missing source error")
	}
}

func writeStoredUploadSource(t *testing.T, root string, index int, name string) {
	t.Helper()
	directory := filepath.Join(root, "items", strconv.Itoa(index), "original")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
}
