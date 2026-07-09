# LabOps Production Deployment Script (Windows)
param(
    [string]$Mode = "compose",
    [switch]$InstallDeps,
    [string]$DataDir = "$env:ProgramData\LabOps",
    [string]$BinDir = "$env:ProgramFiles\LabOps",
    [switch]$SkipService
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

function Write-Step($msg) {
    Write-Host "`n==> $msg" -ForegroundColor Cyan
}

function Write-Info($msg) {
    Write-Host "[INFO]  $msg" -ForegroundColor Green
}

function Write-Warn($msg) {
    Write-Host "[WARN]  $msg" -ForegroundColor Yellow
}

# ---------------------------------------------------------------------------
# Check prerequisites
# ---------------------------------------------------------------------------
function Check-Prereqs {
    Write-Step "Checking prerequisites"

    $goVer = (go version 2>$null) -replace '.*go(\d+\.\d+).*', '$1'
    if (-not $goVer) {
        if ($InstallDeps) {
            Write-Info "Installing Go..."
            winget install GoLang.Go --accept-package-agreements 2>$null
            $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
        } else {
            throw "Go not found. Install it or use -InstallDeps."
        }
    }
    Write-Info "Go $goVer"

    $nodeVer = (node --version 2>$null)
    if (-not $nodeVer) {
        if ($InstallDeps) {
            Write-Info "Installing Node.js..."
            winget install OpenJS.NodeJS.LTS --accept-package-agreements 2>$null
            $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
        } else {
            throw "Node.js not found. Install it or use -InstallDeps."
        }
    }
    Write-Info "Node.js $nodeVer"
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
function Build-Native {
    Write-Step "Building server"
    Push-Location "$root\server"
    $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o "$root\build\labops-server.exe" .\cmd\server
    Pop-Location
    Write-Info "Server: build\labops-server.exe"

    Write-Step "Building agent"
    Push-Location "$root\agent"
    go build -ldflags="-s -w" -o "$root\build\labops-agent.exe" .\cmd\agent
    Pop-Location
    Write-Info "Agent: build\labops-agent.exe"
}

function Build-Web {
    Write-Step "Building web frontend"
    Push-Location "$root\web"
    npm ci --silent 2>$null
    npm run build
    Pop-Location
    Write-Info "Web: web\dist\"
}

# ---------------------------------------------------------------------------
# Install (native mode)
# ---------------------------------------------------------------------------
function Install-Native {
    Write-Step "Installing to $BinDir"

    if (-not (Test-Path $BinDir)) { New-Item -ItemType Directory -Path $BinDir -Force | Out-Null }
    if (-not (Test-Path $DataDir)) { New-Item -ItemType Directory -Path $DataDir -Force | Out-Null }

    Copy-Item "$root\build\labops-server.exe" "$BinDir\labops-server.exe" -Force
    Copy-Item "$root\build\labops-agent.exe" "$BinDir\labops-agent.exe" -Force

    # Create env file
    $envPath = Join-Path $BinDir "env.ps1"
    if (-not (Test-Path $envPath)) {
        @'
# LabOps server configuration
$env:LABOPS_ADDR = ":8080"
$env:LABOPS_DB_PATH = "$env:ProgramData\LabOps\labops.db"
$env:LABOPS_AGENT_TOKEN = "change-me-agent-token"
$env:LABOPS_WEB_TOKEN = "change-me-web-token"
'@ | Out-File $envPath -Encoding UTF8
        Write-Warn "Edit $envPath to change default tokens!"
    }

    if (-not $SkipService) {
        Write-Step "Creating Windows Service (via NSSM or sc)"
        Write-Warn "Windows service creation requires NSSM (nssm.cc). Install it first:"
        Write-Warn "  winget install NSSM.NSSM"
        Write-Warn "Then:"
        Write-Warn "  nssm install LabOps-Server $BinDir\labops-server.exe"
    }
}

# ---------------------------------------------------------------------------
# Compose mode
# ---------------------------------------------------------------------------
function Deploy-Compose {
    Write-Step "Deploying with Podman/Docker Compose"

    if (Get-Command docker -ErrorAction SilentlyContinue) {
        $runtime = "docker"
    } elseif (Get-Command podman -ErrorAction SilentlyContinue) {
        $runtime = "podman"
    } else {
        throw "Neither docker nor podman found. Install one or use -Mode native."
    }
    Write-Info "Using: $runtime"

    if (-not (Test-Path "$root\.env")) {
        Copy-Item "$root\.env.example" "$root\.env"
        Write-Warn "Created .env from .env.example. Edit before production use!"
    }

    Invoke-Expression "$runtime compose up -d --build"
    Write-Info "Containers started."
    Write-Info "Web UI: http://localhost:5173"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
Write-Host @"
  LabOps Deployment Script (Windows)
  =================================
"@ -ForegroundColor Cyan

if ($InstallDeps) {
    # Refresh PATH
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
}

if ($Mode -eq "native") {
    Check-Prereqs
    New-Item -ItemType Directory -Path "$root\build" -Force | Out-Null
    Build-Native
    Build-Web
    Install-Native

    Write-Host "`n=== Deployment Complete ===" -ForegroundColor Green
    Write-Host @"

  Start server:
    & "$BinDir\labops-server.exe"

  Start agent:
    & "$BinDir\labops-agent.exe" --server=http://localhost:8080 --token=<token> --id=<id> --name=<name>

  Web UI:  http://localhost:5173 (dev) or serve web/dist/ with nginx/Caddy
  Data:    $DataDir\
  Config:  $BinDir\env.ps1
"@
} else {
    Deploy-Compose
}
