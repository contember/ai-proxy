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

## Installation

### macOS

Use Homebrew (recommended):

```bash
brew tap contember/ai-proxy https://github.com/contember/ai-proxy
brew install caddy-llm-proxy

# Add your API key
echo "LLM_API_KEY=sk-your-key" >> /opt/homebrew/etc/caddy-llm-proxy/env

# Start the service
sudo brew services start caddy-llm-proxy
```

> **Note:** On macOS, Docker cannot discover local processes outside the container. Native installation is required for full process discovery.

### Linux

Use Docker Compose (recommended):

```bash
export LLM_API_KEY=your-key
docker compose up -d
```

Then test with:

```bash
curl -k https://myapp.localhost
```

## Alternative Installation

<details>
<summary>Install script (macOS/Linux)</summary>

```bash
curl -fsSL https://raw.githubusercontent.com/contember/ai-proxy/main/install.sh | bash
```

Handles macOS Gatekeeper automatically.
</details>

<details>
<summary>Manual download</summary>

Download from [Releases](../../releases), then:

```bash
tar xzf caddy-darwin-arm64.tar.gz
sudo LLM_API_KEY=your-key ./caddy run --config Caddyfile
```
</details>

<details>
<summary>Build from source</summary>

```bash
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
xcaddy build --with github.com/contember/ai-proxy/llm_resolver=./llm_resolver
LLM_API_KEY=your-key ./caddy run --config Caddyfile
```
</details>

<details>
<summary>Using a local LLM (Ollama, etc.)</summary>

```bash
export LLM_API_KEY=your-key
export LLM_API_URL=http://localhost:11434/v1/chat/completions
export MODEL=llama3.2
```
</details>

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
# Run with hot reload (requires xcaddy)
xcaddy run --config Caddyfile

# Build Docker image
docker build -t llm-proxy .

# Run tests
go test ./...
```

## License

MIT
