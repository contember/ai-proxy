package llm_resolver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m *LLMResolver) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	hostname := extractHostname(r)

	m.logger.Debug("handling request",
		zap.String("host", hostname),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)

	// Handle special paths

	// TLS check endpoint for on-demand TLS
	if r.URL.Path == "/_tls_check" {
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			domain = hostname
		}
		if strings.HasSuffix(domain, ".localhost") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return nil
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Not allowed"))
		return nil
	}

	// Debug endpoint
	if hostname == "proxy.localhost" || r.URL.Path == "/_debug" {
		return m.handleDebug(w, r)
	}

	// API endpoints for mapping management
	if strings.HasPrefix(r.URL.Path, "/_api/mappings/") {
		return m.handleMappingsAPI(w, r)
	}

	// Second-level proxy for inter-service communication
	if strings.HasPrefix(r.URL.Path, "/_proxy/") {
		return m.handleSecondLevelProxy(w, r, hostname, next)
	}

	// Ignore common browser requests
	if r.URL.Path == "/favicon.ico" || r.URL.Path == "/robots.txt" {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	// Check for force refresh and custom prompt
	force := r.URL.Query().Has("force")
	userPrompt := r.URL.Query().Get("prompt")

	// Get or resolve target
	var mapping *RouteMapping
	var err error

	if !force {
		mapping = m.cache.Get(hostname)
	}

	if mapping == nil {
		m.logger.Info("resolving target",
			zap.String("hostname", hostname),
			zap.Bool("forced", force),
		)

		// Use singleflight to deduplicate concurrent requests for same hostname
		result, err, shared := m.resolveGroup.Do(hostname, func() (interface{}, error) {
			// Double-check cache inside singleflight (another request may have just finished)
			if cached := m.cache.Get(hostname); cached != nil && !force {
				return cached, nil
			}

			resolved, err := m.resolver.ResolveTarget(hostname, userPrompt, m.cache.GetAll())
			if err != nil {
				return nil, err
			}

			// Cache the result
			m.cache.Set(hostname, resolved)
			if err := m.cache.Save(); err != nil {
				m.logger.Warn("failed to save cache", zap.Error(err))
			}

			return resolved, nil
		})

		if err != nil {
			m.logger.Error("failed to resolve target",
				zap.String("hostname", hostname),
				zap.Error(err),
			)
			http.Error(w, fmt.Sprintf("Failed to resolve target: %v", err), http.StatusBadGateway)
			return nil
		}

		mapping = result.(*RouteMapping)

		m.logger.Info("resolved target",
			zap.String("hostname", hostname),
			zap.String("type", mapping.Type),
			zap.String("target", mapping.Target),
			zap.Int("port", mapping.Port),
			zap.String("reason", mapping.LLMReason),
			zap.Bool("shared", shared),
		)
	}

	// Build upstream URL
	upstream, err := m.buildUpstreamURL(mapping)
	if err != nil {
		m.logger.Error("failed to build upstream URL",
			zap.String("hostname", hostname),
			zap.Error(err),
		)
		http.Error(w, fmt.Sprintf("Failed to build upstream: %v", err), http.StatusBadGateway)
		return nil
	}

	m.logger.Debug("proxying request",
		zap.String("hostname", hostname),
		zap.String("upstream", upstream),
	)

	// Set the upstream variable for reverse_proxy to use
	caddyhttp.SetVar(r.Context(), "upstream", upstream)

	return next.ServeHTTP(w, r)
}

// handleSecondLevelProxy handles /_proxy/serviceName/path requests
func (m *LLMResolver) handleSecondLevelProxy(w http.ResponseWriter, r *http.Request, originHostname string, next caddyhttp.Handler) error {
	// Parse /_proxy/serviceName/path
	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/_proxy/"), "/", 2)
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Invalid proxy path", http.StatusBadRequest)
		return nil
	}

	serviceName := pathParts[0]
	remainingPath := "/"
	if len(pathParts) > 1 {
		remainingPath = "/" + pathParts[1]
	}

	m.logger.Info("second-level proxy request",
		zap.String("origin", originHostname),
		zap.String("service", serviceName),
		zap.String("path", remainingPath),
	)

	force := r.URL.Query().Has("force")
	userPrompt := r.URL.Query().Get("prompt")

	// Cache key for related service
	cacheKey := fmt.Sprintf("%s:%s", originHostname, serviceName)

	var mapping *RouteMapping
	var err error

	if !force {
		mapping = m.cache.Get(cacheKey)
	}

	if mapping == nil {
		// Use singleflight to deduplicate concurrent requests for same cache key
		result, err, shared := m.resolveGroup.Do(cacheKey, func() (interface{}, error) {
			// Double-check cache inside singleflight
			if cached := m.cache.Get(cacheKey); cached != nil && !force {
				return cached, nil
			}

			// Get origin mapping for context
			originMapping := m.cache.Get(originHostname)

			resolved, err := m.resolver.ResolveRelatedService(
				originHostname,
				originMapping,
				serviceName,
				userPrompt,
				m.cache.GetAll(),
			)
			if err != nil {
				return nil, err
			}

			// Cache the result
			m.cache.Set(cacheKey, resolved)
			if err := m.cache.Save(); err != nil {
				m.logger.Warn("failed to save cache", zap.Error(err))
			}

			return resolved, nil
		})

		if err != nil {
			m.logger.Error("failed to resolve related service",
				zap.String("origin", originHostname),
				zap.String("service", serviceName),
				zap.Error(err),
			)
			http.Error(w, fmt.Sprintf("Failed to resolve service: %v", err), http.StatusBadGateway)
			return nil
		}

		mapping = result.(*RouteMapping)

		m.logger.Info("resolved related service",
			zap.String("origin", originHostname),
			zap.String("service", serviceName),
			zap.String("type", mapping.Type),
			zap.String("target", mapping.Target),
			zap.Int("port", mapping.Port),
			zap.Bool("shared", shared),
		)
	}

	// Build upstream URL
	upstream, err := m.buildUpstreamURL(mapping)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build upstream: %v", err), http.StatusBadGateway)
		return nil
	}

	// Modify request path to remove /_proxy/serviceName prefix
	r.URL.Path = remainingPath

	// Set upstream for reverse_proxy
	caddyhttp.SetVar(r.Context(), "upstream", upstream)

	return next.ServeHTTP(w, r)
}

// handleDebug returns debug information
func (m *LLMResolver) handleDebug(w http.ResponseWriter, r *http.Request) error {
	// Check if HTML is requested
	acceptsHTML := strings.Contains(r.Header.Get("Accept"), "text/html")

	if acceptsHTML {
		return m.handleDebugHTML(w, r)
	}

	// Return JSON
	data := map[string]interface{}{
		"mappings":   m.cache.GetAll(),
		"model":      m.Model,
		"cache_file": m.CacheFile,
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

// handleDebugHTML returns an HTML debug page
func (m *LLMResolver) handleDebugHTML(w http.ResponseWriter, r *http.Request) error {
	// Get discovery data for the page
	processes, _ := DiscoverLocalProcesses()
	containers, _ := DiscoverDockerContainers(m.ComposeProject)
	mappings := m.cache.GetAll()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>LLM Proxy Debug</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #1a1a2e; color: #eee; }
        h1 { color: #00d9ff; }
        h2 { color: #ff6b6b; border-bottom: 1px solid #333; padding-bottom: 5px; }
        .section { background: #16213e; border-radius: 8px; padding: 15px; margin-bottom: 20px; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 8px; border-bottom: 1px solid #333; }
        th { color: #00d9ff; }
        .type-process { color: #4ade80; }
        .type-docker { color: #60a5fa; }
        pre { background: #0f0f1a; padding: 10px; border-radius: 4px; overflow-x: auto; }
        .actions button { background: #ff6b6b; color: white; border: none; padding: 5px 10px; border-radius: 4px; cursor: pointer; }
        .actions button:hover { background: #ff5252; }
        .refresh { background: #00d9ff !important; }
        .refresh:hover { background: #00b8d9 !important; }
    </style>
</head>
<body>
    <h1>LLM Proxy Debug Dashboard</h1>
    <p><button class="refresh" onclick="location.reload()">Refresh</button></p>

    <div class="section">
        <h2>Configuration</h2>
        <table>
            <tr><th>Model</th><td>` + m.Model + `</td></tr>
            <tr><th>Cache File</th><td>` + m.CacheFile + `</td></tr>
        </table>
    </div>

    <div class="section">
        <h2>Current Mappings</h2>
        <table>
            <tr><th>Hostname</th><th>Type</th><th>Target</th><th>Port</th><th>Reason</th><th>Actions</th></tr>`

	for hostname, mapping := range mappings {
		typeClass := "type-process"
		if mapping.Type == "docker" {
			typeClass = "type-docker"
		}
		html += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td class="%s">%s</td>
                <td>%s</td>
                <td>%d</td>
                <td>%s</td>
                <td class="actions"><button onclick="deleteMapping('%s')">Delete</button></td>
            </tr>`, hostname, typeClass, mapping.Type, mapping.Target, mapping.Port, mapping.LLMReason, hostname)
	}

	html += `
        </table>
    </div>

    <div class="section">
        <h2>Local Processes</h2>
        <table>
            <tr><th>Port</th><th>Command</th><th>Working Directory</th></tr>`

	for _, proc := range processes {
		// Use args if available (more informative), fallback to command
		cmd := proc.Args
		if cmd == "" {
			cmd = proc.Command
		}
		// Truncate long commands for display
		if len(cmd) > 100 {
			cmd = cmd[:100] + "..."
		}
		html += fmt.Sprintf(`
            <tr>
                <td>%d</td>
                <td>%s</td>
                <td>%s</td>
            </tr>`, proc.Port, cmd, proc.Workdir)
	}

	html += `
        </table>
    </div>

    <div class="section">
        <h2>Docker Containers</h2>
        <table>
            <tr><th>Name</th><th>Image</th><th>Ports</th><th>IP</th><th>Workdir</th></tr>`

	for _, container := range containers {
		ports := ""
		for i, p := range container.Ports {
			if i > 0 {
				ports += ", "
			}
			ports += fmt.Sprintf("%d", p)
		}
		html += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
            </tr>`, container.Name, container.Image, ports, container.IP, container.Workdir)
	}

	html += `
        </table>
    </div>

    <script>
        async function deleteMapping(hostname) {
            if (!confirm('Delete mapping for ' + hostname + '?')) return;
            const resp = await fetch('/_api/mappings/' + encodeURIComponent(hostname), { method: 'DELETE' });
            if (resp.ok) location.reload();
            else alert('Failed to delete');
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
	return nil
}

// handleMappingsAPI handles CRUD operations for mappings
func (m *LLMResolver) handleMappingsAPI(w http.ResponseWriter, r *http.Request) error {
	hostname := strings.TrimPrefix(r.URL.Path, "/_api/mappings/")
	hostname = strings.TrimSuffix(hostname, "/")

	switch r.Method {
	case http.MethodGet:
		if hostname == "" {
			// List all mappings
			w.Header().Set("Content-Type", "application/json")
			return json.NewEncoder(w).Encode(m.cache.GetAll())
		}
		// Get specific mapping
		mapping := m.cache.Get(hostname)
		if mapping == nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return nil
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(mapping)

	case http.MethodPut:
		var body struct {
			Type   string `json:"type"`
			Target string `json:"target"`
			Port   int    `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return nil
		}
		if body.Type != "process" && body.Type != "docker" {
			http.Error(w, "Invalid type", http.StatusBadRequest)
			return nil
		}
		mapping := &RouteMapping{
			Type:      body.Type,
			Target:    body.Target,
			Port:      body.Port,
			CreatedAt: timeNow(),
			LLMReason: "Manually edited",
		}
		m.cache.Set(hostname, mapping)
		if err := m.cache.Save(); err != nil {
			http.Error(w, "Failed to save", http.StatusInternalServerError)
			return nil
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Updated"))
		return nil

	case http.MethodDelete:
		m.cache.Delete(hostname)
		if err := m.cache.Save(); err != nil {
			http.Error(w, "Failed to save", http.StatusInternalServerError)
			return nil
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Deleted"))
		return nil

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil
	}
}

// buildUpstreamURL creates the upstream URL for the reverse proxy
func (m *LLMResolver) buildUpstreamURL(mapping *RouteMapping) (string, error) {
	if mapping.Type == "process" {
		port := mapping.Port

		// Try dynamic port resolution if ProcessIdentifier is available
		if mapping.ProcessIdentifier != nil && m.processCache != nil {
			resolvedPort, err := ResolveProcessPort(mapping.ProcessIdentifier, m.processCache)
			if err != nil {
				m.logger.Warn("dynamic port resolution failed, using cached port",
					zap.String("workdir", mapping.ProcessIdentifier.Workdir),
					zap.Int("fallbackPort", mapping.Port),
					zap.Error(err),
				)
			} else {
				port = resolvedPort
			}
		}

		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}

	// Docker container - get IP
	ip, err := GetContainerIP(mapping.Target)
	if err != nil || ip == "" {
		return "", fmt.Errorf("cannot resolve IP for container %s: %v", mapping.Target, err)
	}

	return fmt.Sprintf("%s:%d", ip, mapping.Port), nil
}

// extractHostname extracts the hostname from the request, removing the port
func extractHostname(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	// Handle IPv6 addresses: [::1]:port or [2001:db8::1]:8080
	if strings.HasPrefix(host, "[") {
		// IPv6 address in brackets
		if idx := strings.LastIndex(host, "]:"); idx != -1 {
			// Has port, extract just the bracketed address
			host = host[:idx+1]
		}
		// Remove brackets for cleaner hostname
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
		return host
	}
	// IPv4 or hostname: remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}

// timeNow returns the current time as ISO string
func timeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
