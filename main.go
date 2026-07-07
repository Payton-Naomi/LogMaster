package main

import (
	"flag"
	"fmt"
	"os"

	"logmaster-agent/agent"
	"logmaster-agent/agent/config"
)

func main() {
	configPath := flag.String("config", "agent/config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	ag := agent.New(cfg)
	if err := ag.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}
}