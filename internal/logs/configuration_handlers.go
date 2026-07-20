package logs

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	serial "go.bug.st/serial"

	"logmaster-agent/internal/response"
)

func (s *Service) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days != 30 {
		days = 7
	}
	stats, err := s.repo.Dashboard(r.Context(), days)
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
	projects, err := s.repo.Projects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query projects failed")
		return
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: projects})
}

func (s *Service) comPortsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	ports, err := serial.GetPortsList()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "enumerate serial ports failed")
		return
	}
	if ports == nil {
		ports = []string{}
	}
	response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: ports})
}

func (s *Service) rulesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules, err := s.repo.ListRules(r.Context())
		if err != nil {
			writeError(w, 500, "query rules failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: rules})
	case http.MethodPost:
		var rule ParseRule
		if err := decodeJSON(r, &rule); err != nil || validateRule(rule) != nil {
			writeError(w, 400, "invalid rule")
			return
		}
		saved, err := s.repo.SaveRule(r.Context(), rule)
		if err != nil {
			writeError(w, 500, "save rule failed")
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: saved})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) ruleHandler(w http.ResponseWriter, r *http.Request) {
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
		saved, err := s.repo.SaveRule(r.Context(), rule)
		if err != nil {
			handleQueryError(w, err)
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: saved})
	case http.MethodDelete:
		if err := s.repo.DeleteRule(r.Context(), id); err != nil {
			handleQueryError(w, err)
			return
		}
		response.JSON(w, response.APIResponse{Code: 0, Message: "success", Data: nil})
	default:
		methodNotAllowed(w)
	}
}

func (s *Service) scenariosHandler(w http.ResponseWriter, r *http.Request) {
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
		saved, err := s.repo.SaveScenario(r.Context(), item)
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
	id := lastPathPart(r.URL.Path)
	switch r.Method {
	case http.MethodPut:
		var item TestScenario
		if err := decodeJSON(r, &item); err != nil {
			writeError(w, 400, "invalid scenario")
			return
		}
		item.ID = id
		saved, err := s.repo.SaveScenario(r.Context(), item)
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
