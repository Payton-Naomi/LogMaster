package main

import (
	"flag"
	"fmt"
	"os"

	"logmaster-agent/agent"
	"logmaster-agent/agent/config"
)

func main() {
	configPath := flag.String("config", "agent/config.yaml", "配置文件路径")
	serverPort := flag.Int("port", 9527, "HTTP 服务端口")
	noBrowser := flag.Bool("no-browser", false, "不自动打开浏览器")
	noUI := flag.Bool("no-ui", false, "纯命令行模式（不启动 UI）")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	ag := agent.New(cfg)

	if *noUI {
		// 纯命令行模式
		if err := ag.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Agent 错误: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 启动 HTTP 服务（REST API + WebSocket + 嵌入式 UI）
	fmt.Fprintf(os.Stderr, "LogMaster Agent 启动中...\n")
	if err := ag.ServeUI(*serverPort, *noBrowser); err != nil {
		fmt.Fprintf(os.Stderr, "服务启动失败: %v\n", err)
		os.Exit(1)
	}
}