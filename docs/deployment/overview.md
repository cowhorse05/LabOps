# 部署方式概览

> **目标读者:** 需要选择部署方式的用户
> **前置阅读:** [项目概述](../project/overview.md)

---

LabOps 支持多种部署方式，你可以根据使用场景和技术背景选择最适合的一种。

## 部署方式对比

| 部署方式 | 难度 | 适用场景 | 需要 Docker | 数据存储 | 推荐程度 |
|---------|:----:|---------|:-----------:|---------|:-------:|
| [Docker Compose（生产）](docker-compose.md) | ⭐⭐ | 有域名的云服务器 | 是 | Docker Volume + 宿主机 | ⭐⭐⭐ 最推荐 |
| [服务器部署教程](server-deployment.md) | ⭐⭐⭐ | 第一次部署到云服务器 | 是 | Docker Volume + 宿主机 | ⭐⭐⭐ 最推荐 |
| [Docker Compose（开发）](docker-compose.md) | ⭐ | 本机开发和测试 | 是 | Docker Volume | ⭐⭐ |
| [原生 Linux + systemd](native-linux.md) | ⭐⭐⭐⭐ | 不想用 Docker | 否 | 宿主机文件系统 | ⭐ |
| [原生 Windows](../deployment-guide.md) | ⭐⭐⭐⭐ | Windows 服务器 | 否 | 宿主机文件系统 | ⭐ |

## 各部署方式详解

### Docker Compose（生产环境）— 推荐

**适用人群：** 大多数用户

**优点：**
- 一条命令部署（`docker compose up -d`）
- 自动处理依赖（MySQL、Nginx、Server 一起启动）
- 网络隔离（数据库不暴露公网）
- 健康检查和自动重启
- 升级方便（拉新镜像重启）

**缺点：**
- 需要安装 Docker
- 容器有一定资源开销

**数据保存位置：**
- MySQL 数据：Docker Volume `mysql-data`
- TLS 证书：宿主机 `/etc/letsencrypt/`
- Agent 凭据：宿主机 `/etc/labops-agent/credentials.json`
- 备份：宿主机 `/var/backups/labops/`

**部署命令概览：**
```bash
cp .env.example .env          # 配置环境变量
docker compose config --quiet  # 验证配置
docker compose build           # 构建镜像
docker compose up -d           # 启动服务
```

**详细教程：** [Docker Compose 部署详解](docker-compose.md)

### 原生 Linux + systemd

**适用人群：** 不想使用 Docker、希望直接管理进程的高级用户

**优点：**
- 无容器开销
- 进程直接管理
- 与系统服务深度集成

**缺点：**
- 需要手动安装 Go、Node.js、MySQL
- 需要手动配置 Nginx
- 升级和回滚需要更多手动步骤

**详细教程：** [原生 Linux 部署](native-linux.md)

---

## 部署架构总览

不管你选择哪种部署方式，最终的运行架构都是：

```
Internet
  │
  ▼
域名 / 公网 IP
  │
  ▼
云防火墙（安全组 / 轻量服务器防火墙）
  │
  ▼
Nginx（:80 → :443 重定向 + TLS 终止）
  │
  ├──► /api/* → Go Server (:8080)
  │              │
  │              ├──► MySQL (:3306，内网)
  │              └──► LLM API（出站，可选）
  │
  ├──► /agent/ws → WebSocket 升级 → Agent 连接
  │
  └──► /* → 静态文件（React SPA）
```

**关键安全设计：**
- MySQL 的 3306 端口**不暴露到公网**（仅在 Docker 内部网络或 localhost）
- Server 的 8080 端口**不暴露到公网**（仅 Nginx 反向代理访问）
- 对外只开放 80（HTTP→HTTPS 重定向）和 443（HTTPS + WSS）
- Agent 通过 WSS（WebSocket over TLS）连接，通信加密

---

## 选择建议

| 你的情况 | 推荐方式 |
|---------|---------|
| 第一次部署，有阿里云/腾讯云服务器 | [服务器部署教程](server-deployment.md) |
| 有 Docker 经验，想快速部署 | [Docker Compose 部署](docker-compose.md) |
| 本机开发测试 | [Docker Compose（开发）](docker-compose.md) 或 `scripts/dev.ps1` |
| 不用 Docker，用 systemd | [原生 Linux 部署](native-linux.md) |

---

## 部署后必读

部署完成后，建议按顺序阅读：

1. [Nginx 与 HTTPS](nginx-and-https.md) — 理解网络层
2. [Agent 部署](agent-deployment.md) — 接入被管理设备
3. [数据持久化](../operations/data-storage.md) — 理解数据存在哪里
4. [备份与恢复](../operations/backup-restore.md) — 配置自动备份
