package main

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

// sourceEnvFile reads the env file and sets environment variables
func sourceEnvFile(envFile string) {
	file, err := os.Open(envFile)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")
			os.Setenv(key, value)
		}
	}
}

// delegateToCaddy replaces the current process with the caddy binary
func delegateToCaddy(config *Config, args []string) error {
	// Source the env file
	sourceEnvFile(config.EnvFile)

	// Set CADDY_DATA_DIR if not already set
	if os.Getenv("CADDY_DATA_DIR") == "" {
		// Determine var dir based on config dir location
		var varDir string
		if strings.HasPrefix(config.ConfigDir, "/opt/homebrew") {
			varDir = "/opt/homebrew/var/lib/caddy-llm-proxy"
		} else if strings.HasPrefix(config.ConfigDir, "/usr/local") {
			varDir = "/usr/local/var/lib/caddy-llm-proxy"
		}
		if varDir != "" {
			os.Setenv("CADDY_DATA_DIR", varDir)
		}
	}

	// Build argv: binary path + remaining args
	argv := append([]string{config.BinaryPath}, args...)

	// Replace current process with caddy binary
	return syscall.Exec(config.BinaryPath, argv, os.Environ())
}
