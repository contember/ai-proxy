# Caddy LLM Proxy

A Caddy module that provides LLM-based dynamic routing for local development. It automatically discovers local processes and Docker containers, then uses an LLM (via OpenRouter) to intelligently route requests based on hostname patterns.

## Features

- **Dynamic hostname resolution** using LLM (Claude via OpenRouter)
- **Automatic service discovery**:
  - Local processes with open ports (Linux: `ss`/`/proc`, macOS: `lsof`)
  - Docker containers (via Docker API)
- **Cross-platform**: Works on Linux and macOS
- **On-demand TLS certificates** for `*.localhost` domains
- **Persistent mapping cache** (JSON file)
- **Debug dashboard** at `proxy.localhost` or `/_debug`
- **Second-level proxy** for inter-service communication (`/_proxy/serviceName/path`)
- **REST API** for managing mappings (`/_api/mappings/`)

## Quick Start

### Pre-built Binaries

Download pre-built binaries from [Releases](../../releases):

- `caddy-linux-amd64` - Linux x86_64
- `caddy-linux-arm64` - Linux ARM64
- `caddy-darwin-amd64` - macOS Intel
- `caddy-darwin-arm64` - macOS Apple Silicon

```bash
# Download and run (example for macOS ARM64)
chmod +x caddy-darwin-arm64
sudo OPENROUTER_API_KEY=your-key ./caddy-darwin-arm64 run --config Caddyfile
```

### Using Docker Compose

```bash
# Set your OpenRouter API key
export OPENROUTER_API_KEY=your-key-here

# Build and run
docker compose up -d

# Test
curl -k https://myapp.localhost
```

### Building Manually

```bash
# Install xcaddy
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest

# Build Caddy with the plugin
xcaddy build --with github.com/contember/ai-proxy/llm_resolver=./llm_resolver

# Run
OPENROUTER_API_KEY=your-key ./caddy run --config Caddyfile
```

### Running on macOS

On macOS, running inside Docker limits process discovery (can only see processes inside the container). For full local process visibility, run natively:

```bash
# Build
xcaddy build --with github.com/contember/ai-proxy/llm_resolver=./llm_resolver

# Run (requires sudo for ports 80/443)
sudo OPENROUTER_API_KEY=your-key ./caddy run --config Caddyfile
```

The proxy uses `lsof` on macOS to discover listening processes, which works without any special privileges when running natively.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | (required) | API key for OpenRouter |
| `MODEL` | `anthropic/claude-haiku-4.5` | LLM model to use |
| `COMPOSE_PROJECT` | - | Own compose project name to filter out |

### Caddyfile Directives

```caddyfile
llm_resolver {
    openrouter_api_key {env.OPENROUTER_API_KEY}
    model anthropic/claude-haiku-4.5
    cache_file /data/mappings.json
    compose_project myproject
}
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/_tls_check` | TLS verification for on-demand certificates |
| `/_debug` | Debug dashboard (HTML or JSON based on Accept header) |
| `/_api/mappings/` | GET: list all mappings |
| `/_api/mappings/{hostname}` | GET/PUT/DELETE: manage specific mapping |
| `/_proxy/{service}/{path}` | Second-level proxy for inter-service communication |

## Query Parameters

| Parameter | Description |
|-----------|-------------|
| `?force` | Force re-resolution (bypass cache) |
| `?prompt=text` | Provide additional context to the LLM |

## How It Works

1. Request arrives with a hostname (e.g., `api.myproject.localhost`)
2. Module checks the mapping cache
3. If not cached, it:
   - Discovers local processes with open ports
   - Discovers running Docker containers
   - Calls the LLM with hostname + service list
   - LLM returns the best matching target
   - Result is cached
4. Request is proxied to the resolved target

## Development

```bash
# Run with hot reload
go run . run --config Caddyfile

# Build Docker image
docker build -t llm-proxy .

# Run tests
go test ./...
```

## License

MIT
