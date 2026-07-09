#!/usr/bin/env bash
set -euo pipefail

# LabOps dev launcher — auto-detects docker vs podman
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Detect container runtime
if command -v docker &>/dev/null; then
  RUNTIME="docker"
elif command -v podman &>/dev/null; then
  RUNTIME="podman"
else
  echo "ERROR: Neither docker nor podman found. Please install one of them." >&2
  exit 1
fi

echo "Using container runtime: $RUNTIME"
echo "Starting LabOps demo with Docker Compose..."
"$RUNTIME" compose up --build
