package logs

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLog(t *testing.T) {
	input := "INFO boot complete\nERROR disk full\nwarning temperature high\nFATAL recorder crashed\n"
	summary, err := parseLog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Lines != 4 || summary.Errors != 2 || summary.Warnings != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Results) != 3 {
		t.Fatalf("got %d results", len(summary.Results))
	}
}

func TestSafeTargetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../secret.log", "folder/../../secret.log", "C:/secret.log"} {
		if _, _, err := safeTarget(root, name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestSafeTargetNormalizesRootedDevicePath(t *testing.T) {
	root := t.TempDir()
	target, relative, err := safeTarget(root, "/logfile_0")
	if err != nil {
		t.Fatal(err)
	}
	if relative != "logfile_0" || target != filepath.Join(root, "logfile_0") {
		t.Fatalf("target=%q relative=%q", target, relative)
	}
}
