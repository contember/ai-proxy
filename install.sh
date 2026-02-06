#!/bin/bash
set -e

REPO="contember/tudy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/usr/local/etc/tudy}"
DATA_DIR="${DATA_DIR:-/var/lib/tudy}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest version
if [ -z "$VERSION" ]; then
  VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
fi

if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version"
  exit 1
fi

echo "Installing tudy v${VERSION} for ${OS}/${ARCH}..."

# Download and extract
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/caddy-${OS}-${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "Downloading from ${DOWNLOAD_URL}..."
curl -sL "$DOWNLOAD_URL" | tar xz -C "$TMP_DIR"

# Install binary
SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
  SUDO="sudo"
  echo "Installing to $INSTALL_DIR (requires sudo)..."
fi

$SUDO mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$DATA_DIR"
$SUDO mv "$TMP_DIR/caddy" "$INSTALL_DIR/tudy"
$SUDO chmod +x "$INSTALL_DIR/tudy"

# Install Caddyfile if not exists
if [ ! -f "$CONFIG_DIR/Caddyfile" ] && [ -f "$TMP_DIR/Caddyfile" ]; then
  $SUDO mv "$TMP_DIR/Caddyfile" "$CONFIG_DIR/Caddyfile"
fi

# Create env file template if not exists
if [ ! -f "$CONFIG_DIR/env" ]; then
  $SUDO tee "$CONFIG_DIR/env" > /dev/null << EOF
# Add your API key here:
# LLM_API_KEY=sk-your-key-here

CADDY_DATA_DIR=$DATA_DIR
EOF
fi

# macOS: remove quarantine and sign
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/tudy" 2>/dev/null || true
  codesign --force --deep --sign - "$INSTALL_DIR/tudy" 2>/dev/null || true
fi

cat << EOF

Installed to $INSTALL_DIR/tudy
Config: $CONFIG_DIR/Caddyfile

Add your API key and start:
  echo 'LLM_API_KEY=sk-your-key' >> $CONFIG_DIR/env
  sudo bash -c 'set -a; source $CONFIG_DIR/env; tudy run --config $CONFIG_DIR/Caddyfile'
EOF
