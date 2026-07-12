# 原生 Linux 部署（不含 Docker）

> **目标读者:** 不想使用 Docker 的用户
> **前置阅读:** [部署方式概览](overview.md)

---

## 适用人群

- 不想安装或使用 Docker 的用户
- 希望进程直接由 systemd 管理的用户
- 对容器化不熟悉的用户

## 优点

- 无容器开销
- 进程直接管理，更直观
- 与系统服务深度集成

## 缺点

- 需要手动安装 Go、Node.js、MySQL
- 需要手动配置 Nginx
- 升级和回滚步骤更多

---

## 前置条件

- Linux 服务器（Ubuntu 20.04+ / Debian 11+ / RHEL 8+ / Fedora 36+ / Arch）
- 有 sudo 权限的用户
- 网络可访问（用于下载依赖）

---

## 自动部署（推荐）

项目提供了 `scripts/deploy.sh` 脚本来自动化部署：

```bash
sudo bash scripts/deploy.sh --mode native --install-deps
```

**这个脚本做了什么：**

1. 检测操作系统（通过 `/etc/os-release`）
2. 安装 Go 和 Node.js 22.x
3. 编译 Server 和 Agent 为静态二进制文件（`CGO_ENABLED=0`）
4. 构建 React 前端（`npm run build` → `web/dist/`）
5. 创建 `labops` 系统用户
6. 安装二进制到 `/usr/local/bin/`
7. 创建数据目录 `/var/lib/labops`
8. 创建配置目录 `/etc/labops`
9. 生成默认环境变量文件 `/etc/labops/env`
10. 安装 systemd 服务（`labops-server.service` 和 `labops-agent@.service`）

---

## 手动部署

### 1. 安装依赖

**Go 1.25+：**

```bash
# dnf (Alibaba Cloud Linux / RHEL 系)
sudo dnf install -y golang

# apt (Ubuntu / Debian)
sudo snap install go --classic

# 或从官网下载：https://go.dev/dl/
```

**Node.js 22+：**

```bash
# 使用 NodeSource 仓库（Debian/Ubuntu）
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs

# 或从官网下载：https://nodejs.org/
```

**MySQL 8.0+：**

```bash
# dnf
sudo dnf install -y mysql-server
sudo systemctl enable --now mysqld

# apt
sudo apt install -y mysql-server
sudo systemctl enable --now mysql
```

### 2. 创建数据库

```bash
sudo mysql -e "CREATE DATABASE IF NOT EXISTS labops CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
sudo mysql -e "CREATE USER IF NOT EXISTS 'labops'@'localhost' IDENTIFIED BY '你的密码';"
sudo mysql -e "GRANT ALL PRIVILEGES ON labops.* TO 'labops'@'localhost';"
```

### 3. 编译

```bash
# 编译 Server
cd server
CGO_ENABLED=0 go build -ldflags="-s -w" -o labops-server ./cmd/server

# 编译 Agent
cd ../agent
CGO_ENABLED=0 go build -ldflags="-s -w" -o labops-agent ./cmd/agent

# 编译前端
cd ../web
npm ci
npm run build   # 输出到 web/dist/
```

### 4. 安装

```bash
# 创建系统用户
sudo useradd -r -s /bin/false -d /var/lib/labops -m labops

# 安装二进制
sudo cp server/labops-server /usr/local/bin/
sudo cp agent/labops-agent /usr/local/bin/

# 创建目录
sudo mkdir -p /etc/labops /var/lib/labops
sudo chown -R labops:labops /var/lib/labops
```

### 5. 配置环境变量

创建 `/etc/labops/env`：

```env
LABOPS_ADDR=:8080
LABOPS_DB_DRIVER=mysql
LABOPS_MYSQL_DSN=labops:你的密码@tcp(localhost:3306)/labops?parseTime=true&charset=utf8mb4
LABOPS_BOOTSTRAP_ADMIN_PASSWORD=你的管理员密码
LABOPS_ENCRYPTION_KEY=你的加密密钥
LABOPS_ENV=production
LABOPS_PUBLIC_ORIGIN=https://你的域名
```

### 6. 配置 systemd

创建 `/etc/systemd/system/labops-server.service`：

```ini
[Unit]
Description=LabOps Server
After=network.target mysql.service

[Service]
Type=simple
User=labops
Group=labops
EnvironmentFile=/etc/labops/env
ExecStart=/usr/local/bin/labops-server
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

### 7. 配置 Nginx

参考 [Nginx 与 HTTPS](nginx-and-https.md) 配置反向代理。

### 8. 启动

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now labops-server
sudo systemctl status labops-server
```

---

## 路径总览

| 路径 | 内容 |
|------|------|
| `/usr/local/bin/labops-server` | Server 二进制 |
| `/usr/local/bin/labops-agent` | Agent 二进制 |
| `/etc/labops/env` | 环境变量配置 |
| `/var/lib/labops/` | 运行时数据 |
| `/etc/systemd/system/labops-server.service` | Server 服务 |
| `/etc/systemd/system/labops-agent@.service` | Agent 模板服务 |

---

## 卸载

```bash
sudo systemctl stop labops-server
sudo systemctl disable labops-server
sudo rm -f /etc/systemd/system/labops-server.service
sudo rm -f /usr/local/bin/labops-server
sudo rm -f /usr/local/bin/labops-agent
sudo rm -rf /etc/labops
# /var/lib/labops 包含数据库数据，谨慎删除
```
