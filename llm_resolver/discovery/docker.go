package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const commandTimeout = 10 * time.Second

// Pre-compiled regex for port extraction
var portRegex = regexp.MustCompile(`^(\d+)`)

// DockerContainer represents a discovered Docker container
type DockerContainer struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Ports   []int             `json:"ports"`
	IP      string            `json:"ip"`
	Network string            `json:"network"`
	Workdir string            `json:"workdir"`
	Labels  map[string]string `json:"labels"`
}

// dockerPsOutput represents the JSON output from docker ps
type dockerPsOutput struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Ports  string `json:"Ports"`
	Labels string `json:"Labels"`
}

// dockerInspectOutput represents the relevant parts of docker inspect output
type dockerInspectOutput struct {
	Name            string `json:"Name"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Config struct {
		Image        string            `json:"Image"`
		Labels       map[string]string `json:"Labels"`
		ExposedPorts map[string]struct{}
		WorkingDir   string `json:"WorkingDir"`
	} `json:"Config"`
}

// DiscoverDockerContainers discovers running Docker containers
func DiscoverDockerContainers(ownComposeProject string) ([]DockerContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	// Check if docker is available
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return nil, nil // Docker not available, return empty list
	}

	// Get running containers
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []DockerContainer

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var ps dockerPsOutput
		if err := json.Unmarshal([]byte(line), &ps); err != nil {
			continue
		}

		// Get detailed container info
		details, err := getContainerDetails(ps.ID)
		if err != nil || details == nil {
			continue
		}

		// Filter out containers from our own compose project
		if ownComposeProject != "" {
			containerProject := details.Labels["com.docker.compose.project"]
			if containerProject == ownComposeProject {
				continue
			}
		}

		containers = append(containers, *details)
	}

	return containers, nil
}

// getContainerDetails gets detailed information about a container
func getContainerDetails(containerID string) (*DockerContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var inspectData []dockerInspectOutput
	if err := json.Unmarshal(output, &inspectData); err != nil {
		return nil, err
	}

	if len(inspectData) == 0 {
		return nil, nil
	}

	data := inspectData[0]

	// Get first available network and IP
	var ip, network string
	for netName, netConfig := range data.NetworkSettings.Networks {
		if netConfig.IPAddress != "" {
			ip = netConfig.IPAddress
			network = netName
			break
		}
	}

	// Extract exposed ports
	var ports []int
	for portSpec := range data.Config.ExposedPorts {
		if match := portRegex.FindStringSubmatch(portSpec); len(match) > 1 {
			if port, err := parsePort(match[1]); err == nil {
				ports = append(ports, port)
			}
		}
	}

	// Get workdir - prefer docker-compose working_dir label, then container's WorkingDir
	workdir := data.Config.Labels["com.docker.compose.project.working_dir"]
	if workdir == "" {
		workdir = data.Config.WorkingDir
	}

	// Clean container name (remove leading /)
	name := strings.TrimPrefix(data.Name, "/")

	return &DockerContainer{
		ID:      containerID,
		Name:    name,
		Image:   data.Config.Image,
		Ports:   ports,
		IP:      ip,
		Network: network,
		Workdir: workdir,
		Labels:  data.Config.Labels,
	}, nil
}

// GetContainerIP gets the IP address of a container by name or ID
func GetContainerIP(containerIDOrName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerIDOrName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// parsePort parses and validates a port string, returning error for invalid input
func parsePort(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty port string")
	}
	var port int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character in port: %c", c)
		}
		port = port*10 + int(c-'0')
		if port > 65535 {
			return 0, fmt.Errorf("port out of range: %d", port)
		}
	}
	if port < 1 {
		return 0, fmt.Errorf("port must be >= 1")
	}
	return port, nil
}
