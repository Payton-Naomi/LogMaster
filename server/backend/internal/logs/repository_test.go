package logs

import "testing"

func TestUploadProgress(t *testing.T) {
	tests := []struct {
		name   string
		upload Upload
		want   int
	}{
		{name: "uploading", upload: Upload{Status: "uploading"}, want: 10},
		{name: "queued", upload: Upload{Status: "queued"}, want: 25},
		{name: "parsing without totals", upload: Upload{Status: "parsing"}, want: 30},
		{name: "parsing by bytes", upload: Upload{Status: "parsing", TotalBytes: 1000, ProcessedBytes: 500}, want: 62},
		{name: "parsing by processed files", upload: Upload{Status: "parsing", TotalFiles: 4, ProcessedFiles: 2}, want: 62},
		{name: "parsing caps before completion", upload: Upload{Status: "parsing", TotalFiles: 1, ProcessedFiles: 1}, want: 95},
		{name: "completed", upload: Upload{Status: "completed"}, want: 100},
		{name: "failed", upload: Upload{Status: "failed"}, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadProgress(tt.upload); got != tt.want {
				t.Fatalf("uploadProgress() = %d, want %d", got, tt.want)
			}
		})
	}
}
