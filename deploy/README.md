# LabOps Ubuntu 22.04 production deployment

This guide targets one amd64 Ubuntu 22.04 test server. Do not run the commands until the operator has reviewed the ports, data paths, and rollback described below.

## Impact and rollback

- Opens TCP 80 for ACME validation/redirect and TCP 443 for HTTPS/WSS. MySQL and port 8080 remain private to Docker.
- Creates Docker volumes, `/etc/letsencrypt`, `/var/backups/labops`, and optionally a `labops-agent` system user.
- Does not alter SSH configuration or delete existing databases.
- Roll back the application with `LABOPS_VERSION=<previous-tag> docker compose up -d`; restore data only from a verified pre-upgrade backup.

## 1. Prepare secrets

```bash
cp .env.example .env
chmod 600 .env
# Edit every CHANGE_ME value. SERVER_HOST may be a public IPv4 address.
docker compose config --quiet
```

Generate the encryption key without printing unrelated environment data:

```bash
openssl rand -base64 32
```

## 2. Obtain the initial IP certificate

Ports 80/443 must be allowed in the cloud security group before this step. Certbot 5.4+ is required for public IP certificates.

```bash
sudo snap install core
sudo snap refresh core
sudo snap install --classic certbot
sudo /snap/bin/certbot certonly --standalone --preferred-profile shortlived --ip-address "${SERVER_HOST}"
```

Run once with `--staging` first if desired. IP certificates last about six days, so confirm the snap Certbot timer is active and install a deploy hook that runs `docker compose exec -T web nginx -s reload` from the LabOps directory.

## 3. Back up and deploy

```bash
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh  # existing installations only
docker compose build
docker compose up -d
docker compose ps
curl --fail "https://${SERVER_HOST}/api/health"
```

The bootstrap password is consumed only when no users exist or when replacing the legacy `admin/admin` password. Change it after first login and remove it from `.env` once a non-default administrator exists.

## 4. Enroll the host agent

Build the Linux binary, create a one-time code in **设备接入**, then install:

```bash
cd agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o labops-agent ./cmd/agent
cd ..
sudo bash scripts/install-agent.sh --server "https://${SERVER_HOST}" --enroll-code "${ONE_TIME_CODE}" --name "$(hostname)" --group homelab --binary agent/labops-agent
```

Revoke the device in the Web console before uninstalling it:

```bash
sudo bash scripts/uninstall-agent.sh
```

## 5. Backup and restore checks

Daily backups retain seven daily and four weekly copies. Restore is deliberately guarded:

```bash
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
sudo LABOPS_CONFIRM_RESTORE=RESTORE bash scripts/restore.sh /var/backups/labops/<verified-backup>.sql.gz
```

Test restore against a separate disposable database before relying on a backup.

For an installation at `/opt/labops`, enable the supplied timer:

```bash
sudo install -m 0644 deploy/systemd/labops-backup.service deploy/systemd/labops-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now labops-backup.timer
sudo systemctl start labops-backup.service
sudo systemctl status labops-backup.service
```
