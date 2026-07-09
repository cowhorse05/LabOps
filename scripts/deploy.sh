#!/usr/bin/env bash
set -euo pipefail

# LabOps Production Deployment Script (Linux)
# Supports: Ubuntu 20.04+, Debian 11+, RHEL 8+, Fedora 36+, Arch
# Two modes: native (systemd) or container (docker/podman compose)

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

BOLD="\033[1m"
GREEN="\033[32m"
YELLOW="\033[33m"
RED="\033[31m"
CYAN="\033[36m"
RESET="\033[0m"

log()  { echo -e "${GREEN}[INFO]${RESET}  $*"; }
warn() { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
err()  { echo -e "${RED}[ERROR]${RESET} $*"; exit 1; }
step() { echo -e "\n${CYAN}${BOLD}==> $*${RESET}\n"; }

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
MODE="native"
INSTALL_DEPS="false"
DATA_DIR="/var/lib/labops"
BIN_DIR="/usr/local/bin"
SKIP_SYSTEMD="false"

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --mode native|compose   Deployment mode (default: native)
  --install-deps          Auto-install Go, Node.js, etc.
  --data-dir DIR          Data directory (default: /var/lib/labops)
  --bin-dir DIR           Binary install path (default: /usr/local/bin)
  --skip-systemd          Don't create systemd services
  -h, --help              Show this help

Native mode: builds Go binaries + React frontend, runs via systemd.
Compose mode: uses docker/podman compose (same as dev but production config).
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --install-deps) INSTALL_DEPS="true"; shift ;;
    --data-dir) DATA_DIR="$2"; shift 2 ;;
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --skip-systemd) SKIP_SYSTEMD="true"; shift ;;
    -h|--help) usage ;;
    *) err "Unknown option: $1" ;;
  esac
done

# ---------------------------------------------------------------------------
# Detect OS
# ---------------------------------------------------------------------------
detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS="$ID"
    OS_VERSION="$VERSION_ID"
  elif [ -f /etc/arch-release ]; then
    OS="arch"
    OS_VERSION="rolling"
  else
    OS="unknown"
    OS_VERSION="unknown"
  fi
  log "Detected OS: $OS $OS_VERSION"
}

# ---------------------------------------------------------------------------
# Install dependencies
# ---------------------------------------------------------------------------
install_deps() {
  step "Installing dependencies"
  case "$OS" in
    ubuntu|debian)
      sudo apt-get update -qq
      if ! command -v go &>/dev/null; then
        log "Installing Go..."
        sudo apt-get install -y -qq golang-go
      fi
      if ! command -v node &>/dev/null; then
        log "Installing Node.js..."
        curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
        sudo apt-get install -y -qq nodejs
      fi
      ;;
    fedora|rhel|centos)
      if ! command -v go &>/dev/null; then
        sudo dnf install -y golang
      fi
      if ! command -v node &>/dev/null; then
        sudo dnf install -y nodejs
      fi
      ;;
    arch)
      sudo pacman -S --noconfirm go nodejs npm 2>/dev/null || true
      ;;
    *)
      warn "Unknown OS. Please install Go 1.23+ and Node.js 20+ manually."
      ;;
  esac
  log "Dependencies installed"
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
build_server() {
  step "Building server"
  cd "$ROOT/server"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$ROOT/build/labops-server" ./cmd/server
  log "Server binary: build/labops-server"
}

build_agent() {
  step "Building agent"
  cd "$ROOT/agent"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$ROOT/build/labops-agent" ./cmd/agent
  log "Agent binary: build/labops-agent"
}

build_web() {
  step "Building web frontend"
  cd "$ROOT/web"
  npm ci --silent
  npm run build
  log "Web build: web/dist/"
}

# ---------------------------------------------------------------------------
# Install (native mode)
# ---------------------------------------------------------------------------
install_native() {
  step "Installing binaries to $BIN_DIR"
  sudo mkdir -p "$BIN_DIR"
  sudo cp "$ROOT/build/labops-server" "$BIN_DIR/labops-server"
  sudo cp "$ROOT/build/labops-agent" "$BIN_DIR/labops-agent"
  sudo chmod +x "$BIN_DIR/labops-server" "$BIN_DIR/labops-agent"

  sudo mkdir -p "$DATA_DIR"
  sudo mkdir -p /etc/labops

  # Create env file
  if [ ! -f /etc/labops/env ]; then
    log "Creating default /etc/labops/env"
    sudo tee /etc/labops/env > /dev/null <<'ENVEOF'
# LabOps server configuration
LABOPS_ADDR=:8080
LABOPS_DB_PATH=/var/lib/labops/labops.db
LABOPS_AGENT_TOKEN=change-me-agent-token
LABOPS_WEB_TOKEN=change-me-web-token
ENVEOF
    warn "Edit /etc/labops/env to change default tokens!"
  fi
}

install_systemd() {
  if [ "$SKIP_SYSTEMD" = "true" ]; then
    warn "Skipping systemd service creation"
    return
  fi

  step "Creating systemd service"

  # Create a dedicated user
  if ! id -u labops &>/dev/null; then
    sudo useradd -r -s /bin/false -d /var/lib/labops -m labops
    sudo chown -R labops:labops "$DATA_DIR"
  fi

  sudo tee /etc/systemd/system/labops-server.service > /dev/null <<SERVICEOF
[Unit]
Description=LabOps Server
After=network.target

[Service]
Type=simple
User=labops
Group=labops
EnvironmentFile=/etc/labops/env
ExecStart=$BIN_DIR/labops-server
Restart=on-failure
RestartSec=5
WorkingDirectory=/var/lib/labops

[Install]
WantedBy=multi-user.target
SERVICEOF

  sudo tee /etc/systemd/system/labops-agent@.service > /dev/null <<AGENTEOF
[Unit]
Description=LabOps Agent (%i)
After=network.target labops-server.service
BindsTo=labops-server.service

[Service]
Type=simple
User=labops
Group=labops
ExecStart=$BIN_DIR/labops-agent \\
  --server=http://localhost:8080 \\
  --token=\${LABOPS_AGENT_TOKEN} \\
  --id=%i \\
  --name=%i \\
  --group=default
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
AGENTEOF

  sudo systemctl daemon-reload
  sudo systemctl enable labops-server
  log "Systemd services created. Start with:"
  log "  sudo systemctl start labops-server"
  log "  sudo systemctl start labops-agent@my-pc"
}

# ---------------------------------------------------------------------------
# Compose mode
# ---------------------------------------------------------------------------
deploy_compose() {
  step "Deploying with Docker/Podman Compose"

  # Detect runtime
  if command -v docker &>/dev/null; then
    RUNTIME="docker"
  elif command -v podman &>/dev/null; then
    RUNTIME="podman"
  else
    err "Neither docker nor podman found. Install one or use --mode native."
  fi

  # Create .env from example if missing
  if [ ! -f "$ROOT/.env" ]; then
    cp "$ROOT/.env.example" "$ROOT/.env"
    warn "Created .env from .env.example. Edit before production use!"
  fi

  "$RUNTIME" compose up -d --build
  log "Containers started. Check status: $RUNTIME compose ps"
  log "Web UI: http://localhost:5173"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo -e "${BOLD}${CYAN}"
  echo "  LabOps Deployment Script"
  echo -e "  ========================${RESET}\n"

  detect_os

  if [ "$INSTALL_DEPS" = "true" ]; then
    install_deps
  fi

  # Check prerequisites
  if ! command -v go &>/dev/null; then
    err "Go not found. Install it or use --install-deps."
  fi
  if [ "$MODE" = "native" ] && ! command -v node &>/dev/null; then
    err "Node.js not found. Install it or use --install-deps."
  fi

  mkdir -p "$ROOT/build"

  if [ "$MODE" = "native" ]; then
    build_server
    build_agent
    build_web
    install_native
    install_systemd

    echo ""
    echo -e "${GREEN}${BOLD}=== Deployment Complete ===${RESET}"
    echo ""
    echo "  Start server:  sudo systemctl start labops-server"
    echo "  Start agent:   sudo systemctl start labops-agent@AGENT-ID"
    echo "  Web UI:        http://localhost:8080 (server API)"
    echo "  Data:          $DATA_DIR/"
    echo "  Config:        /etc/labops/env"
    echo ""
    echo "  To serve the web frontend, use nginx or serve directly:"
    echo "    cd $ROOT/web && npx vite preview --host 0.0.0.0"
    echo ""
  else
    deploy_compose
  fi
}

main
