//go:build darwin

package llm_resolver

import "go.uber.org/zap"

// NetworkTunnel is disabled on macOS for now.
// The WireGuard tunnel to Docker VM requires a setup container that is not
// publicly available. Instead, we rely on published port detection which
// works well for most use cases.
//
// To access Docker container IPs directly on macOS, consider using:
// - docker-mac-net-connect: https://github.com/chipmk/docker-mac-net-connect
// - Or publish ports with -p flag when running containers
type NetworkTunnel struct{}

// NewNetworkTunnel creates a no-op tunnel on macOS
func NewNetworkTunnel(logger *zap.Logger) *NetworkTunnel {
	logger.Debug("network tunnel disabled on macOS, using published port detection")
	return &NetworkTunnel{}
}

// Start is a no-op - published port detection is used instead
func (nt *NetworkTunnel) Start() error {
	return nil
}

// Stop is a no-op
func (nt *NetworkTunnel) Stop() {}

// IsRunning always returns false
func (nt *NetworkTunnel) IsRunning() bool {
	return false
}
