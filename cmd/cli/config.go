package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration
type Config struct {
	ConfigDir    string
	EnvFile      string
	CaddyFile    string
	BinaryPath   string
	DefaultURL   string
	DashboardURL string
}

// Possible installation paths (Homebrew ARM, Homebrew Intel, manual)
var configPaths = []string{
	"/opt/homebrew/etc/tudy",
	"/usr/local/etc/tudy",
}

var binaryPaths = []string{
	"/opt/homebrew/bin/tudy-bin",
	"/usr/local/bin/tudy-bin",
}

// LoadConfig detects the installation and loads configuration
func LoadConfig() (*Config, error) {
	config := &Config{
		DashboardURL: "https://proxy.localhost",
		DefaultURL:   "https://proxy.localhost",
	}

	// Find config directory
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			config.ConfigDir = path
			config.EnvFile = filepath.Join(path, "env")
			config.CaddyFile = filepath.Join(path, "Caddyfile")
			break
		}
	}

	if config.ConfigDir == "" {
		return nil, fmt.Errorf("tudy config not found in %v", configPaths)
	}

	// Find binary
	for _, path := range binaryPaths {
		if _, err := os.Stat(path); err == nil {
			config.BinaryPath = path
			break
		}
	}

	if config.BinaryPath == "" {
		return nil, fmt.Errorf("tudy-bin binary not found in %v", binaryPaths)
	}

	// Load default URL from env if present
	if url := config.GetEnvValue("DEFAULT_URL"); url != "" {
		config.DefaultURL = url
	}

	return config, nil
}

// GetEnvValue reads a specific value from the env file
func (c *Config) GetEnvValue(key string) string {
	file, err := os.Open(c.EnvFile)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'")
			return value
		}
	}
	return ""
}

// SetEnvValue updates or adds a value in the env file
func (c *Config) SetEnvValue(key, value string) error {
	// Read existing content
	content, err := os.ReadFile(c.EnvFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	newLines := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
			found = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
	}

	// Write back - may need admin privileges for Homebrew-owned directories
	tempFile, err := os.CreateTemp("", "env-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(strings.Join(newLines, "\n")); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	return copyFileWithAdmin(tempPath, c.EnvFile)
}

// HasAPIKey checks if an API key is configured
func (c *Config) HasAPIKey() bool {
	key := c.GetEnvValue("LLM_API_KEY")
	return key != ""
}

// GetAPIKey returns the configured API key
func (c *Config) GetAPIKey() string {
	return c.GetEnvValue("LLM_API_KEY")
}

// SetAPIKey updates the API key in the env file
func (c *Config) SetAPIKey(key string) error {
	return c.SetEnvValue("LLM_API_KEY", key)
}
