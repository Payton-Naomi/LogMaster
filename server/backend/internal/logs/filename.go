package logs

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func uniqueFileName(name string, used func(string) bool, now time.Time) string {
	if !used(name) {
		return name
	}

	directory, base := filepath.Split(name)
	extension := filepath.Ext(base)
	if strings.HasSuffix(strings.ToLower(base), ".tar.gz") {
		extension = base[len(base)-len(".tar.gz"):]
	}
	stem := strings.TrimSuffix(base, extension)
	if stem == "" {
		stem, extension = base, ""
	}
	timestamp := fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/int(time.Millisecond))

	for sequence := 1; ; sequence++ {
		suffix := "_" + timestamp
		if sequence > 1 {
			suffix += fmt.Sprintf("_%d", sequence)
		}
		candidate := filepath.Join(directory, stem+suffix+extension)
		if !used(candidate) {
			return candidate
		}
	}
}
