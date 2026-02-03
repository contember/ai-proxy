#!/bin/bash
set -e

REPO="contember/ai-proxy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="caddy-llm-proxy"

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

echo "Installing caddy-llm-proxy v${VERSION} for ${OS}/${ARCH}..."

# Download and extract
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${VERSION}/caddy-${OS}-${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "Downloading from ${DOWNLOAD_URL}..."
curl -sL "$DOWNLOAD_URL" | tar xz -C "$TMP_DIR"

# Install binary
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/caddy" "$INSTALL_DIR/$BINARY_NAME"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP_DIR/caddy" "$INSTALL_DIR/$BINARY_NAME"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

# macOS: remove quarantine and sign
if [ "$OS" = "darwin" ]; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
  codesign --force --deep --sign - "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || true
fi

echo "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "To run:"
echo "  export LLM_API_KEY='your-api-key'"
echo "  $BINARY_NAME run --config /path/to/Caddyfile"
