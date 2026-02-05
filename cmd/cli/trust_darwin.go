//go:build darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
		if !strings.Contains(string(output), "already exists") {
			fmt.Fprintf(os.Stderr, "warning: certificate import failed: %s\n", strings.TrimSpace(string(output)))
		}
	}

	// Check if certificate is already trusted
	if isCertTrusted() {
		return nil
	}

	// Try to add as trusted root certificate with SSL policy
	if output, err := exec.Command("security", "add-trusted-cert", "-r", "trustRoot", "-p", "ssl", "-k", loginKeychain, tempCert).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: add-trusted-cert failed: %s\n", strings.TrimSpace(string(output)))
	}

	// Verify trust was set
	if isCertTrusted() {
		return nil
	}

	// Trust settings not properly set - open certificate for manual trust via macOS UI
	exec.Command("open", tempCert).Run()
	time.Sleep(2 * time.Second)
	return fmt.Errorf("certificate opened in Keychain Access - set 'Always Trust' for SSL, then restart your browser")
}

// isCertTrusted checks if the Caddy root CA has SSL trust settings configured
func isCertTrusted() bool {
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
			if strings.Contains(lower, "number of trust settings : 0") {
				inCaddyCert = false
				continue
			}
			if strings.Contains(lower, "policy oid") && strings.Contains(lower, "ssl") {
				return true
			}
			if strings.HasPrefix(strings.TrimSpace(line), "Cert ") {
				inCaddyCert = false
			}
		}
	}
	return false
}

// isCertTrustedCheck returns whether the certificate is already trusted (for setup flow)
func isCertTrustedCheck() bool {
	return isCertTrusted()
}

// copyFileWithAdmin copies a file using admin privileges via osascript
func copyFileWithAdmin(src, dst string) error {
	// Try direct copy first
	if err := copyFile(src, dst); err == nil {
		return nil
	}

	script := fmt.Sprintf(`do shell script "cp %q %q" with administrator privileges`, src, dst)
	cmd := exec.Command("osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy file: %s: %w", string(output), err)
	}
	return nil
}
