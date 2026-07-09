$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# Detect container runtime: prefer podman, fallback to docker
if (Get-Command podman -ErrorAction SilentlyContinue) {
    $runtime = "podman"
} elseif (Get-Command docker -ErrorAction SilentlyContinue) {
    $runtime = "docker"
} else {
    Write-Host "ERROR: Neither podman nor docker found. Please install one of them." -ForegroundColor Red
    exit 1
}

Write-Host "Using container runtime: $runtime" -ForegroundColor Cyan
Write-Host "Starting LabOps demo with Docker Compose..."
& $runtime compose up --build
