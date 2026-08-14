package logs

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"logmaster-agent/internal/response"
)

type UploadCapacity struct {
	MaxUploadBytes    int64 `json:"max_upload_bytes"`
	MaxFilesPerUpload int   `json:"max_files_per_upload"`
}

func (r *Repository) UploadCapacity(ctx context.Context, fallbackBytes int64, fallbackFiles int) (UploadCapacity, error) {
	capacity := UploadCapacity{MaxUploadBytes: fallbackBytes, MaxFilesPerUpload: fallbackFiles}
	err := r.db.QueryRowContext(ctx, `SELECT max_upload_bytes, max_files_per_upload
		FROM logmaster_api.upload_capacity_config WHERE singleton = TRUE`).
		Scan(&capacity.MaxUploadBytes, &capacity.MaxFilesPerUpload)
	if errors.Is(err, sql.ErrNoRows) {
		return capacity, nil
	}
	return capacity, err
}

func (s *Service) uploadConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	capacity, err := s.repo.UploadCapacity(r.Context(), s.config.MaxUploadBytes, s.config.MaxFilesPerUpload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query upload capacity failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: capacity})
}
