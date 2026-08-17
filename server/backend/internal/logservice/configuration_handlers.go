package logservice

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"logmaster-agent/internal/response"
)

func (s *Service) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days != 30 {
		days = 7
	}
	stats, err := s.repo.Dashboard(r.Context(), ownerOpenID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query dashboard failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: stats})
}

func (s *Service) projectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if _, ok := s.requireCurrentUser(w, r); !ok {
		return
	}
	projects, err := s.repo.Projects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query projects failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: projects})
}

func (s *Service) rulesHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := s.repo.ListRules(r.Context(), ownerOpenID)
		if err != nil {
			writeError(w, 500, "query rules failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: rules})
	case http.MethodPost:
		writeError(w, http.StatusForbidden, "规则只能由管理员通过异常关键词模块创建")
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) ruleBatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	var request struct {
		IDs     []int64 `json:"ids"`
		Enabled bool    `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil || len(request.IDs) == 0 || len(request.IDs) > 500 {
		writeError(w, http.StatusBadRequest, "select between 1 and 500 rules")
		return
	}
	seen := make(map[int64]struct{}, len(request.IDs))
	ids := make([]int64, 0, len(request.IDs))
	for _, id := range request.IDs {
		if id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid rule id")
			return
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	updated, err := s.repo.SaveRuleSettingsBatch(r.Context(), ownerOpenID, ids, request.Enabled)
	if err != nil {
		handleQueryError(w, err)
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: map[string]any{"updated": updated}})
}

func (s *Service) ruleHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(lastPathPart(r.URL.Path), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid rule id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var rule ParseRule
		if err := decodeJSON(r, &rule); err != nil {
			writeError(w, 400, "invalid rule")
			return
		}
		rule.ID = id
		if validateRule(rule) != nil {
			writeError(w, 400, "invalid rule")
			return
		}
		saved, err := s.repo.SaveRule(r.Context(), ownerOpenID, rule)
		if err != nil {
			handleQueryError(w, err)
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: saved})
	case http.MethodDelete:
		if err := s.repo.DeleteRule(r.Context(), ownerOpenID, id); err != nil {
			if errors.Is(err, ErrRuleInUse) {
				writeError(w, http.StatusConflict, "rule is referenced by a test scenario")
				return
			}
			handleQueryError(w, err)
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) scenariosHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := s.repo.ListScenarios(r.Context())
		if err != nil {
			writeError(w, 500, "query scenarios failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: items})
	case http.MethodPost:
		var item TestScenario
		if err := decodeJSON(r, &item); err != nil || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" {
			writeError(w, 400, "invalid scenario")
			return
		}
		prepared, err := s.repo.ValidateAndSnapshotScenario(r.Context(), ownerOpenID, item)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		saved, err := s.repo.SaveScenario(r.Context(), prepared)
		if err != nil {
			writeError(w, 500, "save scenario failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: saved})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) scenarioHandler(w http.ResponseWriter, r *http.Request) {
	ownerOpenID, ok := s.requireCurrentUser(w, r)
	if !ok {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/scenarios/"), "/")
	parts := strings.Split(path, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid scenario id")
		return
	}
	if len(parts) == 2 && parts[1] == "enabled" {
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		var request struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid enabled state")
			return
		}
		item, err := s.repo.SetScenarioEnabled(r.Context(), id, request.Enabled)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "not found")
			} else {
				writeError(w, http.StatusInternalServerError, "update scenario state failed")
			}
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: item})
		return
	}
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.repo.GetScenario(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "not found")
			} else {
				writeError(w, http.StatusInternalServerError, "query scenario failed")
			}
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: item})
	case http.MethodPut:
		var item TestScenario
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, 400, "invalid scenario")
			return
		}
		item.ID = id
		prepared, err := s.repo.ValidateAndSnapshotScenario(r.Context(), ownerOpenID, item)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		saved, err := s.repo.SaveScenario(r.Context(), prepared)
		if err != nil {
			writeError(w, 500, "save scenario failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: saved})
	case http.MethodDelete:
		if err := s.repo.DeleteScenario(r.Context(), id); err != nil {
			if err == sql.ErrNoRows {
				writeError(w, 404, "not found")
			} else {
				writeError(w, 500, "delete scenario failed")
			}
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
	default:
		methodNotAllowed(w)
	}
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(target)
}
