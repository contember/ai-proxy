package llm_resolver

import (
	"sync"
	"time"
)

const defaultProcessCacheTTL = 5 * time.Second

// ProcessCache provides short-lived caching for process discovery results.
// This avoids repeatedly calling DiscoverLocalProcesses() on every request
// while still reflecting port changes within a few seconds.
type ProcessCache struct {
	mu         sync.RWMutex
	processes  []LocalProcess
	lastUpdate time.Time
	ttl        time.Duration
}

// NewProcessCache creates a new ProcessCache with the default TTL (5 seconds)
func NewProcessCache() *ProcessCache {
	return &ProcessCache{
		ttl: defaultProcessCacheTTL,
	}
}

// Get returns the cached processes, refreshing if stale.
// Returns an error only if discovery fails and no cached data exists.
func (c *ProcessCache) Get() ([]LocalProcess, error) {
	c.mu.RLock()
	if time.Since(c.lastUpdate) < c.ttl && c.processes != nil {
		processes := c.processes
		c.mu.RUnlock()
		return processes, nil
	}
	c.mu.RUnlock()

	// Need to refresh
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(c.lastUpdate) < c.ttl && c.processes != nil {
		return c.processes, nil
	}

	processes, err := DiscoverLocalProcesses()
	if err != nil {
		// If we have stale data, return it with a warning
		if c.processes != nil {
			return c.processes, nil
		}
		return nil, err
	}

	c.processes = processes
	c.lastUpdate = time.Now()
	return processes, nil
}

// Invalidate clears the cache, forcing a refresh on next Get()
func (c *ProcessCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUpdate = time.Time{}
}
