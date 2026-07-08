package api

import (
	"embed"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"logmaster-agent/agent/proxy"
)

// Server 是 Agent 的 HTTP 服务，提供 REST API、WebSocket 和嵌入式 UI。
type Server struct {
	proxy  *proxy.Proxy
	uiFS   embed.FS
	router *http.ServeMux
}

// New 创建一个新的 HTTP 服务实例。
// uiFS 是嵌入的前端静态文件系统。
func New(p *proxy.Proxy, uiFS embed.FS) *Server {
	s := &Server{
		proxy:  p,
		uiFS:   uiFS,
		router: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Start 启动 HTTP 服务。
func (s *Server) Start(addr string) error {
	fmt.Printf("串口调试工具已启动: http://%s\n", addr)
	return http.ListenAndServe(addr, s.router)
}

// OpenBrowser 在默认浏览器中打开指定 URL。
func OpenBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}
	_ = exec.Command(cmd, args...).Start()
}

func (s *Server) registerRoutes() {
	// REST API
	s.router.HandleFunc("/api/ports", s.handlePorts)
	s.router.HandleFunc("/api/connect", s.handleConnect)
	s.router.HandleFunc("/api/disconnect", s.handleDisconnect)
	s.router.HandleFunc("/api/send", s.handleSend)
	s.router.HandleFunc("/api/status", s.handleStatus)

	// WebSocket
	s.router.HandleFunc("/ws", s.handleWebSocket)

	// 嵌入式 UI
	s.router.HandleFunc("/", s.handleUI)
}