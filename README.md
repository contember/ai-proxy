# Caddy LLM Proxy

A Caddy module that provides LLM-based dynamic routing for local development. It automatically discovers local processes and Docker containers, then uses an LLM to intelligently route requests based on hostname patterns. Supports any OpenAI-compatible API (OpenRouter, Ollama, LM Studio, vLLM, etc.).

## Features

- **Dynamic hostname resolution** using any OpenAI-compatible LLM API
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

### macOS (Homebrew)

```bash
# Add tap and install
brew tap contember/ai-proxy https://github.com/contember/ai-proxy
brew install caddy-llm-proxy

# Run manually
export LLM_API_KEY=your-key
sudo caddy-llm-proxy run --config /path/to/Caddyfile

# Or as a background service
brew services start caddy-llm-proxy
```

### macOS/Linux (Install Script)

```bash
curl -fsSL https://raw.githubusercontent.com/contember/ai-proxy/main/install.sh | bash
```

The script automatically handles macOS security (Gatekeeper) by removing quarantine and signing the binary.

### Pre-built Binaries

Download pre-built binaries from [Releases](../../releases):

```bash
# Download, extract and run
tar xzf caddy-darwin-arm64.tar.gz
sudo LLM_API_KEY=your-key ./caddy run --config Caddyfile
```

### Using Docker Compose

```bash
# Set your API key
export LLM_API_KEY=your-key-here

# Optionally use a local LLM
export LLM_API_URL=http://localhost:11434/v1/chat/completions  # Ollama
export MODEL=llama3.2

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
LLM_API_KEY=your-key ./caddy run --config Caddyfile
```

### Running on macOS

On macOS, running inside Docker limits process discovery (can only see processes inside the container). For full local process visibility, run natively using Homebrew or the install script above.

The proxy uses `lsof` on macOS to discover listening processes, which works without any special privileges when running natively. Requires `sudo` for binding to ports 80/443.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_API_KEY` | (required) | API key for the LLM API |
| `LLM_API_URL` | `https://openrouter.ai/api/v1/chat/completions` | LLM API endpoint (OpenAI-compatible) |
| `MODEL` | `anthropic/claude-haiku-4.5` | LLM model to use |
| `COMPOSE_PROJECT` | - | Own compose project name to filter out |

### Caddyfile Directives

```caddyfile
llm_resolver {
    api_key {env.LLM_API_KEY}
    api_url {env.LLM_API_URL}
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
