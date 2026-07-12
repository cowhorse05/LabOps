# Docker Compose 部署详解

> **目标读者:** 熟悉 Docker 的用户、开发者
> **前置阅读:** [部署方式概览](overview.md)

---

## 适用人群

- 有 Docker 使用经验的开发者
- 希望快速部署的用户
- 不需要手把手教学的用户

## 优点

- 一条命令启动所有服务
- 网络隔离（数据库不暴露公网）
- 健康检查和自动重启
- 升级方便

## 缺点

- 需要安装 Docker
- 容器有一定开销

---

## 前置条件

1. 已安装 Docker Engine 24+ 和 Docker Compose Plugin v2
2. 已安装 Git
3. 已获取 SSL 证书（生产环境），或使用自签名证书

---

## 开发环境部署

### 1. 克隆项目

```bash
git clone https://github.com/cowhorse05/LabOps.git
cd LabOps
```

### 2. 启动开发环境

```bash
# Linux / macOS
bash scripts/dev.sh

# Windows (PowerShell)
.\scripts\dev.ps1
```

**这个命令做了什么：**
- 使用 `compose.dev.yaml` 启动三个服务（mysql、server、web）
- MySQL 端口映射到宿主机的 3307
- Server 端口映射到 8080
- Web 使用 Vite 开发服务器，端口 5173
- 自动启动 4 个模拟 Agent（Ubuntu、Windows、Server、Edge 配置）
- 密码和 DSN 都是预配置的开发值

### 3. 打开浏览器

访问 `http://localhost:5173`，使用 `compose.dev.yaml` 中配置的管理员密码登录。

### 4. 停止

```bash
bash scripts/compose-down.sh      # Linux
.\scripts\compose-down.ps1        # Windows
```

---

## 生产环境部署

### 1. 配置环境变量

```bash
cp .env.example .env
chmod 600 .env
```

编辑 `.env`，替换所有 `CHANGE_ME` 值。详见 [服务器部署教程 - 配置环境变量](server-deployment.md#7-配置环境变量)。

### 2. 获取 SSL 证书（使用 certbot standalone 模式）

```bash
# 确保证书前 80 端口没有被占用
sudo /snap/bin/certbot certonly --standalone -d cowhorse.xyz
```

### 3. 构建并启动

```bash
docker compose config --quiet     # 验证配置
docker compose build               # 构建镜像
docker compose up -d               # 后台启动
```

### 4. 验证

```bash
docker compose ps                  # 所有服务应为 Up (healthy)
curl https://cowhorse.xyz/api/health
```

### 5. 证书续期钩子

证书续期后需要重载 Nginx：

```bash
sudo tee /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh << 'EOF'
#!/bin/bash
cd /opt/labops && docker compose exec -T web nginx -s reload
EOF
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
```

---

## Compose 文件解析

### 服务与网络

```
compose.yaml 定义了三个服务 + 三个网络：

服务：
  mysql    — MySQL 8.0 官方镜像，数据存储在 mysql-data 卷
  server   — Go API 服务端，从 ./server 构建，健康检查 /api/health
  web      — Nginx 反向代理，从 ./web 构建，暴露 80/443

网络：
  backend  — 内部网络（internal: true），MySQL 和 Server 不对外
  egress   — 出站网络，Server 用它访问外部 LLM API
  edge     — 边缘网络，Nginx 对外暴露 80/443
```

### 健康检查

```yaml
# MySQL：每 10 秒 ping 一次，失败 20 次后判定不健康
healthcheck:
  test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
  interval: 10s
  timeout: 5s
  retries: 20

# Server：每 30 秒访问 /api/health，失败 10 次后判定不健康
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/health"]
  interval: 30s
  timeout: 3s
  retries: 10
```

### 依赖顺序

```yaml
# web 等待 server 健康后才启动
web:
  depends_on:
    server:
      condition: service_healthy

# server 等待 mysql 健康后才启动
server:
  depends_on:
    mysql:
      condition: service_healthy
```

---

## 常用操作

| 操作 | 命令 |
|------|------|
| 查看日志 | `docker compose logs -f` |
| 重启单个服务 | `docker compose restart server` |
| 重建并重启 | `docker compose up -d --build` |
| 停止（保留数据） | `docker compose stop` |
| 停止并删除容器 | `docker compose down` |
| ⚠️ 删除所有数据 | `docker compose down -v` |
| 查看资源占用 | `docker stats` |

---

## 升级

```bash
git pull origin master
docker compose build
docker compose up -d
```

**回滚：**

```bash
git checkout <之前的提交>
docker compose build
docker compose up -d
```
