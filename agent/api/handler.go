package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
)

// 请求/响应结构体

type connectRequest struct {
	Device    string `json:"device"`
	BaudRate  int    `json:"baud_rate"`
	DataBits  int    `json:"data_bits"`
	StopBits  int    `json:"stop_bits"`
	Parity    string `json:"parity"`
}

type sendRequest struct {
	Data string `json:"data"`
	Mode string `json:"mode"` // "str" 或 "hex"
}

type apiResponse struct {
	OK    bool        `json:"ok"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// handlePorts 返回可用串口列表。
func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "仅支持 GET 请求"})
		return
	}

	ports, err := s.proxy.ListPorts()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: ports})
}

// handleConnect 连接串口。
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "仅支持 POST 请求"})
		return
	}

	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "无效的请求体"})
		return
	}

	if req.Device == "" {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "串口设备名不能为空"})
		return
	}
	if req.BaudRate == 0 {
		req.BaudRate = 115200
	}
	if req.DataBits == 0 {
		req.DataBits = 8
	}
	if req.StopBits == 0 {
		req.StopBits = 1
	}
	if req.Parity == "" {
		req.Parity = "none"
	}

	if err := s.proxy.Connect(req.Device, req.BaudRate, req.DataBits, req.StopBits, req.Parity); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: s.proxy.Status()})
}

// handleDisconnect 断开串口。
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "仅支持 POST 请求"})
		return
	}

	if err := s.proxy.Disconnect(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{OK: true})
}

// handleSend 发送数据。
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "仅支持 POST 请求"})
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "无效的请求体"})
		return
	}

	var data []byte
	var err error

	switch req.Mode {
	case "hex":
		data, err = hex.DecodeString(req.Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{OK: false, Error: "无效的 HEX 数据"})
			return
		}
	default:
		data = []byte(req.Data)
	}

	n, err := s.proxy.Send(data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{OK: false, Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: map[string]int{"sent": n}})
}

// handleStatus 返回连接状态。
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiResponse{OK: false, Error: "仅支持 GET 请求"})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: s.proxy.Status()})
}

// handleUI 返回嵌入式 UI 页面。
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := s.uiFS.ReadFile("ui/index.html")
	if err != nil {
		http.Error(w, "UI 页面未找到", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}