# Caddy LLM Proxy

This is a Go project - a Caddy plugin for LLM-based dynamic routing.

## Development

- Use `xcaddy build --with github.com/contember/ai-proxy/llm_resolver=./llm_resolver` to build
- Use `xcaddy run --config Caddyfile` to run with hot reload
- Use `go test ./...` to run tests
- Use `go mod tidy` to update dependencies (run in llm_resolver/ directory)

## Building

```bash
# Build Docker image
docker build -t llm-proxy .

# Or use docker compose
docker compose up -d
```

## Project Structure

```
llm_resolver/                    # Caddy module (Go package)
├── module.go                    # Caddy module registration
├── handler.go                   # HTTP middleware handler
├── resolver.go                  # LLM resolution logic
├── cache.go                     # Persistent mapping storage
├── discovery.go                 # Re-exports discovery functions
├── port_resolver.go             # Dynamic port resolution
├── process_cache.go             # Short-lived cache for process discovery
├── network_tunnel_darwin.go     # macOS Docker VM networking
├── network_tunnel_other.go      # Stub for non-Darwin platforms
└── discovery/                   # Service discovery
    ├── docker.go                # Docker container discovery
    └── processes.go             # Local process discovery

cmd/menubar/                     # macOS menubar app
├── main.go                      # Entry point
├── app.go                       # Application logic
├── config.go                    # Configuration handling
├── dialog.go                    # UI dialogs
└── proxy.go                     # Proxy control
```

## Environment Variables

- `LLM_API_KEY` - Required API key for the LLM API
- `LLM_API_URL` - LLM API URL (default: `https://openrouter.ai/api/v1/chat/completions`)
- `MODEL` - LLM model (default: `anthropic/claude-haiku-4.5`)
- `COMPOSE_PROJECT` - Own compose project name to filter out
