package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// showInputDialog shows a native macOS input dialog and returns the entered text
func showInputDialog(title, prompt, defaultValue string) (string, bool, error) {
	script := fmt.Sprintf(`
		set dialogResult to display dialog %q default answer %q with title %q buttons {"Cancel", "OK"} default button "OK"
		if button returned of dialogResult is "OK" then
			return text returned of dialogResult
		else
			return ""
		end if
	`, prompt, defaultValue, title)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		// User clicked Cancel
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("dialog failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	return result, true, nil
}

// showAlert shows a native macOS alert dialog
func showAlert(title, message string, isError bool) error {
	iconType := "note"
	if isError {
		iconType = "stop"
	}

	script := fmt.Sprintf(`
		display dialog %q with title %q buttons {"OK"} default button "OK" with icon %s
	`, message, title, iconType)

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// showConfirmDialog shows a native macOS confirmation dialog
func showConfirmDialog(title, message string) (bool, error) {
	script := fmt.Sprintf(`
		set dialogResult to display dialog %q with title %q buttons {"Cancel", "OK"} default button "OK"
		if button returned of dialogResult is "OK" then
			return "true"
		else
			return "false"
		end if
	`, message, title)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		// User clicked Cancel
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("dialog failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	return result == "true", nil
}

// copyFileWithAdmin copies a file using admin privileges
func copyFileWithAdmin(src, dst string) error {
	script := fmt.Sprintf(`do shell script "cp %q %q" with administrator privileges`, src, dst)
	cmd := exec.Command("osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy file: %s: %w", string(output), err)
	}
	return nil
}
