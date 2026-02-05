#!/bin/bash
set -e

# Build script for the Caddy LLM Proxy menu bar app

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
APP_NAME="Caddy LLM Proxy"
APP_BUNDLE="$BUILD_DIR/$APP_NAME.app"

echo "Building Caddy LLM Proxy menu bar app..."

# Clean previous build
rm -rf "$APP_BUNDLE"
mkdir -p "$BUILD_DIR"

# Build the Go binary
echo "Compiling Go binary..."
cd "$PROJECT_ROOT/cmd/menubar"

# CGO is required for systray on macOS
CGO_ENABLED=1 go build -ldflags="-s -w" -o "$BUILD_DIR/menubar" .

echo "Creating app bundle..."

# Create app bundle structure
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy binary
cp "$BUILD_DIR/menubar" "$APP_BUNDLE/Contents/MacOS/menubar"

# Copy Info.plist
cp "$PROJECT_ROOT/cmd/menubar/resources/Info.plist" "$APP_BUNDLE/Contents/Info.plist"

# Create a simple app icon (using built-in macOS icon)
# For production, you'd want a proper .icns file
echo "Note: Using default app icon. For a custom icon, create AppIcon.icns and place in Resources/"

# Make binary executable
chmod +x "$APP_BUNDLE/Contents/MacOS/menubar"

echo ""
echo "Build complete!"
echo "App bundle: $APP_BUNDLE"
echo ""
echo "To install:"
echo "  cp -r '$APP_BUNDLE' /Applications/"
echo ""
echo "To run directly:"
echo "  open '$APP_BUNDLE'"
echo ""
echo "To add to Login Items (start at login):"
echo "  1. Open System Preferences > General > Login Items"
echo "  2. Click '+' and select '$APP_NAME.app'"
