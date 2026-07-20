package mockserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
)

var batchIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Server struct {
	Dir       string
	FailFirst int64
	attempts  atomic.Int64
	mu        sync.Mutex
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/logs/upload", s.handleUpload)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return mux
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.attempts.Add(1) <= s.FailFirst {
		http.Error(w, "simulated failure", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Content-Encoding") != "gzip" {
		http.Error(w, "gzip required", http.StatusUnsupportedMediaType)
		return
	}
	zipper, err := gzip.NewReader(r.Body)
	if err != nil {
		http.Error(w, "invalid gzip", http.StatusBadRequest)
		return
	}
	defer zipper.Close()
	var batch model.UploadBatch
	if err := json.NewDecoder(io.LimitReader(zipper, 32*1024*1024)).Decode(&batch); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if !batchIDPattern.MatchString(batch.BatchID) || r.Header.Get("Idempotency-Key") != batch.BatchID || batch.AgentID == "" || batch.ProjectID == "" || batch.DeviceSN == "" || len(batch.Logs) == 0 {
		http.Error(w, "invalid batch", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.Dir, batch.BatchID+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		data, err := json.MarshalIndent(batch, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		http.Error(w, fmt.Sprintf("stat batch: %v", err), http.StatusInternalServerError)
		return
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var stored model.UploadBatch
		if err := json.Unmarshal(data, &stored); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if stored.AgentID != batch.AgentID || stored.ProjectID != batch.ProjectID || stored.DeviceSN != batch.DeviceSN || len(stored.Logs) != len(batch.Logs) {
			http.Error(w, "batch id reused with different payload", http.StatusConflict)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(model.UploadResponse{Accepted: true, BatchID: batch.BatchID, Received: len(batch.Logs)})
}
