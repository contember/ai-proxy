package main

import (
	"fmt"
	"os"
)

const usage = `Caddy LLM Proxy - AI-powered local development proxy

Usage:
  caddy-llm-proxy <command> [args...]

Commands:
  setup       Interactive first-time setup (API key, certificate, start)
  status      Show proxy status
  start       Start the proxy
  stop        Stop the proxy
  restart     Restart the proxy
  trust       Trust the HTTPS certificate

All other commands (run, version, etc.) are passed through to Caddy.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "help", "--help", "-h":
		fmt.Print(usage)
		os.Exit(0)

	case "setup":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			printError("Make sure caddy-llm-proxy is installed via Homebrew.")
			os.Exit(1)
		}
		os.Exit(runSetup(config))

	case "status":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		printStatus(config)

	case "start":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		status := CheckProxyStatus(config)
		if status == StatusRunning {
			fmt.Println("Proxy is already running.")
			os.Exit(0)
		}
		fmt.Print("Starting proxy... ")
		if err := StartProxy(config); err != nil {
			fmt.Println()
			printError(fmt.Sprintf("Failed to start proxy: %v", err))
			os.Exit(1)
		}
		fmt.Println("done")

	case "stop":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		status := CheckProxyStatus(config)
		if status == StatusStopped {
			fmt.Println("Proxy is already stopped.")
			os.Exit(0)
		}
		fmt.Print("Stopping proxy... ")
		if err := StopProxy(config); err != nil {
			fmt.Println()
			printError(fmt.Sprintf("Failed to stop proxy: %v", err))
			os.Exit(1)
		}
		fmt.Println("done")

	case "restart":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		fmt.Print("Restarting proxy... ")
		if err := RestartProxy(config); err != nil {
			fmt.Println()
			printError(fmt.Sprintf("Failed to restart proxy: %v", err))
			os.Exit(1)
		}
		fmt.Println("done")

	case "trust":
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		fmt.Print("Trusting certificate... ")
		if err := TrustCertificate(config); err != nil {
			fmt.Println()
			printWarning(err.Error())
			os.Exit(1)
		}
		fmt.Println("done")

	default:
		// Delegate everything else to the caddy binary
		config, err := LoadConfig()
		if err != nil {
			printError(fmt.Sprintf("Failed to load configuration: %v", err))
			os.Exit(1)
		}
		if err := delegateToCaddy(config, os.Args[1:]); err != nil {
			printError(fmt.Sprintf("Failed to exec caddy: %v", err))
			os.Exit(1)
		}
	}
}
