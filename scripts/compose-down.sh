#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if command -v docker &>/dev/null; then
  RUNTIME="docker"
elif command -v podman &>/dev/null; then
  RUNTIME="podman"
else
  echo "ERROR: Neither docker nor podman found." >&2
  exit 1
fi

echo "Stopping LabOps demo..."
"$RUNTIME" compose down
