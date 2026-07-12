# 服务器迁移

> **目标读者:** 运维人员
> **前置阅读:** [备份与恢复](backup-restore.md)、[数据持久化](data-storage.md)

---

## 迁移概述

服务器迁移是将 LabOps 从旧服务器迁移到新服务器。整个过程的关键是**数据完整性**——迁移完成后，所有设备、任务、审计日志、用户信息必须完整保留。

---

## 迁移流程图

```
旧服务器                新服务器
   │                      │
   │ 1. 停止写入           │
   │ docker compose stop   │
   │   server             │
   │                      │
   │ 2. 备份数据库         │
   │ mysqldump → .sql.gz │
   │                      │
   │ 3. 备份配置文件       │
   │ .env、证书            │
   │                      │
   │ 4. 传输到新服务器     │
   │ scp / rsync         │ ──► 接收文件
   │                      │
   │                      │ 5. 在新服务器部署项目
   │                      │ git clone → .env → docker compose
   │                      │
   │                      │ 6. 恢复数据库
   │                      │ gzip -dc → mysql
   │                      │
   │                      │ 7. 配置 Nginx + HTTPS
   │                      │
   │                      │ 8. 验证
   │                      │ 登录 → 检查数据 → Agent 重连
   │                      │
   │ 9. 修改 DNS（如果需要）│
   │                      │
   │ 10. 保留旧服务器       │
   │ 用于紧急回滚          │
```

---

## 步骤 1：停止写入（旧服务器）

```bash
cd /opt/cowhorse/source
docker compose stop server
```

只停止 Server 容器（Agent 不需要管——它们会自动重连到新服务器或保持断线等待）。MySQL 保持运行以执行备份。

---

## 步骤 2：备份（旧服务器）

```bash
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
cp .env /var/backups/labops/env-migration.bak
```

---

## 步骤 3：传输（旧 → 新）

```bash
# 传输备份文件
scp /var/backups/labops/labops-最新备份.sql.gz \
    admin@新服务器IP:/tmp/

# 传输 .env（需要修改 SERVER_HOST）
scp /opt/cowhorse/source/.env \
    admin@新服务器IP:/tmp/env-migration

# （可选）传输 TLS 证书
sudo tar czf /tmp/letsencrypt-backup.tar.gz /etc/letsencrypt/
scp /tmp/letsencrypt-backup.tar.gz admin@新服务器IP:/tmp/
```

---

## 步骤 4：部署（新服务器）

按照 [服务器部署教程](../deployment/server-deployment.md) 在新服务器上完成基本部署。

修改 `.env` 中的 `SERVER_HOST` 为新服务器的域名或 IP。

---

## 步骤 5：恢复数据（新服务器）

```bash
sudo LABOPS_CONFIRM_RESTORE=RESTORE \
  bash scripts/restore.sh /tmp/备份文件.sql.gz
```

---

## 步骤 6：配置 Nginx + HTTPS

```bash
sudo certbot --nginx -d 新域名
sudo nginx -t && sudo systemctl reload nginx
```

---

## 步骤 7：验证

```bash
# 检查数据完整性
curl -H "Cookie: ..." https://新域名/api/devices | jq length
# 设备数量应与旧服务器一致

# 检查用户是否可以登录
# 浏览器打开 https://新域名 → 用旧账号密码登录
```

---

## 步骤 8：修改 DNS

将域名 A 记录从旧 IP 改为新 IP。

TTL 建议先设为 600（10 分钟）以便快速生效。

---

## 步骤 9：Agent 重连

如果 SERVER_HOST 变了，Agent 需要更新配置：

```bash
# 在每台 Agent 设备上
sudo sed -i 's|旧域名|新域名|g' /etc/labops-agent/agent.env
sudo systemctl restart labops-agent
```

如果 SERVER_HOST 不变（只改了 IP），Agent 会自动重连——它们使用域名，DNS 更新后会自动指向新 IP。

---

## 回滚计划

如果在迁移后发现问题：

1. 将 DNS 改回旧服务器 IP
2. 在旧服务器上重启 Server：`docker compose start server`
3. Agent 会自动重连到旧服务器

保留旧服务器至少 48 小时（覆盖 DNS 缓存时间）。
