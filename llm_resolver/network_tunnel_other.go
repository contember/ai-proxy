//go:build !darwin

package llm_resolver

import "go.uber.org/zap"

// NetworkTunnel is a no-op on non-macOS platforms
type NetworkTunnel struct{}

// NewNetworkTunnel creates a no-op tunnel on non-macOS
func NewNetworkTunnel(logger *zap.Logger) *NetworkTunnel {
	return &NetworkTunnel{}
}

// Start is a no-op on non-macOS (Docker networks are directly accessible on Linux)
func (nt *NetworkTunnel) Start() error {
	return nil
}

// Stop is a no-op on non-macOS
func (nt *NetworkTunnel) Stop() {}

// IsRunning always returns false on non-macOS
func (nt *NetworkTunnel) IsRunning() bool {
	return false
}
