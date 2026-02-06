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

// ensurePath adds common binary directories to PATH.
// This is needed when running as a launchd service (e.g. via brew services)
// where PATH is minimal (/usr/bin:/bin:/usr/sbin:/sbin) and tools like
// docker, lsof, etc. may not be found.
func ensurePath() {
	extraPaths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}
	current := os.Getenv("PATH")
	existing := make(map[string]bool)
	for _, p := range strings.Split(current, ":") {
		existing[p] = true
	}
	var toAdd []string
	for _, p := range extraPaths {
		if !existing[p] {
			toAdd = append(toAdd, p)
		}
	}
	if len(toAdd) > 0 {
		os.Setenv("PATH", current+":"+strings.Join(toAdd, ":"))
	}
}

// delegateToCaddy replaces the current process with the caddy binary
func delegateToCaddy(config *Config, args []string) error {
	// Source the env file
	sourceEnvFile(config.EnvFile)

	// Ensure PATH includes common binary directories for launchd services
	ensurePath()

	// Set CADDY_DATA_DIR if not already set
	if os.Getenv("CADDY_DATA_DIR") == "" {
		// Determine var dir based on config dir location
		var varDir string
		if strings.HasPrefix(config.ConfigDir, "/opt/homebrew") {
			varDir = "/opt/homebrew/var/lib/tudy"
		} else if strings.HasPrefix(config.ConfigDir, "/usr/local") {
			varDir = "/usr/local/var/lib/tudy"
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
