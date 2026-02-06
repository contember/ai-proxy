package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Caddy admin API endpoint
	adminAPI = "http://localhost:2019"

	// Homebrew service name
	brewServiceName = "tudy"
)

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

// CheckProxyStatus checks if the proxy is running
func CheckProxyStatus(config *Config) ProxyStatus {
	if isAdminAPIResponding() {
		return StatusRunning
	}
	if isProcessRunning() {
		return StatusStarting
	}
	return StatusStopped
}

// isAdminAPIResponding checks if Caddy's admin API is accessible
func isAdminAPIResponding() bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get(adminAPI + "/config/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isProcessRunning checks if tudy process is running
func isProcessRunning() bool {
	cmd := exec.Command("pgrep", "-f", "tudy")
	err := cmd.Run()
	return err == nil
}

// StartProxy starts the proxy via brew services (requires one-time sudo)
func StartProxy(config *Config) error {
	if isBrewServiceAvailable() {
		return startViaBrew()
	}
	return startDirect(config)
}

// isBrewServiceAvailable checks if the brew service is installed
func isBrewServiceAvailable() bool {
	cmd := exec.Command("brew", "services", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), brewServiceName)
}

// startViaBrew starts the proxy using brew services
func startViaBrew() error {
	cleanupBrewService()

	// Try without sudo first
	cmd := exec.Command("brew", "services", "start", brewServiceName)
	if err := cmd.Run(); err == nil {
		return waitForStart()
	}

	// Need sudo - use osascript for GUI prompt
	script := fmt.Sprintf(
		`do shell script "launchctl bootout system/homebrew.mxcl.%s 2>/dev/null; rm -f /Library/LaunchDaemons/homebrew.mxcl.%s.plist 2>/dev/null; brew services start %s 2>/dev/null; exit 0" with administrator privileges`,
		brewServiceName, brewServiceName, brewServiceName,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(ctx, "osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timed out waiting for admin privileges")
		}
		if err := waitForStart(); err == nil {
			return nil
		}
		return fmt.Errorf("failed to start service (try: sudo brew services start %s)", brewServiceName)
	}
	return waitForStart()
}

// cleanupBrewService removes any broken service state
func cleanupBrewService() {
	uid := os.Getuid()
	exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/homebrew.mxcl.%s", uid, brewServiceName)).Run()

	home, _ := os.UserHomeDir()
	userPlist := filepath.Join(home, "Library", "LaunchAgents", "homebrew.mxcl."+brewServiceName+".plist")
	os.Remove(userPlist)

	exec.Command("brew", "services", "stop", brewServiceName).Run()
}

// waitForStart waits for the proxy to become responsive
func waitForStart() error {
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if isAdminAPIResponding() || isProcessRunning() {
			return nil
		}
	}
	return fmt.Errorf("proxy did not start within timeout")
}

// startDirect starts the proxy directly (fallback if brew not available)
func startDirect(config *Config) error {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, "Library", "Logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, "tudy.log")

	shellCmd := fmt.Sprintf(
		"set -a; source '%s'; '%s' run --config '%s' >> '%s' 2>&1 &",
		config.EnvFile,
		config.BinaryPath,
		config.CaddyFile,
		logFile,
	)

	script := fmt.Sprintf(
		`do shell script "%s" with administrator privileges`,
		escapeAppleScript(shellCmd),
	)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start proxy: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// StopProxy stops the proxy via Caddy admin API (no sudo required!)
func StopProxy(config *Config) error {
	if isAdminAPIResponding() {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		req, err := http.NewRequest("POST", adminAPI+"/stop", nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return stopViaBrewOrKill()
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			for i := 0; i < 20; i++ {
				if !isProcessRunning() {
					return nil
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	}

	return stopViaBrewOrKill()
}

// stopViaBrewOrKill stops the proxy via brew services or direct kill
func stopViaBrewOrKill() error {
	if isBrewServiceAvailable() {
		cmd := exec.Command("brew", "services", "stop", brewServiceName)
		if err := cmd.Run(); err == nil {
			for i := 0; i < 10; i++ {
				if !isProcessRunning() {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	if isProcessRunning() {
		exec.Command("pkill", "-f", "tudy").Run()
		time.Sleep(500 * time.Millisecond)

		if isProcessRunning() {
			script := `do shell script "pkill -f tudy; exit 0" with administrator privileges`
			exec.Command("osascript", "-e", script).Run()
		}
	}

	for i := 0; i < 10; i++ {
		if !isProcessRunning() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	if isProcessRunning() {
		return fmt.Errorf("proxy did not stop within timeout")
	}
	return nil
}

// RestartProxy restarts the proxy
func RestartProxy(config *Config) error {
	if isAdminAPIResponding() {
		caddyfileContent, err := os.ReadFile(config.CaddyFile)
		if err == nil {
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			req, err := http.NewRequest("POST", adminAPI+"/load",
				bytes.NewReader(caddyfileContent))
			if err == nil {
				req.Header.Set("Content-Type", "text/caddyfile")
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return nil
					}
					body, _ := io.ReadAll(resp.Body)
					return fmt.Errorf("reload failed: %s", string(body))
				}
			}
		}
	}

	if err := StopProxy(config); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

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
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// getLogFile returns the current log file path, checking locations each time
func getLogFile() string {
	home, _ := os.UserHomeDir()
	locations := []string{
		filepath.Join(home, "Library", "Logs", "Homebrew", "tudy.log"),
		filepath.Join(home, "Library", "Logs", "tudy.log"),
		"/var/log/tudy.log",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	// Default fallback (file may not exist yet)
	return filepath.Join(home, "Library", "Logs", "tudy.log")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}
