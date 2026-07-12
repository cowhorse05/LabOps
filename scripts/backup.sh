#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BACKUP_DIR="${LABOPS_BACKUP_DIR:-/var/backups/labops}"
mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
docker compose exec -T mysql sh -c 'exec mysqldump --single-transaction --routines --triggers -uroot -p"$MYSQL_ROOT_PASSWORD" labops' | gzip -9 > "$BACKUP_DIR/labops-$STAMP.sql.gz"
gzip -t "$BACKUP_DIR/labops-$STAMP.sql.gz"
if [[ "$(date -u +%u)" == "7" ]]; then
  cp "$BACKUP_DIR/labops-$STAMP.sql.gz" "$BACKUP_DIR/labops-weekly-$STAMP.sql.gz"
fi
find "$BACKUP_DIR" -type f -name 'labops-[0-9]*.sql.gz' -mtime +7 -delete
find "$BACKUP_DIR" -type f -name 'labops-weekly-*.sql.gz' -mtime +28 -delete
echo "$BACKUP_DIR/labops-$STAMP.sql.gz"
