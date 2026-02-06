package main

import (
	"fmt"
	"os"
)

// runSetup runs the interactive setup flow
func runSetup(config *Config) int {
	printHeader("Tudy Setup")

	// Step 1: Configure API Key
	printStep(1, 3, "Configure API Key")

	currentKey := config.GetAPIKey()
	if currentKey != "" {
		masked := currentKey[:4] + "..." + currentKey[len(currentKey)-4:]
		printDim(fmt.Sprintf("  Current key: %s", masked))
		if !promptYesNo("Update API key?", false) {
			printOK("API key unchanged")
		} else {
			if !configureAPIKey(config) {
				return 1
			}
		}
	} else {
		if !configureAPIKey(config) {
			return 1
		}
	}
	fmt.Println()

	// Step 2: Trust HTTPS Certificate
	printStep(2, 3, "Trust HTTPS Certificate")

	if isCertAlreadyTrusted() {
		printOK("Certificate already trusted")
	} else {
		// Need to start proxy first to generate certificate if not running
		status := CheckProxyStatus(config)
		if status != StatusRunning {
			printDim("  Starting proxy to generate certificate...")
			if err := StartProxy(config); err != nil {
				printWarning(fmt.Sprintf("Could not start proxy: %v", err))
				printWarning("You can trust the certificate later with: tudy trust")
			} else {
				printDim("  Trusting certificate...")
				if err := TrustCertificate(config); err != nil {
					printWarning(fmt.Sprintf("Certificate trust: %v", err))
				} else {
					printOK("Certificate trusted")
				}
			}
		} else {
			printDim("  Trusting certificate...")
			if err := TrustCertificate(config); err != nil {
				printWarning(fmt.Sprintf("Certificate trust: %v", err))
			} else {
				printOK("Certificate trusted")
			}
		}
	}
	fmt.Println()

	// Step 3: Start Proxy
	printStep(3, 3, "Start Proxy")

	status := CheckProxyStatus(config)
	if status == StatusRunning {
		// Restart to pick up new config
		printDim("  Restarting proxy...")
		if err := RestartProxy(config); err != nil {
			printError(fmt.Sprintf("Failed to restart proxy: %v", err))
			return 1
		}
		printOK("Proxy restarted")
	} else {
		printDim("  Starting proxy...")
		if err := StartProxy(config); err != nil {
			printError(fmt.Sprintf("Failed to start proxy: %v", err))
			return 1
		}
		printOK("Proxy started")
	}

	// Print summary
	fmt.Println()
	printHeader("Setup Complete!")
	fmt.Printf("  Dashboard: %s\n", config.DashboardURL)
	fmt.Printf("  Config:    %s\n", config.ConfigDir)
	fmt.Println()
	fmt.Println("Try it out:")
	fmt.Println("  curl https://myapp.localhost")
	fmt.Println()

	return 0
}

// configureAPIKey prompts for and saves an API key
func configureAPIKey(config *Config) bool {
	key := promptString("Enter your OpenRouter API key", "")
	if key == "" {
		printError("API key is required")
		return false
	}

	if err := config.SetAPIKey(key); err != nil {
		printError(fmt.Sprintf("Failed to save API key: %v", err))
		return false
	}

	printOK("API key saved")
	return true
}

// isCertAlreadyTrusted checks if the certificate is already trusted
// This is a platform-specific check
func isCertAlreadyTrusted() bool {
	// On non-darwin, isCertTrusted is not available, so we check if the function exists
	// via the build-tagged trust files. For simplicity, we just call the platform function.
	return isCertTrustedCheck()
}

// printStatus prints the current proxy status
func printStatus(config *Config) {
	status := CheckProxyStatus(config)

	switch status {
	case StatusRunning:
		fmt.Printf("%s●%s Proxy is running\n", colorGreen, colorReset)
	case StatusStarting:
		fmt.Printf("%s●%s Proxy is starting...\n", colorYellow, colorReset)
	default:
		fmt.Printf("%s●%s Proxy is stopped\n", colorRed, colorReset)
	}

	fmt.Printf("  Dashboard: %s\n", config.DashboardURL)
	fmt.Printf("  Config:    %s\n", config.ConfigDir)

	if config.HasAPIKey() {
		key := config.GetAPIKey()
		masked := key[:4] + "..." + key[len(key)-4:]
		fmt.Printf("  API Key:   %s\n", masked)
	} else {
		fmt.Printf("  API Key:   %s(not configured)%s\n", colorRed, colorReset)
	}

	// Show log file location
	logFile := getLogFile()
	if _, err := os.Stat(logFile); err == nil {
		fmt.Printf("  Logs:      %s\n", logFile)
	}
}
