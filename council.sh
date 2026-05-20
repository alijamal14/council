#!/bin/bash

# 💡 AI Council Orchestrator Wrapper
# This script is a wrapper for the native Go binary.
# Primary Source: ./council.go

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/council"

# Prefer prebuilt Linux amd64 artifact when cwd is Linux (tracked `council` may be another OS/arch).
case "$(uname -s)-$(uname -m)" in
  Linux-x86_64|Linux-amd64)
    [ -x "$SCRIPT_DIR/council-linux-amd64" ] && BINARY="$SCRIPT_DIR/council-linux-amd64"
    ;;
esac

# Auto-build if binary is missing
if [ ! -f "$BINARY" ]; then
    echo "🔧 Council binary not found. Building..."
    (cd "$SCRIPT_DIR" && go build -o council .)
fi

# Execute the Go binary with all arguments
exec "$BINARY" "$@"
