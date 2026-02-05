package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// Set up logging to a file visible to the user (menubar apps have no stderr)
	if home, err := os.UserHomeDir(); err == nil {
		logPath := filepath.Join(home, "Library", "Logs", "caddy-llm-proxy-menubar.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(f)
		}
	}

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		showAlert("Configuration Error",
			fmt.Sprintf("Failed to load configuration: %v\n\nMake sure caddy-llm-proxy is installed.", err),
			true)
		os.Exit(1)
	}

	// Create and run the app
	app := NewApp(config)
	app.Run()
}
