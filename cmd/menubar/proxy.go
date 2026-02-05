package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
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
	brewServiceName = "caddy-llm-proxy"
)

// LogFile is where proxy logs are written (when using brew services)
var LogFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	// Brew services log location
	LogFile = filepath.Join(home, "Library", "Logs", "Homebrew", "caddy-llm-proxy.log")

	// Fall back to our custom location if brew logs don't exist
	if _, err := os.Stat(LogFile); os.IsNotExist(err) {
		LogFile = filepath.Join(home, "Library", "Logs", "caddy-llm-proxy.log")
	}
}

// ErrManualTrustRequired is returned when the certificate was opened in Keychain Access
// for manual trust configuration. This is not a failure — it just needs user action.
var ErrManualTrustRequired = fmt.Errorf("certificate opened in Keychain Access - set 'Always Trust' for SSL, then restart your browser")

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
	// Check if Caddy admin API responds (most reliable)
	if isAdminAPIResponding() {
		return StatusRunning
	}

	// Fallback: check if process is running
	if isProcessRunning() {
		// Process running but admin API not responding - might be starting
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

// isProcessRunning checks if caddy-llm-proxy process is running
func isProcessRunning() bool {
	cmd := exec.Command("pgrep", "-f", "caddy-llm-proxy")
	err := cmd.Run()
	return err == nil
}

// StartProxy starts the proxy via brew services (requires one-time sudo)
func StartProxy(config *Config) error {
	// Check if brew services can be used
	if isBrewServiceAvailable() {
		return startViaBrew()
	}

	// Fallback to direct start with admin privileges
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
	// First, clean up any broken service state
	cleanupBrewService()

	// Try without sudo first (works if already authenticated or user has permissions)
	cmd := exec.Command("brew", "services", "start", brewServiceName)
	if err := cmd.Run(); err == nil {
		return waitForStart()
	}

	// Need sudo - use osascript for GUI prompt
	// First cleanup any broken system state, then start
	// Redirect stderr to suppress brew warnings
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
		// Check if it actually started despite the error
		if err := waitForStart(); err == nil {
			return nil
		}
		return fmt.Errorf("failed to start service (try: sudo brew services start %s)", brewServiceName)
	}
	return waitForStart()
}

// cleanupBrewService removes any broken service state
func cleanupBrewService() {
	// Unload from launchctl if stuck
	uid := os.Getuid()
	exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/homebrew.mxcl.%s", uid, brewServiceName)).Run()

	// Remove user-level plist if exists
	home, _ := os.UserHomeDir()
	userPlist := filepath.Join(home, "Library", "LaunchAgents", "homebrew.mxcl."+brewServiceName+".plist")
	os.Remove(userPlist)

	// Try to stop via brew (ignore errors)
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
	logFile := filepath.Join(home, "Library", "Logs", "caddy-llm-proxy.log")

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
	// Try graceful stop via admin API first
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
			// API not responding, try other methods
			return stopViaBrewOrKill()
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Wait for process to exit
			for i := 0; i < 20; i++ {
				if !isProcessRunning() {
					return nil
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
	}

	// Fallback to brew services stop or kill
	return stopViaBrewOrKill()
}

// stopViaBrewOrKill stops the proxy via brew services or direct kill
func stopViaBrewOrKill() error {
	// Try brew services stop (doesn't require sudo for stop)
	if isBrewServiceAvailable() {
		cmd := exec.Command("brew", "services", "stop", brewServiceName)
		if err := cmd.Run(); err == nil {
			// Wait for stop
			for i := 0; i < 10; i++ {
				if !isProcessRunning() {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	// Last resort: kill the process (may need sudo)
	if isProcessRunning() {
		// Try regular kill first
		exec.Command("pkill", "-f", "caddy-llm-proxy").Run()
		time.Sleep(500 * time.Millisecond)

		if isProcessRunning() {
			// Need sudo
			script := `do shell script "pkill -f caddy-llm-proxy; exit 0" with administrator privileges`
			exec.Command("osascript", "-e", script).Run()
		}
	}

	// Verify stopped
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
	// If admin API is available, we can do a graceful reload
	if isAdminAPIResponding() {
		// Read the Caddyfile and reload via API
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
					// Read error message
					body, _ := io.ReadAll(resp.Body)
					return fmt.Errorf("reload failed: %s", string(body))
				}
			}
		}
	}

	// Fallback to stop + start
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
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// TrustCertificate installs Caddy's root CA certificate to the system trust store
func TrustCertificate(config *Config) error {
	home, _ := os.UserHomeDir()

	// Try multiple possible certificate locations
	certPaths := []string{
		filepath.Join(home, "Library", "Application Support", "Caddy", "pki", "authorities", "local", "root.crt"),
		"/opt/homebrew/var/lib/caddy-llm-proxy/pki/authorities/local/root.crt",
		"/usr/local/var/lib/caddy-llm-proxy/pki/authorities/local/root.crt",
		"/var/lib/caddy-llm-proxy/pki/authorities/local/root.crt",
	}

	loginKeychain := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	tempCert := filepath.Join(os.TempDir(), "caddy-root-ca.crt")

	// Try to copy certificate from each path (may need admin privileges for root-owned files)
	var copyErr error
	for _, certPath := range certPaths {
		// First try without admin privileges
		if err := copyFile(certPath, tempCert); err == nil {
			copyErr = nil
			break
		}

		// Try with admin privileges (handles permission denied on root-owned directories)
		copyScript := fmt.Sprintf(
			`do shell script "test -f %q && cp %q %q && chmod 644 %q" with administrator privileges`,
			certPath, certPath, tempCert, tempCert,
		)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, "osascript", "-e", copyScript)
		_, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			copyErr = nil
			break
		}
		copyErr = fmt.Errorf("certificate not found at %s", certPath)
	}

	if copyErr != nil {
		return fmt.Errorf("certificate not found - start the proxy first and make an HTTPS request")
	}

	// Clean up temp file when done
	defer os.Remove(tempCert)

	// Import to login keychain
	if output, err := exec.Command("security", "import", tempCert, "-k", loginKeychain).CombinedOutput(); err != nil {
		// "already exists" is not a real error
		if !strings.Contains(string(output), "already exists") {
			log.Printf("warning: certificate import failed: %s", strings.TrimSpace(string(output)))
		}
	}

	// Check if certificate is already trusted
	if isCertTrusted() {
		return nil
	}

	// Try to add as trusted root certificate with SSL policy
	if output, err := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-p", "ssl", "-k", loginKeychain, tempCert).CombinedOutput(); err != nil {
		log.Printf("warning: add-trusted-cert failed: %s", strings.TrimSpace(string(output)))
	}

	// Verify trust was set
	if isCertTrusted() {
		return nil
	}

	// Trust settings not properly set - open certificate for manual trust via macOS UI
	// This opens Keychain Access which allows the user to set trust properly.
	// Give Keychain Access time to read the file before the deferred os.Remove cleans it up.
	exec.Command("open", tempCert).Run()
	time.Sleep(2 * time.Second)
	return ErrManualTrustRequired
}

// isCertTrusted checks if the Caddy root CA has SSL trust settings configured
func isCertTrusted() bool {
	// Check both user and admin trust settings domains
	for _, domain := range []string{"", "-d"} {
		args := []string{"dump-trust-settings"}
		if domain != "" {
			args = append(args, domain)
		}
		output, err := exec.Command("security", args...).CombinedOutput()
		if err != nil {
			continue
		}
		if checkTrustOutput(string(output)) {
			return true
		}
	}
	return false
}

// checkTrustOutput parses security dump-trust-settings output for Caddy cert trust
func checkTrustOutput(output string) bool {
	lines := strings.Split(output, "\n")
	inCaddyCert := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "caddy local authority") {
			inCaddyCert = true
			continue
		}
		if inCaddyCert {
			// Check if this cert has trust settings
			if strings.Contains(lower, "number of trust settings : 0") {
				inCaddyCert = false
				continue
			}
			if strings.Contains(lower, "policy oid") && strings.Contains(lower, "ssl") {
				return true
			}
			// New certificate section starts
			if strings.HasPrefix(strings.TrimSpace(line), "Cert ") {
				inCaddyCert = false
			}
		}
	}
	return false
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}
