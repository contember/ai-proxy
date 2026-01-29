package discovery

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LocalProcess represents a discovered local process
type LocalProcess struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Args    string `json:"args"`
	Workdir string `json:"workdir"`
}

// Processes to filter out (system/non-dev)
var ignoredCommands = map[string]bool{
	"docker-proxy":    true,
	"code":            true,
	"spotify":         true,
	"jetbrains-toolb": true,
	"phpstorm":        true,
	"webstorm":        true,
	"idea":            true,
	"goland":          true,
	"chrome":          true,
	"chromium":        true,
	"firefox":         true,
	"slack":           true,
	"discord":         true,
	"telegram":        true,
	"signal":          true,
	"zoom":            true,
	"cupsd":           true,
	"caddy":           true,
	"systemd":         true,
	"dbus-daemon":     true,
	"pulseaudio":      true,
	"pipewire":        true,
	"fsnotifier":      true,
}

// Workdirs that indicate container/system processes
var ignoredWorkdirs = map[string]bool{
	"/":     true,
	"/app":  true,
	"/srv":  true,
	"/root": true,
}

// Patterns in args that indicate non-dev processes
var ignoredArgsPatterns = []string{
	"jetbrains",
	"intellij",
	"java.rmi.server",
	"idea.home",
	"phpstorm",
	"webstorm",
	"goland",
	"rider",
	"clion",
	"datagrip",
	"rubymine",
	"pycharm",
}

// DiscoverLocalProcesses discovers locally running processes with open ports
func DiscoverLocalProcesses() ([]LocalProcess, error) {
	// Try ss first (faster), fallback to /proc parsing
	processes, err := tryWithSs()
	if err != nil || len(processes) == 0 {
		processes, err = parseFromProc()
		if err != nil {
			return nil, err
		}
	}

	// Filter out system/non-dev processes
	var filtered []LocalProcess
	for _, p := range processes {
		if ignoredCommands[p.Command] {
			continue
		}
		if p.Workdir != "" && ignoredWorkdirs[p.Workdir] {
			continue
		}
		// Check if args contain ignored patterns (case-insensitive)
		if shouldIgnoreByArgs(p.Args) {
			continue
		}
		filtered = append(filtered, p)
	}

	return filtered, nil
}

// shouldIgnoreByArgs checks if process args contain ignored patterns
func shouldIgnoreByArgs(args string) bool {
	argsLower := strings.ToLower(args)
	for _, pattern := range ignoredArgsPatterns {
		if strings.Contains(argsLower, pattern) {
			return true
		}
	}
	return false
}

// tryWithSs uses the ss command to discover processes (faster)
func tryWithSs() ([]LocalProcess, error) {
	cmd := exec.Command("ss", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var processes []LocalProcess
	lines := strings.Split(string(output), "\n")

	// Skip header line
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		localAddr := parts[3]
		processInfo := strings.Join(parts[5:], " ")

		// Extract port from local address
		portMatch := regexp.MustCompile(`:(\d+)$`).FindStringSubmatch(localAddr)
		if len(portMatch) < 2 {
			continue
		}
		port, _ := strconv.Atoi(portMatch[1])
		if port < 1024 {
			continue
		}

		// Extract PID from process info
		pidMatch := regexp.MustCompile(`pid=(\d+)`).FindStringSubmatch(processInfo)
		if len(pidMatch) < 2 {
			continue
		}

		pid, _ := strconv.Atoi(pidMatch[1])

		// Get command from /proc instead of ss output (ss shows thread name which is often unhelpful)
		command := getProcessCommand(pid)
		args := cleanArgs(getProcessArgs(pid))
		workdir := getProcessWorkdir(pid)

		processes = append(processes, LocalProcess{
			Port:    port,
			PID:     pid,
			Command: command,
			Args:    args,
			Workdir: workdir,
		})
	}

	return processes, nil
}

// parseFromProc parses /proc/net/tcp to discover processes (slower fallback)
func parseFromProc() ([]LocalProcess, error) {
	var processes []LocalProcess
	seenPorts := make(map[int]bool)

	// Build inode -> pid map first
	inodeToPid := buildInodePidMap()

	for _, tcpFile := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(tcpFile)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// Skip header
		scanner.Scan()

		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Fields(line)
			if len(parts) < 10 {
				continue
			}

			localAddr := parts[1]
			state := parts[3]

			// Only LISTEN state (0A)
			if state != "0A" {
				continue
			}

			// Extract port (hex)
			addrParts := strings.Split(localAddr, ":")
			if len(addrParts) < 2 {
				continue
			}
			port64, _ := strconv.ParseInt(addrParts[1], 16, 32)
			port := int(port64)

			if port < 1024 || seenPorts[port] {
				continue
			}
			seenPorts[port] = true

			inode := parts[9]
			pid, ok := inodeToPid[inode]
			if !ok {
				continue
			}

			command := getProcessCommand(pid)
			args := cleanArgs(getProcessArgs(pid))
			workdir := getProcessWorkdir(pid)

			processes = append(processes, LocalProcess{
				Port:    port,
				PID:     pid,
				Command: command,
				Args:    args,
				Workdir: workdir,
			})
		}
	}

	return processes, nil
}

// buildInodePidMap builds a map of socket inode -> PID
func buildInodePidMap() map[string]int {
	result := make(map[string]int)

	// Read /proc directory
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return result
	}

	socketRegex := regexp.MustCompile(`socket:\[(\d+)\]`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (all digits)
		name := entry.Name()
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}

		// Read fd directory
		fdDir := filepath.Join("/proc", name, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}

			if match := socketRegex.FindStringSubmatch(link); len(match) > 1 {
				result[match[1]] = pid
			}
		}
	}

	return result
}

// getProcessCommand gets the command name for a PID
func getProcessCommand(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// getProcessArgs gets the command line arguments for a PID
func getProcessArgs(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	// Replace null bytes with spaces
	args := strings.ReplaceAll(string(data), "\x00", " ")
	return strings.TrimSpace(args)
}

// getProcessWorkdir gets the working directory for a PID
func getProcessWorkdir(pid int) string {
	link, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
	if err != nil {
		return ""
	}
	return link
}

// cleanArgs cleans up command line args for better readability
// - Removes full paths to node_modules/.bin/, keeping just the binary name
// - Removes common interpreter paths
func cleanArgs(args string) string {
	if args == "" {
		return args
	}

	parts := strings.Split(args, " ")
	var cleaned []string

	for i, part := range parts {
		// Skip empty parts
		if part == "" {
			continue
		}

		// For the first part (command), simplify common patterns
		if i == 0 {
			// node/bun/python etc - keep as is
			if part == "node" || part == "bun" || part == "python" || part == "python3" ||
				part == "ruby" || part == "php" || part == "java" || part == "go" {
				cleaned = append(cleaned, part)
				continue
			}
			// Full path to interpreter - extract just the name
			if strings.HasPrefix(part, "/") {
				part = filepath.Base(part)
			}
		}

		// For other args, clean up node_modules/.bin paths
		if strings.Contains(part, "node_modules/.bin/") {
			// Extract just the binary name after node_modules/.bin/
			idx := strings.Index(part, "node_modules/.bin/")
			part = part[idx+len("node_modules/.bin/"):]
		} else if strings.Contains(part, "node_modules/") && strings.HasSuffix(part, "/bin/") {
			// Handle other node_modules bin patterns
			part = filepath.Base(strings.TrimSuffix(part, "/"))
		}

		cleaned = append(cleaned, part)
	}

	return strings.Join(cleaned, " ")
}
