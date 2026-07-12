#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Detect container runtime: prefer podman, fallback to docker
if command -v podman &>/dev/null; then
  RUNTIME="podman"
elif command -v docker &>/dev/null; then
  RUNTIME="docker"
else
  echo "ERROR: Neither podman nor docker found." >&2
  exit 1
fi

echo "Stopping LabOps demo..."
"$RUNTIME" compose -f compose.dev.yaml down
