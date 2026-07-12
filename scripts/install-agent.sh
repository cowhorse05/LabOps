#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="${LABOPS_AGENT_BINARY:-$ROOT/agent/labops-agent}"
SERVER_URL=""
ENROLLMENT_CODE=""
AGENT_NAME="$(hostname)"
AGENT_GROUP="default"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server) SERVER_URL="$2"; shift 2 ;;
    --enroll-code) ENROLLMENT_CODE="$2"; shift 2 ;;
    --name) AGENT_NAME="$2"; shift 2 ;;
    --group) AGENT_GROUP="$2"; shift 2 ;;
    --binary) BINARY="$2"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

[[ "${EUID}" -eq 0 ]] || { echo "Run as root with sudo" >&2; exit 1; }
[[ -n "$SERVER_URL" ]] || { echo "--server is required" >&2; exit 2; }
[[ -x "$BINARY" ]] || { echo "Agent binary not found or executable: $BINARY" >&2; exit 2; }

id -u labops-agent >/dev/null 2>&1 || useradd --system --home /var/lib/labops-agent --shell /usr/sbin/nologin labops-agent
install -d -o root -g labops-agent -m 0750 /etc/labops-agent
install -d -o labops-agent -g labops-agent -m 0750 /var/lib/labops-agent
install -o root -g root -m 0755 "$BINARY" /usr/local/bin/labops-agent

if [[ ! -f /etc/labops-agent/credentials.json ]]; then
  [[ -n "$ENROLLMENT_CODE" ]] || { echo "--enroll-code is required for first installation" >&2; exit 2; }
  /usr/local/bin/labops-agent --server="$SERVER_URL" --name="$AGENT_NAME" --group="$AGENT_GROUP" --real \
    --enroll-code="$ENROLLMENT_CODE" --enroll-only --credentials=/etc/labops-agent/credentials.json
fi
chown root:labops-agent /etc/labops-agent/credentials.json
chmod 0640 /etc/labops-agent/credentials.json

install -o root -g root -m 0644 "$ROOT/deploy/systemd/labops-agent.service" /etc/systemd/system/labops-agent.service
printf 'LABOPS_SERVER_URL=%q\nLABOPS_AGENT_NAME=%q\nLABOPS_AGENT_GROUP=%q\n' "$SERVER_URL" "$AGENT_NAME" "$AGENT_GROUP" > /etc/labops-agent/agent.env
chown root:labops-agent /etc/labops-agent/agent.env
chmod 0640 /etc/labops-agent/agent.env
systemctl daemon-reload
systemctl enable --now labops-agent
systemctl --no-pager --full status labops-agent
