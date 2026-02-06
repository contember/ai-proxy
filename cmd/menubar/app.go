package main

import (
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"

	"fyne.io/systray"
)

//go:embed icons/icon_running.png
var iconRunning []byte

//go:embed icons/icon_stopped.png
var iconStopped []byte

// App holds the application state
type App struct {
	config *Config
	mu     sync.Mutex
	status ProxyStatus

	// Menu items
	mStatus        *systray.MenuItem
	mToggle        *systray.MenuItem
	mConfigureKey  *systray.MenuItem
	mTrustCert     *systray.MenuItem
	mDashboard     *systray.MenuItem
	mDefaultURL    *systray.MenuItem
	mViewLogs      *systray.MenuItem
	mQuit          *systray.MenuItem
}

// NewApp creates a new application instance
func NewApp(config *Config) *App {
	return &App{
		config: config,
		status: StatusStopped,
	}
}

// onReady is called when systray is ready
func (a *App) onReady() {
	// Set initial icon (template for macOS dark/light mode support)
	systray.SetTemplateIcon(iconStopped, iconStopped)
	systray.SetTooltip("Tudy")

	// Create menu items
	a.mStatus = systray.AddMenuItem("Status: Checking...", "Current proxy status")
	a.mStatus.Disable()

	systray.AddSeparator()

	a.mToggle = systray.AddMenuItem("Start Proxy", "Start or stop the proxy")
	a.mConfigureKey = systray.AddMenuItem("Configure API Key...", "Set your OpenRouter API key")
	a.mTrustCert = systray.AddMenuItem("Trust Certificate...", "Install Caddy's root certificate to trust HTTPS")

	systray.AddSeparator()

	a.mDashboard = systray.AddMenuItem("Open Dashboard", "Open proxy dashboard in browser")
	a.mDefaultURL = systray.AddMenuItem("Open Default URL", "Open default URL in browser")

	systray.AddSeparator()

	a.mViewLogs = systray.AddMenuItem("View Logs", "Open proxy logs in Console")

	systray.AddSeparator()

	a.mQuit = systray.AddMenuItem("Quit", "Exit the menu bar app")

	// Start status monitoring
	go a.monitorStatus()

	// Handle menu clicks
	go a.handleEvents()

	// Prompt for API key on first run if not configured
	if !a.config.HasAPIKey() {
		go a.promptFirstRunAPIKey()
	}
}

// onExit is called when the app is quitting
func (a *App) onExit() {
	// Cleanup if needed
}

// monitorStatus periodically checks the proxy status
func (a *App) monitorStatus() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Initial check
	a.updateStatus()

	for range ticker.C {
		a.updateStatus()
	}
}

// updateStatus checks and updates the proxy status
func (a *App) updateStatus() {
	newStatus := CheckProxyStatus(a.config)
	a.mu.Lock()
	changed := newStatus != a.status
	if changed {
		a.status = newStatus
	}
	a.mu.Unlock()
	if changed {
		a.refreshUI()
	}
}

// refreshUI updates the UI to reflect current status. Must be called without a.mu held.
func (a *App) refreshUI() {
	a.mu.Lock()
	status := a.status
	a.mu.Unlock()
	switch status {
	case StatusRunning:
		systray.SetTemplateIcon(iconRunning, iconRunning)
		a.mStatus.SetTitle("Status: Running")
		a.mToggle.SetTitle("Stop Proxy")
		a.mDashboard.Enable()
		a.mDefaultURL.Enable()
	case StatusStarting:
		systray.SetTemplateIcon(iconStopped, iconStopped)
		a.mStatus.SetTitle("Status: Starting...")
		a.mToggle.SetTitle("Stop Proxy")
		a.mToggle.Disable()
		a.mDashboard.Disable()
		a.mDefaultURL.Disable()
	case StatusStopping:
		systray.SetTemplateIcon(iconRunning, iconRunning)
		a.mStatus.SetTitle("Status: Stopping...")
		a.mToggle.SetTitle("Start Proxy")
		a.mToggle.Disable()
		a.mDashboard.Disable()
		a.mDefaultURL.Disable()
	default:
		systray.SetTemplateIcon(iconStopped, iconStopped)
		a.mStatus.SetTitle("Status: Stopped")
		a.mToggle.SetTitle("Start Proxy")
		a.mToggle.Enable()
		a.mDashboard.Disable()
		a.mDefaultURL.Disable()
	}
}

// handleEvents processes menu item clicks
func (a *App) handleEvents() {
	for {
		select {
		case <-a.mToggle.ClickedCh:
			go a.handleToggle()
		case <-a.mConfigureKey.ClickedCh:
			go a.handleConfigureKey()
		case <-a.mTrustCert.ClickedCh:
			go a.handleTrustCert()
		case <-a.mDashboard.ClickedCh:
			a.handleOpenDashboard()
		case <-a.mDefaultURL.ClickedCh:
			a.handleOpenDefaultURL()
		case <-a.mViewLogs.ClickedCh:
			a.handleViewLogs()
		case <-a.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// handleToggle starts or stops the proxy
func (a *App) handleToggle() {
	a.mu.Lock()
	currentStatus := a.status
	a.mu.Unlock()

	if currentStatus == StatusRunning {
		// Stop proxy
		a.mu.Lock()
		a.status = StatusStopping
		a.mu.Unlock()
		a.refreshUI()

		if err := StopProxy(a.config); err != nil {
			showAlert("Error", "Failed to stop proxy: "+err.Error(), true)
		}

		// Wait a moment then update status
		time.Sleep(1 * time.Second)
		a.updateStatus()
	} else if currentStatus == StatusStopped {
		// Check if API key is configured
		if !a.config.HasAPIKey() {
			showAlert("API Key Required",
				"Please configure your OpenRouter API key before starting the proxy.",
				true)
			return
		}

		// Start proxy
		a.mu.Lock()
		a.status = StatusStarting
		a.mu.Unlock()
		a.refreshUI()

		if err := StartProxy(a.config); err != nil {
			showAlert("Error", "Failed to start proxy: "+err.Error(), true)
			a.mu.Lock()
			a.status = StatusStopped
			a.mu.Unlock()
			a.refreshUI()
			return
		}

		// Wait for startup then update status
		time.Sleep(2 * time.Second)
		a.updateStatus()
	}
}

// handleConfigureKey shows the API key configuration dialog
func (a *App) handleConfigureKey() {
	currentKey := a.config.GetAPIKey()
	displayKey := ""
	if currentKey != "" {
		// Show masked key
		if len(currentKey) > 8 {
			displayKey = currentKey[:4] + "..." + currentKey[len(currentKey)-4:]
		} else {
			displayKey = "****"
		}
	}

	prompt := "Enter your OpenRouter API Key:"
	if displayKey != "" {
		prompt = "Current key: " + displayKey + "\nEnter new API Key (or leave empty to keep current):"
	}

	newKey, ok, err := showInputDialog("Configure API Key", prompt, "")
	if err != nil {
		showAlert("Error", "Dialog failed: "+err.Error(), true)
		return
	}

	if !ok {
		// User cancelled
		return
	}

	if newKey == "" && currentKey != "" {
		// Keep existing key
		return
	}

	if newKey == "" {
		showAlert("Invalid Key", "API key cannot be empty.", true)
		return
	}

	// Save the new key
	if err := a.config.SetAPIKey(newKey); err != nil {
		showAlert("Error", "Failed to save API key: "+err.Error(), true)
		return
	}

	showAlert("Success", "API key has been saved.", false)

	// Restart proxy if running
	a.mu.Lock()
	running := a.status == StatusRunning
	a.mu.Unlock()
	if running {
		confirmed, _ := showConfirmDialog("Restart Proxy?",
			"The proxy needs to be restarted for the new API key to take effect. Restart now?")
		if confirmed {
			if err := RestartProxy(a.config); err != nil {
				showAlert("Error", "Failed to restart proxy: "+err.Error(), true)
			}
		}
	}
}

// handleOpenDashboard opens the dashboard URL in the default browser
func (a *App) handleOpenDashboard() {
	exec.Command("open", a.config.DashboardURL).Start()
}

// handleOpenDefaultURL opens the default URL in the default browser
func (a *App) handleOpenDefaultURL() {
	exec.Command("open", a.config.DefaultURL).Start()
}

// handleViewLogs opens the log file in Console.app
func (a *App) handleViewLogs() {
	logFile := getLogFile()
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		showAlert("No Logs", "Log file not found. Start the proxy first to generate logs.", true)
		return
	}
	exec.Command("open", "-a", "Console", logFile).Start()
}

// handleTrustCert installs Caddy's root certificate to the system trust store
func (a *App) handleTrustCert() {
	if err := TrustCertificate(a.config); err != nil {
		if errors.Is(err, ErrManualTrustRequired) {
			showAlert("Manual Trust Required",
				"The certificate has been opened in Keychain Access.\n\nDouble-click the certificate, expand \"Trust\", set SSL to \"Always Trust\", then restart your browser.",
				false)
			return
		}
		showAlert("Error", "Failed to trust certificate: "+err.Error(), true)
		return
	}
	showAlert("Success", "Certificate trusted successfully.\n\nRestart your browser for the change to take effect.", false)
}

// promptFirstRunAPIKey asks the user to configure their API key on first launch
func (a *App) promptFirstRunAPIKey() {
	// Small delay so the menu bar icon appears first
	time.Sleep(500 * time.Millisecond)

	confirmed, _ := showConfirmDialog("Welcome to Tudy",
		"No API key is configured yet. Would you like to set your OpenRouter API key now?")
	if confirmed {
		a.handleConfigureKey()
	}
}

// Run starts the systray application
func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}
