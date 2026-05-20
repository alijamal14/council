#!/usr/bin/env bash
# wait-council.sh — timeout-aware wrapper for Claude Code / CI invocation
#
# Exit codes:
#   0 — council completed successfully
#   1 — council reported a real error (partial agent failures)
#   2 — council timed out (check council_runs/ — results may still be on disk)
#
# Usage:
#   ./wait-council.sh "your task"
#   ./wait-council.sh --continue council_runs/run_<dir> "feedback"
#
# Timeout:
#   Default: 1200s (3 agents × 3 attempts × 180s + critique phase overhead)
#   Override: COUNCIL_TIMEOUT=600 ./wait-council.sh "task"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/council"
TIMEOUT="${COUNCIL_TIMEOUT:-1200}"

if [ ! -f "$BINARY" ]; then
  echo "ERROR: council binary not found at $BINARY" >&2
  echo "Build it with: cd $SCRIPT_DIR && go build -o council ." >&2
  exit 1
fi

# macOS ships without GNU timeout — use gtimeout (brew install coreutils) if available
if command -v gtimeout >/dev/null 2>&1; then
  TIMEOUT_CMD="gtimeout"
elif command -v timeout >/dev/null 2>&1; then
  TIMEOUT_CMD="timeout"
else
  echo "WARNING: neither timeout nor gtimeout found — running without timeout guard" >&2
  exec "$BINARY" "$@"
fi

"$TIMEOUT_CMD" "$TIMEOUT" "$BINARY" -- "$@"
STATUS=$?

if [ $STATUS -eq 124 ]; then
  echo "" >&2
  echo "COUNCIL_TIMEOUT: binary ran for ${TIMEOUT}s — check council_runs/ for partial results" >&2
  exit 2
fi

exit $STATUS
