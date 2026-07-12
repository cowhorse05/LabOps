# 升级与回滚

> **目标读者:** 运维人员
> **前置阅读:** [备份与恢复](backup-restore.md)

---

## 升级前准备

### 1. 查看当前版本

```bash
cd /opt/cowhorse/source
git log --oneline -1
```

### 2. 备份数据库

```bash
sudo LABOPS_BACKUP_DIR=/var/backups/labops bash scripts/backup.sh
```

**⚠️ 绝对不要跳过备份！** 如果升级出错，备份是唯一的救命稻草。

---

## 升级步骤（Docker Compose）

### 1. 拉取最新代码

```bash
cd /opt/cowhorse/source
git fetch origin
git pull origin master
```

### 2. 查看变更

```bash
git log HEAD ^origin/master --oneline  # 升级前
git log --oneline -5                    # 升级后查看最近的提交
```

### 3. 重建镜像

```bash
docker compose build
```

### 4. 启动新版本

```bash
docker compose up -d
```

### 5. 等待健康检查通过

```bash
docker compose ps
# 等待所有服务状态变为 (healthy)
```

### 6. 验证

```bash
curl https://你的域名/api/health
# → {"status":"ok"}
```

### 7. 手动测试

- 浏览器登录 Web 界面
- 检查设备列表是否正常
- 执行一条测试命令

---

## 回滚

### 回滚到之前的代码版本

```bash
# 查看 Git 历史
git log --oneline -10

# 切换到之前的提交
git checkout <之前的提交哈希>

# 重建并启动
docker compose build
docker compose up -d
```

### 回滚到特定版本标签

```bash
git checkout v0.1.0
docker compose build
docker compose up -d
```

### 如果代码回滚不够（数据出了问题）

```bash
# 恢复升级前做的备份
sudo LABOPS_CONFIRM_RESTORE=RESTORE \
  bash scripts/restore.sh /var/backups/labops/升级前的备份.sql.gz
```

---

## 什么情况下不能直接回滚数据库

如果新版本修改了数据库 Schema（新增了列或表），直接恢复旧备份可能需要手动处理：

1. 查看新版本添加了哪些 Schema 变更（检查 `server/internal/core/migrations.go`）
2. 如果新版本添加了表，恢复旧备份不会删除这些表
3. 如果新版修改了列定义，恢复旧备份可能导致数据丢失

**安全做法：** 恢复前先在测试数据库上执行恢复，确认无误后再恢复生产数据库。

---

## 镜像标签策略

项目使用 `LABOPS_VERSION` 环境变量控制镜像标签：

```env
LABOPS_VERSION=dev      # 默认：每次构建都是最新
LABOPS_VERSION=v0.2.0   # 固定版本
```

**建议：**
- 生产环境使用 Git 提交哈希作为标签：`git rev-parse --short HEAD`
- 升级前先给当前镜像打备份标签：`docker tag labops-server:dev labops-server:backup-$(date +%Y%m%d)`

---

## 升级检查清单

| 检查项 | 说明 |
|--------|------|
| ✅ 已备份数据库 | 升级前执行 backup.sh |
| ✅ 已验证新代码 | 查看 git log 了解变更 |
| ✅ 健康检查通过 | docker compose ps 显示 healthy |
| ✅ API 正常 | curl /api/health 返回 ok |
| ✅ 可登录 | 浏览器登录成功 |
| ✅ 设备在线 | 设备列表显示在线 |
| ✅ 任务可执行 | 执行测试命令成功 |
