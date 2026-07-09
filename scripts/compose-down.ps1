$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Stopping LabOps demo..." -ForegroundColor Cyan
podman compose down
