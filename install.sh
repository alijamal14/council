#!/bin/bash
set -e

REPO="alijamal14/council"
BINARY="council"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux*)  OS="Linux" ;;
  darwin*) OS="Darwin" ;;
  msys*|cygwin*|mingw*) OS="Windows" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect Architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
echo "Finding latest version of $BINARY..."
LATEST_RELEASE=$(curl -s "https://api.github.com/repos/$REPO/releases/latest")
VERSION=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Error: Could not find latest release."
  exit 1
fi

echo "Installing $BINARY $VERSION for $OS/$ARCH..."

# Construct Asset Name and URL
# Format: council_Linux_x86_64.tar.gz
EXT="tar.gz"
if [ "$OS" == "Windows" ]; then EXT="zip"; fi
ASSET_NAME="${BINARY}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET_NAME"
CHECKSUM_URL="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download asset and checksums
curl -sSL "$URL" -o "$TMP_DIR/$ASSET_NAME"
curl -sSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt"

# Verify checksum
echo "Verifying checksum..."
(cd "$TMP_DIR" && grep "$ASSET_NAME" checksums.txt | sha256sum -c - > /dev/null 2>&1) || {
  # On macOS, sha256sum might not be available, try shasum
  if command -v shasum > /dev/null 2>&1; then
    (cd "$TMP_DIR" && grep "$ASSET_NAME" checksums.txt | shasum -a 256 -c - > /dev/null 2>&1) || { echo "Checksum verification failed!"; exit 1; }
  else
    echo "Warning: sha256sum/shasum not found. Skipping verification."
  fi
}

# Extract
if [ "$EXT" == "tar.gz" ]; then
  tar -xzf "$TMP_DIR/$ASSET_NAME" -C "$TMP_DIR"
else
  unzip -q "$TMP_DIR/$ASSET_NAME" -d "$TMP_DIR"
fi

# Install
echo "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/"
else
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/"
fi

chmod +x "$INSTALL_DIR/$BINARY"

echo "Successfully installed $BINARY $VERSION to $INSTALL_DIR"
$BINARY --version
