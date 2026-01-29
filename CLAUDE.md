# Caddy LLM Proxy

This is a Go project - a Caddy plugin for LLM-based dynamic routing.

## Development

- Use `go build ./...` to build
- Use `go test ./...` to run tests
- Use `go mod tidy` to update dependencies

## Building

```bash
# Build Docker image
docker build -t llm-proxy .

# Or use docker compose
docker compose up -d
```

## Project Structure

```
llm_resolver/          # Caddy module (Go package)
├── module.go          # Caddy module registration
├── handler.go         # HTTP middleware handler
├── resolver.go        # LLM resolution logic
├── cache.go           # Persistent mapping storage
├── discovery.go       # Re-exports discovery functions
└── discovery/         # Service discovery
    ├── docker.go      # Docker container discovery
    └── processes.go   # Local process discovery
```

## Environment Variables

- `OPENROUTER_API_KEY` - Required API key for OpenRouter
- `MODEL` - LLM model (default: `anthropic/claude-haiku-4.5`)
- `COMPOSE_PROJECT` - Own compose project name to filter out
