#!/usr/bin/env bash
set -euo pipefail
[[ $# -eq 1 ]] || { echo "Usage: sudo $0 /path/to/labops-backup.sql.gz" >&2; exit 2; }
BACKUP="$1"
[[ -f "$BACKUP" ]] || { echo "Backup not found: $BACKUP" >&2; exit 2; }
gzip -t "$BACKUP"
echo "Restore replaces the current labops database. Set LABOPS_CONFIRM_RESTORE=RESTORE to continue."
[[ "${LABOPS_CONFIRM_RESTORE:-}" == "RESTORE" ]] || exit 1
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
gzip -dc "$BACKUP" | docker compose exec -T mysql sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" labops'
