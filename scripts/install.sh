#!/bin/bash
# install.sh - Installer for Council Orchestrator
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.sh | bash -s -- --version v1.1.1

set -e

REPO="alijamal14/council"
BINARY_NAME="council"
INSTALL_DIR="${HOME}/.local/bin"

# Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "${ARCH}" in
    x86_64) ARCH="x86_64" ;;
    aarch64|arm64) ARCH="aarch64" ;;
    *) echo "Unsupported architecture: ${ARCH}"; exit 1 ;;
esac

# Function to get latest version
get_latest_version() {
    curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

# Parse arguments
VERSION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [ -z "${VERSION}" ]; then
    VERSION=$(get_latest_version)
fi

if [ -z "${VERSION}" ]; then
    echo "Could not determine latest version. Please specify with --version."
    exit 1
fi

echo "Installing Council Orchestrator ${VERSION} for ${OS}/${ARCH}..."

# Construct download URL
# Example: https://github.com/alijamal14/council/releases/download/v1.1.1/council_linux_x86_64.tar.gz
ASSET_NAME="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"

# Create install directory if it doesn't exist
mkdir -p "${INSTALL_DIR}"

# Download and extract
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT

echo "Downloading ${DOWNLOAD_URL}..."
curl -fL "${DOWNLOAD_URL}" -o "${TEMP_DIR}/council.tar.gz"

echo "Extracting to ${INSTALL_DIR}..."
tar -xzf "${TEMP_DIR}/council.tar.gz" -C "${TEMP_DIR}"
mv "${TEMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo "Successfully installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"

# Check if INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    echo "⚠️  ${INSTALL_DIR} is not in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
fi

echo "Run 'council --version' to verify installation."
