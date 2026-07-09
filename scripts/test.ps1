$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Checking web build..." -ForegroundColor Cyan
Push-Location "$root\web"
if (-not (Test-Path "node_modules")) {
  npm install
}
npm run build
if ($LASTEXITCODE -ne 0) { Write-Host "web build FAILED" -ForegroundColor Red; exit $LASTEXITCODE }
Pop-Location

Write-Host "Checking Go availability..." -ForegroundColor Cyan
$goAvailable = $false
try {
  $goVersion = go version 2>&1
  Write-Host "Local Go found: $goVersion"
  $goAvailable = $true
} catch {
  Write-Host "Local Go not found, will attempt Docker-based test."
}

if ($goAvailable) {
  Write-Host "Running server tests locally..." -ForegroundColor Cyan
  Push-Location "$root\server"
  go test ./...
  if ($LASTEXITCODE -ne 0) { Write-Host "server tests FAILED" -ForegroundColor Red; Pop-Location; exit $LASTEXITCODE }
  Pop-Location

  Write-Host "Running agent tests locally..." -ForegroundColor Cyan
  Push-Location "$root\agent"
  go test ./...
  if ($LASTEXITCODE -ne 0) { Write-Host "agent tests FAILED" -ForegroundColor Red; Pop-Location; exit $LASTEXITCODE }
  Pop-Location
} else {
    # Detect container runtime for fallback test runner: prefer podman, fallback to docker
    if (Get-Command podman -ErrorAction SilentlyContinue) {
        $runtime = "podman"
    } elseif (Get-Command docker -ErrorAction SilentlyContinue) {
        $runtime = "docker"
    } else {
        Write-Host "ERROR: Neither podman nor docker found for containerized tests." -ForegroundColor Red
        exit 1
    }

    Write-Host "Running server tests in container ($runtime)..." -ForegroundColor Cyan
    & $runtime run --rm -v "${root}\server:/src" -w /src golang:1.24-alpine sh -c "go test ./..."
    if ($LASTEXITCODE -ne 0) { Write-Host "server tests FAILED" -ForegroundColor Red; exit $LASTEXITCODE }

    Write-Host "Running agent tests in container ($runtime)..." -ForegroundColor Cyan
    & $runtime run --rm -v "${root}\agent:/src" -w /src golang:1.24-alpine sh -c "go test ./..."
    if ($LASTEXITCODE -ne 0) { Write-Host "agent tests FAILED" -ForegroundColor Red; exit $LASTEXITCODE }
}

Write-Host "All checks completed." -ForegroundColor Green
