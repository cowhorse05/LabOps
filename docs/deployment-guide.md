# LabOps Production Deployment Guide

This guide walks through deploying LabOps to a real production server from scratch. It reflects the actual deployment of [https://cowhorse.xyz](https://cowhorse.xyz) and is suitable as a step-by-step tutorial.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Server Provisioning](#2-server-provisioning)
3. [Clone & Configure Secrets](#3-clone--configure-secrets)
4. [Obtain SSL Certificate](#4-obtain-ssl-certificate)
5. [Build & Start Services](#5-build--start-services)
6. [First Login & Admin Setup](#6-first-login--admin-setup)
7. [Install Agents on Target Machines](#7-install-agents-on-target-machines)
8. [Revoke & Uninstall Agents](#8-revoke--uninstall-agents)
9. [Backup & Restore](#9-backup--restore)
10. [Upgrading & Rollback](#10-upgrading--rollback)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Prerequisites

### Server Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| OS | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 20 GB | 40 GB SSD |
| Docker | Engine 24+ | Engine 27+ |
| Docker Compose | Plugin v2 | Plugin v2 |

### Network Requirements

| Port | Protocol | Purpose | Required |
|------|----------|---------|:--------:|
| 22 | TCP | SSH access | Yes |
| 80 | TCP | HTTP → HTTPS redirect + ACME validation | Yes |
| 443 | TCP | HTTPS/WSS (Web + Agent WebSocket) | Yes |

Ensure these ports are open in your cloud provider's security group / firewall.

### Domain Setup

1. Purchase a domain name (e.g., `cowhorse.xyz`)
2. Add an **A record** pointing to your server's public IPv4 address
3. Verify DNS propagation: `nslookup cowhorse.xyz` should return your server IP

---

## 2. Server Provisioning

### 2.1 Install Ubuntu

Deploy an Ubuntu 22.04 or 24.04 instance. Update the system:

```bash
sudo apt update && sudo apt upgrade -y
sudo reboot  # if kernel was updated
```

### 2.2 Install Docker Engine

Follow the [official Docker installation guide](https://docs.docker.com/engine/install/ubuntu/):

```bash
# Add Docker's official GPG key
sudo apt install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

### 2.3 Configure Docker

```bash
# Add your user to the docker group (avoids sudo for docker commands)
sudo usermod -aG docker $USER

# Enable Docker to start on boot
sudo systemctl enable docker

# Log out and back in for group changes to take effect
exit
```

Verify installation:

```bash
docker --version          # Docker version 27.x.x
docker compose version    # Docker Compose version v2.x.x
docker run hello-world    # Should print confirmation message
```

### 2.4 Install Git

```bash
sudo apt install -y git
```

---

## 3. Clone & Configure Secrets

### 3.1 Clone the Repository

```bash
git clone https://github.com/cowhorse05/LabOps.git /opt/labops
cd /opt/labops
```

### 3.2 Create the `.env` File

```bash
cp .env.example .env
chmod 600 .env
```

Now edit `.env` with your actual values. Generate random secrets as needed:

```bash
# Generate random passwords
openssl rand -base64 16   # For MySQL passwords
openssl rand -base64 32   # For encryption key
```

### 3.3 `.env` Variable Reference

The `.env.example` template requires you to replace every `CHANGE_ME` value:

```env
# Your domain or public IP (used in Nginx TLS config and CORS)
SERVER_HOST=cowhorse.xyz

# Docker image tag (use a release tag like "v0.2.0" or "dev")
LABOPS_VERSION=dev

# MySQL credentials — NEVER use the same password for both
MYSQL_ROOT_PASSWORD=<random-base64-16>
MYSQL_PASSWORD=<random-base64-16>

# MySQL connection string — update the password to match MYSQL_PASSWORD above
# The hostname "mysql" is the Docker Compose service name (internal DNS)
LABOPS_MYSQL_DSN=labops:<MYSQL_PASSWORD>@tcp(mysql:3306)/labops?parseTime=true&charset=utf8mb4

# Bootstrap admin password — used ONLY when the users table is empty
# Must be at least 12 characters. Remove from .env after first login.
LABOPS_BOOTSTRAP_ADMIN_PASSWORD=<strong-password-at-least-12-chars>

# Encryption key — base64 encoding of exactly 32 random bytes
# Used to encrypt LLM API keys at rest with AES-256-GCM
LABOPS_ENCRYPTION_KEY=<base64-32-random-bytes>
```

**PowerShell generation (if generating on Windows):**
```powershell
[Convert]::ToBase64String([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
```

### 3.4 Validate Configuration

```bash
docker compose config --quiet
```

No output means the configuration is valid.

---

## 4. Obtain SSL Certificate

LabOps uses Let's Encrypt for TLS certificates. The certificate is obtained on the host and mounted into the Nginx container.

### 4.1 Install Certbot

```bash
sudo snap install core
sudo snap refresh core
sudo snap install --classic certbot
```

### 4.2 Obtain Certificate

> **Important:** Port 80 must be accessible from the internet for HTTP-01 validation. Ensure no other service is binding to port 80.

```bash
# For domain-based certificates (recommended):
sudo /snap/bin/certbot certonly --standalone -d cowhorse.xyz

# If using certificate for both apex and www:
sudo /snap/bin/certbot certonly --standalone -d cowhorse.xyz -d www.cowhorse.xyz
```

On first run, enter your email address for expiry notifications and agree to the terms of service.

Success output:
```
Successfully received certificate.
Certificate is saved at: /etc/letsencrypt/live/cowhorse.xyz/fullchain.pem
Key is saved at:         /etc/letsencrypt/live/cowhorse.xyz/privkey.pem
```

### 4.3 Certificate Paths

The Docker Compose mounts these paths into the Nginx container:

| Host Path | Container Mount | Purpose |
|-----------|----------------|---------|
| `/etc/letsencrypt/live/cowhorse.xyz/fullchain.pem` | Read-only | TLS certificate chain |
| `/etc/letsencrypt/live/cowhorse.xyz/privkey.pem` | Read-only | TLS private key |
| `./deploy/acme-webroot` | `/var/www/certbot` | ACME challenge webroot |

### 4.4 Automatic Renewal

Certbot snap installs a systemd timer for auto-renewal:

```bash
# Verify the timer is active
sudo systemctl status snap.certbot.renew.timer

# Test renewal (dry run)
sudo /snap/bin/certbot renew --dry-run
```

> **Note:** For renewal to work, port 80 must remain available. The renewal process uses the standalone method by default. If your Nginx is running, you may need a pre-hook to stop it temporarily, or switch to the webroot method.

### 4.5 Certificate Renewal with Nginx Running

Create a deploy hook so Nginx reloads after certificate renewal:

```bash
# Create a renewal hook
sudo mkdir -p /etc/letsencrypt/renewal-hooks/deploy

sudo tee /etc/letsencrypt/renewal-hooks/deploy/labops-nginx-reload.sh << 'EOF'
#!/bin/bash
cd /opt/labops && docker compose exec -T web nginx -s reload
EOF

sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/labops-nginx-reload.sh
```

---

## 5. Build & Start Services

### 5.1 Build Docker Images

```bash
cd /opt/labops
docker compose build
```

This builds three images:
- `labops-server` — Go API server (multi-stage, ~20MB final)
- `labops-web` — Nginx + React build (~30MB final)
- MySQL uses the official `mysql:8.0` image (pulled, not built)

First build takes 3-5 minutes. Subsequent builds are faster due to Docker layer caching.

### 5.2 Start Services

```bash
docker compose up -d
```

### 5.3 Verify All Services are Healthy

```bash
docker compose ps
```

Expected output — all services with `Up` status and `(healthy)`:

```
NAME                STATUS                    PORTS
labops-mysql-1      Up 30 seconds (healthy)   3306/tcp
labops-server-1     Up 20 seconds (healthy)   8080/tcp
labops-web-1        Up 10 seconds (healthy)   0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
```

### 5.4 Verify API Health

```bash
# Local check (on the server)
curl -f http://localhost:8080/api/health

# Public check (from anywhere)
curl -f https://cowhorse.xyz/api/health
```

Expected response: `{"status":"ok"}`

### 5.5 Inspect Logs (if something is wrong)

```bash
docker compose logs server   # Server logs
docker compose logs mysql    # Database logs
docker compose logs web      # Nginx logs
docker compose logs -f       # All logs, follow mode
```

---

## 6. First Login & Admin Setup

### 6.1 Login

1. Open `https://cowhorse.xyz` in your browser
2. Log in with:
   - **Username:** `admin`
   - **Password:** the value of `LABOPS_BOOTSTRAP_ADMIN_PASSWORD` from `.env`

### 6.2 Change Password (Forced)

On first login, you will be forced to change your password. The new password must:
- Be at least 12 characters
- Differ from the bootstrap password

### 6.3 Secure the Bootstrap Password

After successful login and password change, remove the bootstrap password from `.env`:

```bash
# Edit .env and delete or comment out the line:
# LABOPS_BOOTSTRAP_ADMIN_PASSWORD=...
```

The bootstrap password is consumed — it won't be used again unless you delete all users from the database.

### 6.4 Create Additional Users (Optional)

Navigate to **用户管理** (User Management) page to create:
- **operator** accounts — can view devices and execute template commands
- **viewer** accounts — read-only access

---

## 7. Install Agents on Target Machines

Agents are installed on machines you want to monitor and manage. Each agent gets a unique per-device credential via a one-time enrollment code.

### 7.1 Build the Agent Binary

On the LabOps server (or your development machine), build the agent binary for the target platform:

```bash
cd /opt/labops/agent

# For Linux amd64 (most common)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o labops-agent ./cmd/agent

# For Linux arm64 (Raspberry Pi, ARM servers)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o labops-agent ./cmd/agent

# For Windows amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o labops-agent.exe ./cmd/agent
```

> **Note:** Go must be installed on the build machine (`sudo snap install go --classic` or install via apt).

### 7.2 Create an Enrollment Code

1. In the LabOps Web Console, navigate to **设备接入** (Enrollment)
2. Click **生成接入码** (Generate Enrollment Code)
3. Configure:
   - **有效期** (Validity): 10-60 minutes (the code is one-time use)
   - **最大使用次数** (Max Uses): 1-20
4. Copy the generated code — it will only be shown once

### 7.3 Install the Agent (Linux, Automated)

The `scripts/install-agent.sh` script automates the entire agent installation:

```bash
# Copy the agent binary to the target machine first, then:
sudo bash /opt/labops/scripts/install-agent.sh \
  --server "https://cowhorse.xyz" \
  --enroll-code "<ONE_TIME_CODE>" \
  --name "$(hostname)" \
  --group "homelab" \
  --binary ./labops-agent
```

**What the script does:**

1. Creates `labops-agent` system user (no login shell)
2. Creates directories:
   - `/etc/labops-agent/` — configuration and credentials (0750)
   - `/var/lib/labops-agent/` — runtime data (0750)
3. Copies the agent binary to `/usr/local/bin/labops-agent`
4. Runs enrollment: `labops-agent --enroll-only --enroll-code <CODE> --server https://cowhorse.xyz`
   - Agent calls `POST /api/agent/enroll` with the code, device profile, and `--real` flag
   - Receives per-device `deviceId` + `deviceSecret`
   - Saves credentials to `/etc/labops-agent/credentials.json` (0640, owned by root:labops-agent)
5. Creates `/etc/labops-agent/agent.env`:
   ```env
   LABOPS_SERVER_URL=https://cowhorse.xyz
   LABOPS_AGENT_NAME=<name>
   LABOPS_AGENT_GROUP=<group>
   ```
6. Installs systemd unit from `deploy/systemd/labops-agent.service`
7. Enables and starts the service

### 7.4 Systemd Unit Security Hardening

The agent's systemd unit (`deploy/systemd/labops-agent.service`) applies these security restrictions:

| Setting | Value | Purpose |
|---------|-------|---------|
| `User` / `Group` | `labops-agent` | Dedicated non-root user |
| `NoNewPrivileges` | `true` | Prevent privilege escalation |
| `PrivateTmp` | `true` | Isolated /tmp directory |
| `ProtectSystem` | `strict` | Read-only access to /usr, /boot, /etc |
| `ProtectHome` | `true` | No access to /home, /root |
| `ProtectKernelTunables` | `true` | No kernel parameter modification |
| `ProtectKernelModules` | `true` | No module loading |
| `ProtectControlGroups` | `true` | No cgroup manipulation |
| `RestrictSUIDSGID` | `true` | No setuid/setgid |
| `LockPersonality` | `true` | No personality() syscall |
| `RestrictRealtime` | `true` | No real-time scheduling |
| `ReadWritePaths` | `/var/lib/labops-agent` | Only writable path |

### 7.5 Verify Agent is Connected

```bash
# Check service status
sudo systemctl status labops-agent

# Follow agent logs
sudo journalctl -u labops-agent -f
```

Expected log output:
```
agent connecting to https://cowhorse.xyz as my-server
agent registered with server
```

In the Web Console → **设备管理** (Devices), the new device should appear with status **在线** (Online).

### 7.6 Manual Agent Installation (Linux)

If you prefer to install manually:

```bash
# 1. Create directories
sudo mkdir -p /etc/labops-agent /var/lib/labops-agent
sudo chmod 750 /etc/labops-agent /var/lib/labops-agent

# 2. Copy binary
sudo cp labops-agent /usr/local/bin/
sudo chmod 755 /usr/local/bin/labops-agent

# 3. Enroll
sudo /usr/local/bin/labops-agent \
  --enroll-only \
  --enroll-code "<CODE>" \
  --server "https://cowhorse.xyz" \
  --name "$(hostname)" \
  --group "homelab" \
  --real \
  --credentials /etc/labops-agent/credentials.json

# 4. Run agent directly (foreground, for testing)
sudo /usr/local/bin/labops-agent \
  --server "https://cowhorse.xyz" \
  --name "$(hostname)" \
  --group "homelab" \
  --real
```

### 7.7 Agent Installation (Windows)

```powershell
# 1. Build agent for Windows
cd agent
$env:CGO_ENABLED = "0"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -ldflags="-s -w" -o labops-agent.exe .\cmd\agent\

# 2. Create directories
New-Item -ItemType Directory -Force -Path "$env:ProgramData\LabOps"

# 3. Enroll (one-time)
.\labops-agent.exe `
  --enroll-only `
  --enroll-code "<CODE>" `
  --server "https://cowhorse.xyz" `
  --name $env:COMPUTERNAME `
  --group "homelab" `
  --real

# 4. Run agent (foreground)
.\labops-agent.exe `
  --server "https://cowhorse.xyz" `
  --name $env:COMPUTERNAME `
  --group "homelab" `
  --real
```

For production Windows services, wrap the agent with [NSSM](https://nssm.cc/).

### 7.8 Agent CLI Reference

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--server` | `LABOPS_SERVER_URL` | `http://localhost:8080` | LabOps server URL |
| `--token` | `LABOPS_AGENT_TOKEN` | (none) | Legacy shared token |
| `--device-secret` | `LABOPS_DEVICE_SECRET` | (none) | Per-device secret |
| `--enroll-code` | `LABOPS_ENROLLMENT_CODE` | (none) | One-time enrollment code |
| `--credentials` | `LABOPS_AGENT_CREDENTIALS` | `/etc/labops-agent/credentials.json` (Linux) or `%ProgramData%\LabOps\credentials.json` (Windows) | Credentials file path |
| `--name` | `LABOPS_AGENT_NAME` | OS hostname | Device display name |
| `--group` | `LABOPS_AGENT_GROUP` | `default` | Device group |
| `--id` | `LABOPS_AGENT_ID` | `agent-<name>` (sanitized) | Stable agent ID |
| `--mock-profile` | `LABOPS_MOCK_PROFILE` | `ubuntu` | Mock profile: `ubuntu`, `windows-lab`, `server`, `edge-node` |
| `--real` | `LABOPS_AGENT_REAL` | false | Use real system metrics (gopsutil v4) |
| `--enroll-only` | (none) | false | Enroll, save credentials, and exit |

---

## 8. Revoke & Uninstall Agents

### 8.1 Revoke Device Credential

1. In the Web Console → **设备管理** (Devices), find the device
2. Click **吊销凭据** (Revoke Credential)
3. Confirm the action

Revocation immediately invalidates the device's secret — the agent will fail to reconnect and log errors.

### 8.2 Uninstall Agent (Linux)

```bash
sudo bash /opt/labops/scripts/uninstall-agent.sh
```

This stops the service, removes the systemd unit, deletes `/etc/labops-agent/`, and removes the binary from `/usr/local/bin/`.

### 8.3 Manual Cleanup (Linux)

```bash
sudo systemctl stop labops-agent
sudo systemctl disable labops-agent
sudo rm -f /etc/systemd/system/labops-agent.service
sudo systemctl daemon-reload
sudo rm -rf /etc/labops-agent /var/lib/labops-agent
sudo rm -f /usr/local/bin/labops-agent
```

---

## 9. Backup & Restore

### 9.1 Manual Backup

```bash
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
```

**What the script does:**
- Runs `docker compose exec -T mysql mysqldump --all-databases`
- Compresses output with gzip
- Names: `labops-YYYY-MM-DD.sql.gz`
- On Sundays: also saves as `labops-weekly-YYYY-MM-DD.sql.gz`
- Retains: 7 daily + 4 weekly backups (older ones auto-deleted)

### 9.2 Automated Daily Backup

Install the backup systemd timer:

```bash
sudo install -m 0644 deploy/systemd/labops-backup.service deploy/systemd/labops-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now labops-backup.timer
```

The timer fires daily at **03:15 UTC** with a 15-minute randomized delay (`RandomizedDelaySec=900`).

Check timer status:

```bash
sudo systemctl status labops-backup.timer
sudo systemctl list-timers labops-backup.timer
```

### 9.3 Restore from Backup

> **WARNING:** Restore overwrites all current data. Test against a separate database first.

```bash
# The safety guard requires explicit confirmation
sudo LABOPS_BOOTSTRAP_ADMIN_PASSWORD=<current-admin-password> \
     LABOPS_CONFIRM_RESTORE=RESTORE \
     bash scripts/restore.sh /var/backups/labops/<backup-file>.sql.gz
```

**Restore process:**
1. Validates that `LABOPS_CONFIRM_RESTORE=RESTORE` is set
2. Stops the server container (but keeps MySQL running)
3. Drops and recreates the LabOps database
4. Restores from the gzipped SQL dump
5. Restarts the server container

---

## 10. Upgrading & Rollback

### 10.1 Upgrade to a New Version

```bash
cd /opt/labops
git pull origin master
# Update LABOPS_VERSION in .env if using release tags
docker compose build
docker compose up -d
```

### 10.2 Rollback

```bash
# Rollback to a specific version tag
LABOPS_VERSION=<previous-tag> docker compose up -d

# Or use git to checkout a previous commit
git checkout <previous-commit>
docker compose build
docker compose up -d
```

> **Important:** Always back up the database before upgrading. Restore from backup if the upgrade corrupts data.

---

## 11. Troubleshooting

### Health Check Failures

**Server health check failing:**
```bash
docker compose logs server
# Common causes: MySQL not ready, invalid DSN, missing encryption key
```

**MySQL health check failing:**
```bash
docker compose logs mysql
# Common causes: port conflict (3306 already in use), disk full, data corruption
```

**Web (Nginx) health check failing:**
```bash
docker compose logs web
# Common causes: certificate files not found at mount path, port 80/443 conflict
```

### Agent Won't Connect

1. **Check credentials exist:**
   ```bash
   sudo cat /etc/labops-agent/credentials.json
   ```
   Should contain `deviceId` and `deviceSecret`.

2. **Check server URL:**
   ```bash
   sudo cat /etc/labops-agent/agent.env | grep LABOPS_SERVER_URL
   ```
   Must use `https://` (not `http://`) for production.

3. **Check service status:**
   ```bash
   sudo systemctl status labops-agent
   sudo journalctl -u labops-agent -n 50
   ```

4. **Test WebSocket connectivity from agent machine:**
   ```bash
   curl -f https://cowhorse.xyz/api/health
   ```

5. **Check if device is revoked in Web Console** — revoked credentials cannot reconnect.

### Certificate Issues

**Certificate expired or not found:**
```bash
sudo ls -la /etc/letsencrypt/live/cowhorse.xyz/
# Should show: fullchain.pem, privkey.pem

# Test renewal
sudo /snap/bin/certbot renew --dry-run
```

**Nginx can't read certificates:**
```bash
# Certificates are mounted read-only from host. Check permissions:
sudo ls -la /etc/letsencrypt/live/cowhorse.xyz/
# Directories in the chain must be executable by the container user (uid 101 = nginx)
```

### MySQL Connection Errors

```bash
# Check MySQL is running
docker compose ps mysql

# Check MySQL logs
docker compose logs mysql

# Test MySQL connection from server container
docker compose exec server wget -qO- http://localhost:8080/api/health

# Verify DSN format — must use hostname "mysql" (Docker service name), not "localhost"
echo $LABOPS_MYSQL_DSN
# Correct: labops:password@tcp(mysql:3306)/labops?parseTime=true&charset=utf8mb4
```

### Port Conflicts

```bash
# Check what's using port 80 or 443
sudo ss -tlnp | grep -E ':80 |:443 '

# Stop any conflicting service (e.g., existing nginx, apache2)
sudo systemctl stop nginx
sudo systemctl disable nginx
```

### Reset Everything

```bash
# Stop and remove everything (WARNING: deletes database and volumes!)
docker compose down -v

# Delete persistent data
sudo rm -rf /var/backups/labops/*

# Re-run from Step 3 (Clone & Configure)
```

### Graceful Restart of a Single Service

```bash
docker compose restart server   # Restart just the Go server
docker compose restart web      # Restart just Nginx
docker compose restart mysql    # Restart MySQL (takes ~5s to become healthy)
```

---

## Architecture Recap: What's Running

After successful deployment, your server runs:

```
┌─────────────────────────────────────────────────┐
│                  Docker Host                      │
│                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │  mysql   │  │  server  │  │  web (Nginx) │   │
│  │  :3306   │  │  :8080   │  │  :80, :443   │   │
│  │ internal │◀─│ internal │◀─│ external     │   │
│  └──────────┘  └──────────┘  └──────────────┘   │
│       │              │               │            │
│       └──── backend network ────────┘            │
│                      │                            │
│               egress network                      │
│               (LLM API outbound)                  │
└─────────────────────────────────────────────────┘
        ▲                              ▲
        │                              │
   Agent machines              Users (browsers)
   (WebSocket WSS)             (HTTPS)
```

All internal communication (mysql ↔ server, server ↔ web) happens over the private `backend` Docker network. Only port 443 (HTTPS/WSS) is exposed to the internet.
