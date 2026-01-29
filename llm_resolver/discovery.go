package llm_resolver

import (
	"github.com/matej21/caddy-llm-proxy/llm_resolver/discovery"
)

// Re-export types from discovery package
type LocalProcess = discovery.LocalProcess
type DockerContainer = discovery.DockerContainer

// DiscoverLocalProcesses discovers locally running processes with open ports
func DiscoverLocalProcesses() ([]LocalProcess, error) {
	return discovery.DiscoverLocalProcesses()
}

// DiscoverDockerContainers discovers running Docker containers
func DiscoverDockerContainers(ownComposeProject string) ([]DockerContainer, error) {
	return discovery.DiscoverDockerContainers(ownComposeProject)
}

// GetContainerIP gets the IP address of a container by name or ID
func GetContainerIP(containerIDOrName string) (string, error) {
	return discovery.GetContainerIP(containerIDOrName)
}
