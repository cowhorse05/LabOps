# 数据持久化详解

> **目标读者:** 需要理解数据存储位置的运维人员
> **前置阅读:** 项目已部署

---

## "数据保存在主机本地"意味着什么

LabOps 的所有数据都存储在部署中央服务的那台服务器上。具体来说：

- **MySQL 数据库文件** → Docker Volume `mysql-data`（容器内 `/var/lib/mysql`）
- **TLS 证书** → 宿主机 `/etc/letsencrypt/`
- **Agent 凭据** → 每台被管理设备的 `/etc/labops-agent/credentials.json`
- **备份文件** → 宿主机 `/var/backups/labops/`

**这意味着：**

- ✅ 你的数据不会被发送到任何第三方服务
- ✅ 你拥有数据的完全控制权
- ✅ 你可以随时备份、迁移数据
- ⚠️ 你有责任自己做好备份

---

## 每个数据的存储路径

### Docker Compose 部署

| 数据类型 | 宿主机路径 | 容器内路径 | 必须备份 | 丢失影响 |
|---------|-----------|-----------|:------:|---------|
| MySQL 数据库 | Docker Volume `mysql-data` | `/var/lib/mysql` | ✅ | **全部业务数据丢失**（设备、任务、审计、用户） |
| TLS 证书 | `/etc/letsencrypt/` | `/etc/letsencrypt/` (:ro) | 建议 | HTTPS 失效，需重新申请 |
| ACME webroot | `./deploy/acme-webroot/` | `/var/www/certbot` | 否 | 证书续期需重新验证 |
| Agent 凭据 | `/etc/labops-agent/credentials.json` | 无（在 Agent 设备上） | 建议 | Agent 需重新接入 |
| Agent 配置 | `/etc/labops-agent/agent.env` | 无（在 Agent 设备上） | 否 | 可手动重建 |
| .env 文件 | `./.env` | 无（只有宿主机） | ✅ | **服务无法启动** |
| 备份 | `/var/backups/labops/` | 无（只有宿主机） | — | 无法恢复数据 |

### 原生 Linux 部署

| 数据类型 | 路径 | 必须备份 |
|---------|------|:------:|
| MySQL/SQLite 数据 | `/var/lib/labops/` | ✅ |
| 配置文件 | `/etc/labops/env` | ✅ |
| Server 二进制 | `/usr/local/bin/labops-server` | 否（可重新编译） |
| Agent 二进制 | `/usr/local/bin/labops-agent` | 否（可重新编译） |

---

## Docker 操作对数据的影响

### ✅ 安全的操作（不会删除数据）

```bash
docker compose stop        # 停止容器，Volume 保留
docker compose restart     # 重启容器，Volume 保留
docker compose down        # 停止并删除容器，Volume 保留
docker compose up -d       # 重新创建容器，使用已有 Volume
```

### ⚠️ 危险的操作（会删除数据）

```bash
docker compose down -v     # ⚠️ 删除所有 Volume！数据库永久丢失！
docker volume rm mysql-data # ⚠️ 删除数据库 Volume！
docker system prune -a --volumes  # ⚠️ 删除所有未使用的 Volume！
```

### 如何判断数据是否真正持久化

```bash
# 1. 查看 Volume 信息
docker volume inspect labops_mysql-data

# 输出中 Mountpoint 字段显示 Volume 在宿主机上的实际路径（通常在 /var/lib/docker/volumes/）

# 2. 确认 Volume 存在
docker volume ls | grep mysql-data

# 3. 测试：停止容器，删除容器，重新创建，检查数据是否还在
docker compose down        # 注意：不要加 -v
docker compose up -d       # 重新创建
# → 登录 Web 界面，确认设备、任务、用户数据都在
```

---

## 更换服务器前需要迁移的文件

| 文件/目录 | 迁移方式 | 优先级 |
|----------|---------|:----:|
| MySQL 数据库 | mysqldump → .sql.gz → 传输到新服务器 → 恢复 | 🔴 必须 |
| TLS 证书 | 复制 `/etc/letsencrypt/` 或在新服务器重新申请 | 🟡 建议 |
| .env 文件 | 复制到新服务器（修改 SERVER_HOST） | 🔴 必须 |
| Agent 凭据 | 在每个 Agent 设备上重新接入（或复制凭据文件） | 🟡 建议 |

**不迁移证书的做法：** 在新服务器上重新执行 `certbot --nginx` 申请新证书即可。旧证书不需要迁移。

---

## 备份验证

备份不仅是生成文件，还需要验证文件可以恢复：

```bash
# 1. 查看备份文件大小
ls -lh /var/backups/labops/

# 2. 检查备份文件内容（不解压查看）
gzip -t /var/backups/labops/labops-*.sql.gz
# 如果没有报错，说明文件完整

# 3. 查看备份内容（前 50 行）
zcat /var/backups/labops/labops-*.sql.gz | head -50
# 应该看到 CREATE TABLE 和 INSERT 语句
```

详见 [备份与恢复](backup-restore.md)。
