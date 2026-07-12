#!/usr/bin/env bash
set -euo pipefail
[[ "${EUID}" -eq 0 ]] || { echo "Run as root with sudo" >&2; exit 1; }
systemctl disable --now labops-agent 2>/dev/null || true
rm -f /etc/systemd/system/labops-agent.service /usr/local/bin/labops-agent
systemctl daemon-reload
echo "Service removed. Credentials remain in /etc/labops-agent; revoke the device in LabOps before deleting them."
