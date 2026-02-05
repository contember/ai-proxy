package main

import (
	"fmt"
	"os"
)

func main() {
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
