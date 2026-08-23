package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"logmaster-agent/internal/response"
)

type archivePassword struct {
	ID        int64  `json:"id"`
	Masked    string `json:"masked"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Service) archivePasswordsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requirePermission(w, r, permissionKeywords); !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := s.db.QueryContext(r.Context(), `SELECT id, password, created_at::text, updated_at::text FROM logmaster_api.archive_passwords ORDER BY updated_at DESC, id DESC`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询解压密码失败")
			return
		}
		defer rows.Close()
		items := make([]archivePassword, 0)
		for rows.Next() {
			var id int64
			var password, createdAt, updatedAt string
			if err := rows.Scan(&id, &password, &createdAt, &updatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "读取解压密码失败")
				return
			}
			items = append(items, archivePassword{ID: id, Masked: maskArchivePassword(password), CreatedAt: createdAt, UpdatedAt: updatedAt})
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "查询成功", Data: items})
		return
	}
	if r.Method == http.MethodPost {
		openID, _ := s.currentUser(r)
		var input struct {
			Password string `json:"password"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil || strings.TrimSpace(input.Password) == "" || len([]rune(input.Password)) > 256 {
			writeError(w, http.StatusBadRequest, "解压密码不能为空且长度不能超过256个字符")
			return
		}
		_, err := s.db.ExecContext(r.Context(), `INSERT INTO logmaster_api.archive_passwords (password, created_by_open_id) VALUES ($1,$2) ON CONFLICT (password) DO UPDATE SET updated_at=NOW()`, strings.TrimSpace(input.Password), openID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "保存解压密码失败")
			return
		}
		response.JSONStatus(w, http.StatusCreated, response.APIResponse{Code: 0, Message: "解压密码已保存", Data: nil})
		return
	}
	if r.Method == http.MethodDelete {
		id, err := strconv.ParseInt(strings.TrimPrefix(strings.Trim(r.URL.Path, "/"), "api/admin/archive-passwords/"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "解压密码编号无效")
			return
		}
		if _, err = s.db.ExecContext(r.Context(), `DELETE FROM logmaster_api.archive_passwords WHERE id=$1`, id); err != nil {
			writeError(w, http.StatusInternalServerError, "删除解压密码失败")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "解压密码已删除", Data: nil})
		return
	}
	methodNotAllowed(w)
}

func maskArchivePassword(password string) string {
	runes := []rune(password)
	if len(runes) <= 2 {
		return "**"
	}
	return string(runes[0]) + "***" + string(runes[len(runes)-1])
}
