# Build stage - use xcaddy to build Caddy with our plugin
FROM caddy:2-builder AS builder

# Copy the plugin source
COPY . /src

# Build Caddy with the llm_resolver plugin
# Import the llm_resolver subpackage from the root module
RUN xcaddy build \
    --with github.com/contember/tudy/llm_resolver=/src/llm_resolver

# Runtime stage
FROM caddy:2-alpine

# Install required tools:
# - ca-certificates: for HTTPS calls to OpenRouter
# - iproute2: provides 'ss' command for process discovery
# - docker-cli: for Docker container discovery
RUN apk add --no-cache ca-certificates iproute2 docker-cli

# Copy the custom Caddy binary
COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# Copy the Caddyfile
COPY Caddyfile /etc/caddy/Caddyfile

# Create data directory for mappings
RUN mkdir -p /data

# Expose HTTPS port
EXPOSE 443

# Run Caddy
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
