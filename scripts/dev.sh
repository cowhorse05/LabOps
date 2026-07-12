#!/usr/bin/env bash
set -euo pipefail

# LabOps dev launcher — auto-detects docker vs podman
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Detect container runtime: prefer podman, fallback to docker
if command -v podman &>/dev/null; then
  RUNTIME="podman"
elif command -v docker &>/dev/null; then
  RUNTIME="docker"
else
  echo "ERROR: Neither podman nor docker found. Please install one of them." >&2
  exit 1
fi

echo "Using container runtime: $RUNTIME"
echo "Starting LabOps demo with Docker Compose..."
"$RUNTIME" compose -f compose.dev.yaml up --build
