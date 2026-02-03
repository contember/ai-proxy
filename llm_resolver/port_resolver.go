package llm_resolver

import (
	"fmt"
	"regexp"
	"strings"
)

// ResolveProcessPort finds the current port for a process identified by ProcessIdentifier.
// It uses the ProcessCache to get current processes and matches against the identifier.
func ResolveProcessPort(identifier *ProcessIdentifier, cache *ProcessCache) (int, error) {
	if identifier == nil || identifier.Workdir == "" {
		return 0, fmt.Errorf("process identifier with workdir is required")
	}

	processes, err := cache.Get()
	if err != nil {
		return 0, fmt.Errorf("failed to get processes: %w", err)
	}

	var candidates []LocalProcess

	// Filter by workdir (prefix match)
	for _, proc := range processes {
		if !matchesWorkdir(proc.Workdir, identifier.Workdir) {
			continue
		}

		// If CommandPattern is specified, filter further
		if identifier.CommandPattern != "" {
			if !matchesCommand(proc, identifier.CommandPattern) {
				continue
			}
		}

		candidates = append(candidates, proc)
	}

	if len(candidates) == 0 {
		return 0, fmt.Errorf("no process found matching workdir %q", identifier.Workdir)
	}

	// If multiple candidates, prefer the one with the lowest port
	// (common pattern: Vite uses lower ports for main dev server)
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Port < best.Port {
			best = c
		}
	}

	return best.Port, nil
}

// matchesWorkdir checks if a process workdir matches the target workdir.
// Uses prefix matching so /home/user/project matches /home/user/project/frontend
func matchesWorkdir(processWorkdir, targetWorkdir string) bool {
	if processWorkdir == "" || targetWorkdir == "" {
		return false
	}

	// Normalize paths (remove trailing slashes)
	processWorkdir = strings.TrimSuffix(processWorkdir, "/")
	targetWorkdir = strings.TrimSuffix(targetWorkdir, "/")

	// Exact match
	if processWorkdir == targetWorkdir {
		return true
	}

	// Process workdir is under target workdir (target is parent)
	if strings.HasPrefix(processWorkdir, targetWorkdir+"/") {
		return true
	}

	// Target workdir is under process workdir (process is parent)
	if strings.HasPrefix(targetWorkdir, processWorkdir+"/") {
		return true
	}

	return false
}

// matchesCommand checks if a process matches the command pattern regex
func matchesCommand(proc LocalProcess, pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex, treat as literal substring match
		return strings.Contains(proc.Command, pattern) || strings.Contains(proc.Args, pattern)
	}

	return re.MatchString(proc.Command) || re.MatchString(proc.Args)
}
