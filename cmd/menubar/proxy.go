package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LogFile is where proxy logs are written
var LogFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	LogFile = filepath.Join(home, "Library", "Logs", "caddy-llm-proxy.log")
}

// ProxyStatus represents the current state of the proxy
type ProxyStatus int

const (
	StatusStopped ProxyStatus = iota
	StatusRunning
	StatusStarting
	StatusStopping
)

func (s ProxyStatus) String() string {
	switch s {
	case StatusRunning:
		return "Running"
	case StatusStarting:
		return "Starting..."
	case StatusStopping:
		return "Stopping..."
	default:
		return "Stopped"
	}
}

// CheckProxyStatus checks if the proxy is running using multiple methods
func CheckProxyStatus(config *Config) ProxyStatus {
	// Method 1: Check if the process is running
	if isProcessRunning() {
		// Method 2: Verify the HTTP endpoint responds
		if isHTTPResponding() {
			return StatusRunning
		}
		// Process running but not responding yet - might be starting
		return StatusStarting
	}
	return StatusStopped
}

// isProcessRunning checks if caddy-llm-proxy process is running
func isProcessRunning() bool {
	// Check for the actual proxy process (with "run" argument), not menubar app
	cmd := exec.Command("pgrep", "-f", "caddy-llm-proxy run")
	err := cmd.Run()
	return err == nil
}

// isHTTPResponding checks if the proxy responds to HTTP requests
func isHTTPResponding() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://127.0.0.1:80/_tls_check")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// StartProxy starts the caddy-llm-proxy with admin privileges
func StartProxy(config *Config) error {
	// Clear old logs before starting (use > instead of >> to truncate)
	// Build the shell command to run with admin privileges
	// Use set -a to auto-export all variables, source the env file, then run caddy
	shellCmd := fmt.Sprintf(
		"set -a; source '%s'; '%s' run --config '%s' > '%s' 2>&1 &",
		config.EnvFile,
		config.BinaryPath,
		config.CaddyFile,
		LogFile,
	)

	// Use osascript to get admin privileges and run in background
	script := fmt.Sprintf(
		`do shell script "%s" with administrator privileges`,
		escapeAppleScript(shellCmd),
	)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start proxy: %s: %w", string(output), err)
	}

	return nil
}

// StopProxy stops the caddy-llm-proxy
func StopProxy(config *Config) error {
	// Use osascript to kill the process with admin privileges
	// Match "caddy-llm-proxy run" to avoid killing the menubar app
	script := `do shell script "pkill -f 'caddy-llm-proxy run'; exit 0" with administrator privileges`

	cmd := exec.Command("osascript", "-e", script)
	cmd.Run() // Ignore error - we'll verify by checking if process stopped

	// Wait and verify the process actually stopped
	for i := 0; i < 10; i++ {
		if !isProcessRunning() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// If still running after 5 seconds, return error
	if isProcessRunning() {
		return fmt.Errorf("proxy did not stop within timeout")
	}

	return nil
}

// RestartProxy restarts the proxy
func RestartProxy(config *Config) error {
	if err := StopProxy(config); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	// Wait for process to fully stop
	for i := 0; i < 10; i++ {
		if !isProcessRunning() {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	return StartProxy(config)
}

// escapeAppleScript escapes a string for use in AppleScript
func escapeAppleScript(s string) string {
	// Escape backslashes and double quotes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// TrustCertificate installs Caddy's root CA certificate to the system trust store
func TrustCertificate(config *Config) error {
	// Get the path to Caddy's root certificate
	home, _ := os.UserHomeDir()
	certPath := filepath.Join(home, "Library", "Application Support", "Caddy", "pki", "authorities", "local", "root.crt")
	loginKeychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")

	// Copy the certificate to a temp location (the original is owned by root)
	tempCert := filepath.Join(os.TempDir(), "caddy-root-ca.crt")

	// Copy with admin privileges since source is owned by root
	copyScript := fmt.Sprintf(
		`do shell script "cp '%s' '%s' && chmod 644 '%s'" with administrator privileges`,
		escapeAppleScript(certPath),
		escapeAppleScript(tempCert),
		escapeAppleScript(tempCert),
	)

	cmd := exec.Command("osascript", "-e", copyScript)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("certificate not found - start the proxy first: %s", strings.TrimSpace(string(output)))
	}

	// Import to login keychain (ignore error if already exists)
	exec.Command("security", "import", tempCert, "-k", loginKeychain).Run()

	// Add as trusted root certificate to login keychain
	if output, err := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-k", loginKeychain, tempCert).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to trust certificate: %s", strings.TrimSpace(string(output)))
	}

	return nil
}
