# 备份与恢复

> **目标读者:** 运维人员
> **前置阅读:** [数据持久化](data-storage.md)

---

## 备份什么

| 备份内容 | 备份方式 | 频率建议 |
|---------|---------|:-------:|
| MySQL 数据库 | mysqldump → gzip | 每天 |
| .env 配置 | 复制文件 | 每次修改后 |
| Agent 凭据 | 复制文件 | 每次新增设备后 |
| TLS 证书 | 可选（可重新申请） | — |

---

## 手动备份

### 备份 MySQL 数据库

```bash
cd /opt/cowhorse/source
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
```

**这条命令做了什么：**
1. 通过 `docker compose exec mysql` 执行 `mysqldump --all-databases`
2. 将输出通过 gzip 压缩
3. 保存到 `/var/backups/labops/labops-YYYYMMDDTHHMMSSZ.sql.gz`
4. 如果是周六，额外保存一份周备份（保留 4 周）
5. 删除 7 天前的日备份

### 备份 .env 配置

```bash
cp /opt/cowhorse/source/.env /var/backups/labops/env-$(date +%Y%m%d).bak
```

### 备份 Agent 凭据

```bash
# 在 Agent 所在设备上执行
sudo cp /etc/labops-agent/credentials.json /var/backups/labops/credentials-$(hostname).json
```

---

## 自动备份

### 使用 systemd Timer

部署备份定时器（每天 03:15 UTC 执行）：

```bash
sudo cp deploy/systemd/labops-backup.service /etc/systemd/system/
sudo cp deploy/systemd/labops-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now labops-backup.timer
```

**验证定时器：**

```bash
sudo systemctl status labops-backup.timer
sudo systemctl list-timers labops-backup.timer
```

### 使用 Cron

```bash
# 编辑 root 的 crontab
sudo crontab -e

# 添加：每天凌晨 3 点备份
0 3 * * * cd /opt/cowhorse/source && LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
```

---

## 恢复

### ⚠️ 恢复前须知

- 恢复会**覆盖**当前数据库的全部数据
- 恢复前先备份当前数据（即使数据是错的，也比没有好）
- 建议先在测试环境验证备份文件可恢复

### 恢复步骤

```bash
# 安全确认：必须设置环境变量才能恢复（防止误操作）
sudo LABOPS_CONFIRM_RESTORE=RESTORE \
  bash scripts/restore.sh /var/backups/labops/要恢复的备份文件.sql.gz
```

**恢复过程：**
1. 验证确认环境变量
2. 停止 Server 容器
3. 删除现有数据库
4. 从备份文件恢复
5. 重启 Server 容器

### 手动恢复（不使用脚本）

```bash
# 1. 停止 Server
docker compose stop server

# 2. 恢复数据库
gzip -dc 备份文件.sql.gz | \
  docker compose exec -T mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" labops

# 3. 重启 Server
docker compose start server
```

---

## 恢复演练

定期验证备份可以恢复：

### 演练步骤

```bash
# 1. 在另一个位置创建测试数据库
docker compose exec mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "CREATE DATABASE labops_restore_test;"

# 2. 恢复备份到测试数据库
gzip -dc /var/backups/labops/labops-最新的备份.sql.gz | \
  docker compose exec -T mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" labops_restore_test

# 3. 验证数据完整性
docker compose exec mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e \
  "SELECT COUNT(*) FROM labops_restore_test.devices;"

# 4. 清理测试数据库
docker compose exec mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "DROP DATABASE labops_restore_test;"
```

### 成功标准

- 恢复后设备数量和备份时一致
- 恢复后用户可以正常登录
- 恢复后 Agent 可以重新连接（凭据信息完整）

---

## 备份保留策略

| 类型 | 保留数量 | 保留时间 |
|------|:------:|:-------:|
| 日备份 | 7 份 | 7 天 |
| 周备份 | 4 份 | 28 天 |

自动清理由 `scripts/backup.sh` 中的 `find` 命令执行。
