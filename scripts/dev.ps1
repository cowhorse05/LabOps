$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Starting LabOps demo with Docker Compose..." -ForegroundColor Cyan
docker compose up --build
