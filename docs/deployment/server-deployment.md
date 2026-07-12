# 服务器部署教程：从购买服务器到项目上线

> **目标读者:** 第一次接触 Linux 服务器的新手
> **前置阅读:** [项目概述](../project/overview.md)、[部署方式概览](overview.md)
> **预计耗时:** 1-2 小时（含等待时间）

---

本教程带你从零开始，将 LabOps 部署到一台云服务器上，并通过域名或公网 IP 访问。

**本教程基于项目的实际部署环境编写。** 示例服务器配置：

| 项目 | 配置 |
|------|------|
| 云厂商 | 阿里云 |
| 操作系统 | Alibaba Cloud Linux 3（基于 RHEL 8） |
| 包管理器 | dnf |
| 登录用户 | admin（有 sudo 权限） |
| Docker | 已安装 |
| Nginx | 已安装并运行 |
| 端口 | 22（SSH）、80（HTTP）、443（HTTPS）已开放 |

> **如果你使用 Ubuntu/Debian：** 本教程同时提供了 `dnf` 和 `apt` 的命令。当看到两条命令时，根据你的系统选择对应的一条。

---

## 目录

1. [部署前准备](#1-部署前准备)
2. [认识你的服务器操作系统](#2-认识你的服务器操作系统)
3. [SSH 登录服务器](#3-ssh-登录服务器)
4. [云防火墙和系统防火墙](#4-云防火墙和系统防火墙)
5. [安装必要依赖](#5-安装必要依赖)
6. [获取项目源码](#6-获取项目源码)
7. [配置环境变量](#7-配置环境变量)
8. [构建和启动项目](#8-构建和启动项目)
9. [Nginx 反向代理](#9-nginx-反向代理)
10. [域名和 DNS](#10-域名和-dns)
11. [HTTPS 和 SSL 证书](#11-https-和-ssl-证书)
12. [部署 Agent](#12-部署-agent)
13. [部署验收清单](#13-部署验收清单)

---

## 1. 部署前准备

### 你需要准备

| 物品 | 说明 | 是否必须 |
|------|------|:----:|
| 一台 Linux 云服务器 | 最低 2 核 CPU、4 GB 内存、20 GB 磁盘 | ✅ 必须 |
| 公网 IP | 云服务器自带 | ✅ 必须 |
| SSH 登录凭据 | 密码或 SSH 私钥 | ✅ 必须 |
| 域名（可选） | 如 cowhorse.xyz | 推荐（方便记忆 + 申请 HTTPS 证书） |
| Git | 用于下载项目代码 | ✅ 必须 |
| 项目源码 | `git clone https://github.com/cowhorse05/LabOps.git` | ✅ 必须 |

### 服务器最低配置建议

| 配置项 | 最低要求 | 推荐配置 | 说明 |
|--------|---------|---------|------|
| CPU | 2 核 | 4 核 | Go 编译和 MySQL 需要 CPU |
| 内存 | 4 GB | 8 GB | MySQL 默认占用约 500MB，Docker 额外开销 |
| 磁盘 | 20 GB | 40 GB SSD | 项目本身约需 500MB，数据库和备份需要额外空间 |
| 操作系统 | Ubuntu 22.04 / Alibaba Cloud Linux 3 | 两者皆可 | 本教程以 Alibaba Cloud Linux 3 为例 |

### 如果选择阿里云

1. 购买 ECS 实例或轻量应用服务器
2. 选择镜像：Alibaba Cloud Linux 3（默认）或 Ubuntu 22.04
3. 创建实例时设置 SSH 登录密码或上传 SSH 公钥
4. 在安全组中开放端口：**22**（SSH）、**80**（HTTP）、**443**（HTTPS）

> ⚠️ **不要开放 3306**（MySQL）或 **8080**（Server）端口。这些端口只需在服务器内部访问。

---

## 2. 认识你的服务器操作系统

第一次登录到一台新服务器时，先搞清楚你在什么环境里。

### 2.1 查看操作系统信息

```bash
cat /etc/os-release
```

**在哪里执行：** SSH 登录服务器后，在终端中执行。

**这条命令做什么：** 显示操作系统的名称、版本、ID 等信息。

**正常情况会看到：**
```
NAME="Alibaba Cloud Linux"
VERSION="3"
ID="alinux"
```
或者：
```
NAME="Ubuntu"
VERSION="22.04 LTS"
```

**如何判断用什么包管理器：**

| 你看到的信息 | 包管理器 | 安装命令 |
|-------------|:------:|---------|
| Alibaba Cloud Linux / Rocky Linux / AlmaLinux / RHEL / CentOS | dnf | `sudo dnf install -y <包名>` |
| Ubuntu / Debian | apt | `sudo apt install -y <包名>` |

### 2.2 查看当前用户和目录

```bash
whoami    # 显示当前用户名
pwd       # 显示当前所在目录
uname -a  # 显示系统内核信息
```

**正常情况：**
- `whoami` 显示你登录的用户名（如 `admin`）
- `pwd` 显示 `/home/admin`（或 `/root`）
- `uname -a` 显示 Linux 内核版本

### 2.3 判断用户权限

```bash
sudo whoami
```

**这条命令做什么：** 测试当前用户是否有 sudo（管理员）权限。

**正常结果：** 输出 `root`。

**如果出错：** 显示 `admin is not in the sudoers file`——说明你的用户没有管理员权限。你需要切换到 root 用户或联系云厂商。

> **本教程中所有需要管理员权限的命令都以 `sudo` 开头。** 如果你已经是 root 用户（`whoami` 显示 `root`），可以去掉 `sudo`。

---

## 3. SSH 登录服务器

### 3.1 什么是 SSH

SSH（Secure Shell）是一种安全的方式从你的电脑远程连接到服务器。当你在云厂商购买服务器后，得到的是一台在云端运行的电脑——你需要通过 SSH 来操作它。

### 3.2 使用密码登录

从你的**本地电脑**（不是服务器）执行：

```bash
ssh -p ${SERVER_PORT} ${SERVER_USER}@${SERVER_HOST}
```

**说明：**
- `${SERVER_PORT}` — SSH 端口，默认是 22
- `${SERVER_USER}` — 你的服务器用户名（如 `admin` 或 `root`）
- `${SERVER_HOST}` — 你的服务器公网 IP 地址

**举例：**
```bash
ssh -p 22 admin@47.96.xxx.xxx
```

**第一次连接会看到：**
```
The authenticity of host '47.96.xxx.xxx' can't be established.
Are you sure you want to continue connecting (yes/no)?
```

输入 `yes` 并回车。这是正常的——因为你是第一次连接这台服务器。

**然后输入密码。** 注意：输入密码时屏幕上不会显示任何字符（连 `***` 都不会显示），这是安全设计。输完后按回车。

### 3.3 使用 SSH 私钥登录

如果你在创建服务器时上传了 SSH 公钥：

```bash
ssh -i ${SSH_PRIVATE_KEY_PATH} -p ${SERVER_PORT} ${SERVER_USER}@${SERVER_HOST}
```

**说明：**
- `${SSH_PRIVATE_KEY_PATH}` — 你本地电脑上私钥文件的路径。Windows 通常是 `C:\Users\你的用户名\.ssh\id_rsa`，Linux/Mac 通常是 `~/.ssh/id_rsa`

**Windows PowerShell 用户注意：** 如果遇到 "UNPROTECTED PRIVATE KEY FILE" 警告，需要修改私钥文件权限。在 PowerShell 中执行比较麻烦，建议使用 Git Bash 或 WSL。

### 3.4 公钥和私钥的区别（简要说明）

| | 公钥 | 私钥 |
|---|------|------|
| 放在哪里 | 服务器上（~/.ssh/authorized_keys） | 你的本地电脑上 |
| 谁能看 | 公开也没关系 | **绝对不能泄露** |
| 作用 | 验证私钥的签名 | 证明你的身份 |

简单理解：公钥是「锁」，私钥是「钥匙」。你把锁（公钥）放在服务器上，用钥匙（私钥）开锁。

### 3.5 Permission denied 怎么办

**可能原因 1：用户名错误**

你使用的用户名在服务器上不存在。检查云厂商控制台确认正确的用户名。
- 阿里云 ECS：通常是 `root`
- 阿里云轻量服务器：创建时设置的用户名
- Ubuntu 云服务器：通常是 `ubuntu`

**可能原因 2：密码错误**

密码输入错误。尝试重新输入。注意大小写。

**可能原因 3：私钥不匹配**

你本地的私钥和服务器上的公钥不是一对。需要重新生成密钥对或在云厂商控制台重置。

**可能原因 4：SSH 端口不对**

默认端口是 22，但有些服务器会修改。检查云厂商控制台确认端口。

### 3.6 Windows / macOS / Linux 的区别

| 你的电脑系统 | 终端在哪里 | SSH 命令 |
|------------|----------|---------|
| Windows 10/11 | PowerShell 或 CMD | `ssh` 命令内置（Windows 10 1803+） |
| macOS | 终端.app | `ssh` 命令内置 |
| Linux | 终端 | `ssh` 命令内置 |

**如果 Windows 找不到 ssh 命令：** 安装 [Git for Windows](https://git-scm.com/download/win)，它自带 Git Bash，里面有 ssh 命令。

---

## 4. 云防火墙和系统防火墙

服务器有两层防火墙。数据从互联网到达你的项目需要经过：

```
Internet
  │
  ▼
云厂商安全组（或轻量服务器防火墙）  ← 第 1 层：在云厂商控制台配置
  │
  ▼
Linux 系统防火墙（firewalld 或 ufw） ← 第 2 层：在服务器上配置
  │
  ▼
正在监听的程序（Nginx、你的应用）
```

### 4.1 检查哪些端口正在监听

```bash
sudo ss -lntp
```

**这条命令做什么：** 列出所有正在监听的 TCP 端口，以及对应的程序。

**正常情况会看到类似：**
```
LISTEN  0  128  0.0.0.0:22    0.0.0.0:*  users:(("sshd",pid=1234,fd=3))
LISTEN  0  128  0.0.0.0:80    0.0.0.0:*  users:(("nginx",pid=5678,fd=6))
LISTEN  0  128  0.0.0.0:443   0.0.0.0:*  users:(("nginx",pid=5678,fd=7))
```

**说明：**
- `0.0.0.0:22` — SSH 在 22 端口监听
- `0.0.0.0:80` — Nginx 在 80 端口监听（HTTP）
- `0.0.0.0:443` — Nginx 在 443 端口监听（HTTPS）
- `127.0.0.1:3306` — MySQL 只在本地监听（安全！）

### 4.2 阿里云安全组

在阿里云控制台 → ECS 实例 → 安全组 → 配置规则中检查：

| 方向 | 端口 | 协议 | 授权对象 | 用途 |
|------|:---:|------|---------|------|
| 入方向 | 22 | TCP | 0.0.0.0/0 | SSH 登录 |
| 入方向 | 80 | TCP | 0.0.0.0/0 | HTTP 访问 |
| 入方向 | 443 | TCP | 0.0.0.0/0 | HTTPS 访问 |

**⚠️ 安全提醒：** `0.0.0.0/0` 表示允许所有 IP 访问。如果你有固定 IP，建议改为你的 IP 地址。

### 4.3 轻量服务器防火墙

如果你用的是阿里云轻量应用服务器，防火墙在控制台 → 防火墙 中配置，规则同上。

### 4.4 firewalld（Alibaba Cloud Linux / RHEL 系）

```bash
# 查看防火墙状态
sudo systemctl status firewalld

# 如果显示 inactive (dead)，说明防火墙未启用
# 如果显示 active (running)，需要添加规则
sudo firewall-cmd --permanent --add-port=80/tcp
sudo firewall-cmd --permanent --add-port=443/tcp
sudo firewall-cmd --reload
```

**如果 firewalld 为 inactive：** 这通常意味着系统依赖云安全组来做防护——这是许多云服务器的默认配置，是正常的，不需要额外操作。

### 4.5 ufw（Ubuntu）

```bash
# 查看状态
sudo ufw status

# 如果 inactive，启用并添加规则
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## 5. 安装必要依赖

### 5.1 先检查已安装的软件

```bash
git --version
docker --version
nginx -v
certbot --version
```

**这条命令做什么：** 检查 Git、Docker、Nginx、Certbot 是否已经安装。

**如果显示版本号：** 说明已安装，可以跳过对应的安装步骤。

**如果显示 command not found：** 需要安装。

### 5.2 安装 Git

**Alibaba Cloud Linux / Rocky Linux / RHEL 系（dnf）：**

```bash
sudo dnf install -y git
```

**Ubuntu / Debian（apt）：**

```bash
sudo apt update
sudo apt install -y git
```

### 5.3 安装 Docker 和 Docker Compose

**Alibaba Cloud Linux / Rocky Linux / RHEL 系（dnf）：**

```bash
# 安装 Docker
sudo dnf config-manager --add-repo=https://download.docker.com/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 启动 Docker
sudo systemctl enable --now docker

# 将当前用户加入 docker 组（之后不需要 sudo docker）
sudo usermod -aG docker $USER
```

**Ubuntu / Debian（apt）：**

```bash
# 添加 Docker 官方 GPG 密钥
sudo apt install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# 添加 Docker 仓库
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 启动 Docker
sudo systemctl enable --now docker

# 将当前用户加入 docker 组
sudo usermod -aG docker $USER
```

**⚠️ 重要：** 执行 `usermod -aG docker $USER` 后，需要**退出 SSH 重新登录**才能生效。重新登录后，执行 `docker ps` 确认不需要 sudo。

### 5.4 安装 Nginx

**Alibaba Cloud Linux / Rocky Linux / RHEL 系（dnf）：**

```bash
sudo dnf install -y nginx
sudo systemctl enable --now nginx
```

**Ubuntu / Debian（apt）：**

```bash
sudo apt install -y nginx
sudo systemctl enable --now nginx
```

**验证 Nginx 正常运行：**

```bash
sudo systemctl status nginx
curl http://localhost
```

如果看到 Nginx 的欢迎页面 HTML，说明正常。

### 5.5 安装 Certbot（用于 HTTPS 证书）

**Alibaba Cloud Linux / Rocky Linux / RHEL 系（dnf）：**

```bash
sudo dnf install -y certbot python3-certbot-nginx
```

**Ubuntu / Debian（apt）：**

```bash
sudo snap install core
sudo snap refresh core
sudo snap install --classic certbot
```

**验证安装：**

```bash
certbot --version
```

---

## 6. 获取项目源码

### 6.1 建议的目录结构

将项目放在一个组织良好的目录中：

```text
/opt/cowhorse/
├── source/       ← 项目源码（git clone 到这里）
├── config/       ← 配置文件（.env 等）
├── data/         ← 项目数据（可选，Docker Volume 管理）
├── logs/         ← 日志（可选，Docker 日志由 journald 管理）
└── backups/      ← 备份文件
```

### 6.2 创建目录并克隆项目

```bash
# 创建目录
sudo mkdir -p /opt/cowhorse/source

# 修改目录所有者为你的用户（这样不需要 sudo 操作源码）
sudo chown -R $USER:$USER /opt/cowhorse

# 克隆项目
cd /opt/cowhorse/source
git clone https://github.com/cowhorse05/LabOps.git .
```

**在哪里执行：** SSH 登录服务器后执行。

**这条命令做什么：** 创建项目目录 → 设置权限 → 从 GitHub 下载项目源码。

**正常结果：** 看到 `Receiving objects: 100%` 完成信息。

### 6.3 切换到指定版本（可选）

如果你想使用特定版本而不是最新代码：

```bash
# 查看所有版本标签
git tag

# 切换到指定版本（如 v0.2.0）
git checkout v0.2.0
```

---

## 7. 配置环境变量

### 7.1 复制环境变量模板

```bash
cd /opt/cowhorse/source
cp .env.example .env
chmod 600 .env
```

**这条命令做什么：**
- `cp .env.example .env` — 复制模板文件为实际配置文件
- `chmod 600 .env` — 将文件权限设为只有文件所有者可读写（其他用户完全不能访问）

### 7.2 编辑 .env 文件

```bash
nano .env   # 或 vim .env 或 vi .env
```

> **如果你不熟悉命令行编辑器：** `nano` 是最简单的。方向键移动光标，编辑文本，`Ctrl+O` 保存，`Ctrl+X` 退出。

### 7.3 必填环境变量说明

`.env` 文件中有 7 个以 `CHANGE_ME` 开头的值需要替换。以下是每个变量的详细说明：

| 变量 | 是否必填 | 示例 | 含义 | 是否敏感 |
|------|:---:|------|------|:---:|
| `SERVER_HOST` | ✅ 必填 | `cowhorse.xyz` 或 `47.96.xxx.xxx` | 公网域名或 IP | 否 |
| `MYSQL_ROOT_PASSWORD` | ✅ 必填 | 随机 16 位字符串 | MySQL 管理员密码 | ✅ 敏感 |
| `MYSQL_PASSWORD` | ✅ 必填 | 随机 16 位字符串 | MySQL 应用密码 | ✅ 敏感 |
| `LABOPS_MYSQL_DSN` | ✅ 必填 | `labops:密码@tcp(mysql:3306)/labops?...` | MySQL 连接串 | ✅ 敏感 |
| `LABOPS_BOOTSTRAP_ADMIN_PASSWORD` | ✅ 必填 | 至少 12 字符 | 首次登录的管理员密码 | ✅ 敏感 |
| `LABOPS_ENCRYPTION_KEY` | ✅ 必填 | 32 字节 Base64 编码 | LLM API Key 的 AES 加密密钥 | ✅ 敏感 |
| `LABOPS_VERSION` | 可选 | `dev` | Docker 镜像标签 | 否 |

### 7.4 如何生成随机密码和密钥

```bash
# 生成随机 MySQL 密码（16 字节 = 约 24 字符的 Base64）
openssl rand -base64 16

# 生成加密密钥（32 字节 = 约 44 字符的 Base64）
openssl rand -base64 32
```

**在哪里执行：** SSH 登录服务器后执行。

**这条命令做什么：** 使用系统的加密随机数生成器生成安全的随机字符串。

**正常结果：** 输出一行随机字符，如 `xK9mP2vR7nQ5wL8jF3aB1cD6eH4gN0sT`。

### 7.5 完整的 .env 填写示例

```env
SERVER_HOST=cowhorse.xyz
LABOPS_VERSION=dev

MYSQL_ROOT_PASSWORD=xK9mP2vR7nQ5wL8jF3aB
MYSQL_PASSWORD=aB1cD6eH4gN0sTpQ8wL2
LABOPS_MYSQL_DSN=labops:aB1cD6eH4gN0sTpQ8wL2@tcp(mysql:3306)/labops?parseTime=true&charset=utf8mb4

LABOPS_BOOTSTRAP_ADMIN_PASSWORD=my-strong-password-2024
LABOPS_ENCRYPTION_KEY=SkT8mP2vR7nQ5wL8jF3aB1cD6eH4gN0sTpQ8wL2jF3aB1cD6eH4g=
```

> ⚠️ **绝对不要** 把以上示例值用于生产环境！请用 `openssl rand` 生成你自己的随机值。

### 7.6 安全提醒

- `.env` 文件已通过 `.gitignore` 排除，**不会被提交到 Git**
- **不要** 把 `.env` 内容粘贴到聊天记录、博客、README 中
- **不要** 把密钥（密码、加密密钥）直接写进代码
- 生产环境建议使用 secret 管理工具（如 Docker secrets、HashiCorp Vault）

---

## 8. 构建和启动项目

### 8.1 验证配置

```bash
cd /opt/cowhorse/source
docker compose config --quiet
```

**这条命令做什么：** 检查 `compose.yaml` 和 `.env` 的配置是否正确。

**正常结果：** 没有输出（表示配置正确）。

**如果出错：** 会显示错误信息，通常是因为某个环境变量未设置或格式不正确。

### 8.2 构建 Docker 镜像

```bash
docker compose build
```

**这条命令做什么：** 根据 Dockerfile 构建三个镜像（Server、Web、Agent）。

**正常结果：** 看到一系列构建步骤，最后显示 `Successfully tagged`。

**耗时：** 首次构建约 3-5 分钟。后续构建会利用缓存，更快。

### 8.3 启动所有服务

```bash
docker compose up -d
```

**这条命令做什么：**
- `up` — 启动 compose.yaml 中定义的所有服务
- `-d` — detach（后台运行），终端关闭后服务继续运行

**正常结果：** 看到三行 `Container ... Started`。

### 8.4 查看服务状态

```bash
docker compose ps
```

**正常结果：**
```
NAME                   STATUS                    PORTS
labops-mysql-1         Up (healthy)              3306/tcp
labops-server-1        Up (healthy)              8080/tcp
labops-web-1           Up (healthy)              0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
```

**关键信息：**
- `(healthy)` — 服务的健康检查通过
- MySQL 和 Server 没有对外端口映射（`3306/tcp` 不带 `0.0.0.0:` 前缀）——这是安全设计
- Web 的 80 和 443 端口对外暴露

### 8.5 查看日志

```bash
# 查看所有服务日志
docker compose logs

# 查看特定服务日志
docker compose logs server

# 实时跟踪日志（Ctrl+C 退出）
docker compose logs -f
```

### 8.6 验证 API 健康检查

```bash
# 先在服务器内部测试
curl http://localhost:8080/api/health
```

**正常结果：** `{"status":"ok"}`

### 8.7 常用管理命令

| 操作 | 命令 | 说明 |
|------|------|------|
| 启动 | `docker compose up -d` | 后台启动所有服务 |
| 停止 | `docker compose stop` | 停止服务（不删除容器和数据） |
| 重启 | `docker compose restart` | 重新启动服务 |
| 查看状态 | `docker compose ps` | 查看服务运行状态 |
| 查看日志 | `docker compose logs -f` | 实时跟踪日志 |
| 重建并启动 | `docker compose up -d --build` | 修改代码后重建镜像 |

**⚠️ 关于删除数据的命令：**

- `docker compose stop` — **不删除数据**
- `docker compose down` — **不删除数据**（Volume 保留）
- `docker compose down -v` — **⚠️ 会删除数据库！**（删除所有 Volume）
- `docker volume rm mysql-data` — **⚠️ 会删除数据库！**

---

## 9. Nginx 反向代理

### 9.1 什么是反向代理

简单解释：

```
用户浏览器                    Nginx（反向代理）              你的应用
https://cowhorse.xyz  ──►  接收请求，转发给后端      ──►  http://localhost:8080
                           （加解密 HTTPS）
```

**为什么需要反向代理：**

1. **HTTPS 加密** — Go 服务端不直接处理 TLS，由 Nginx 负责加解密
2. **静态文件** — React 前端文件由 Nginx 直接返回，不需要经过 Go 服务端
3. **统一入口** — 前端、API、WebSocket 都通过同一个域名和端口访问
4. **安全隔离** — Go 服务端不直接暴露到公网

### 9.2 创建 Nginx 配置文件

根据你的操作系统，配置文件位置不同：

**Alibaba Cloud Linux / Rocky Linux / RHEL 系：**

```bash
sudo nano /etc/nginx/conf.d/labops.conf
```

**Ubuntu / Debian：**

```bash
sudo nano /etc/nginx/sites-available/labops
```

### 9.3 完整的 Nginx 配置

以下是根据项目实际 Nginx 配置模板（`web/nginx/default.conf.template`）改写的**宿主机 Nginx 配置**：

```nginx
# HTTP → HTTPS 重定向
server {
    listen 80;
    server_name cowhorse.xyz;  # 改为你的域名或 IP

    # 用于证书自动续期的验证
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    # 其他请求全部重定向到 HTTPS
    location / {
        return 301 https://$host$request_uri;
    }
}

# HTTPS 服务
server {
    listen 443 ssl http2;
    server_name cowhorse.xyz;  # 改为你的域名

    # SSL 证书（Certbot 自动配置，先写占位路径）
    ssl_certificate     /etc/letsencrypt/live/cowhorse.xyz/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cowhorse.xyz/privkey.pem;

    # 安全头
    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;

    # API 代理（含 WebSocket 支持）
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 前端静态文件
    location / {
        root /usr/share/nginx/html;  # React 构建产物目录
        try_files $uri $uri/ /index.html;  # SPA 路由回退
    }

    client_max_body_size 2m;
}
```

> **重要：** 如果你还没有 SSL 证书（第 11 章），先只配置 HTTP 部分（第一个 server 块）。等获取证书后再配置 HTTPS 部分。或者直接用 `certbot --nginx` 自动配置（见第 11 章）。

### 9.4 关于 WebSocket

Agent 通过 WSS（WebSocket over TLS）连接服务端。Nginx 需要正确配置 WebSocket 代理：

```nginx
proxy_set_header Upgrade $http_upgrade;     # 告诉后端：这个请求要升级为 WebSocket
proxy_set_header Connection "upgrade";      # 保持连接
```

这两行配置在 `/api/` 的 location 块中。因为 Agent 的 WebSocket 端点是 `/api/agent/ws`，它在 `/api/` 前缀下，所以 WebSocket 升级会正常工作。

### 9.5 测试并重载 Nginx

每次修改 Nginx 配置后：

```bash
# 1. 测试配置语法
sudo nginx -t

# 2. 如果显示 "syntax is ok" 和 "test is successful"，重载配置
sudo systemctl reload nginx
```

**为什么先 `nginx -t` 再 reload：**
- `nginx -t` 只检查语法，不实际修改运行中的 Nginx
- 如果语法正确，`reload` 让 Nginx 平滑加载新配置（不中断现有连接）
- 如果语法错误，`reload` 会失败，但现有的 Nginx 继续正常运行——不会因为配置错误导致服务中断

### 9.6 验证 Nginx 代理正常

```bash
# 通过 Nginx 访问 API
curl http://localhost/api/health
```

**正常结果：** `{"status":"ok"}`

这说明 Nginx 成功将请求从 80 端口转发到了 Go 服务端的 8080 端口。

---

## 10. 域名和 DNS

### 10.1 什么是 DNS

DNS（Domain Name System）将域名翻译成 IP 地址：

```
cowhorse.xyz  ──DNS 解析──►  47.96.xxx.xxx（服务器 IP）
```

**类比：** DNS 就像电话簿。域名是「人名」，IP 地址是「电话号码」。你输入人名，DNS 帮你找到对应的电话号码。

### 10.2 添加 A 记录

在你的域名提供商（如阿里云 DNS、Cloudflare、GoDaddy）添加一条 **A 记录**：

| 主机记录 | 记录类型 | 记录值 | TTL |
|---------|:------:|-------|:---:|
| `@` | A | `47.96.xxx.xxx`（你的服务器 IP） | 600 |
| `www` | A | `47.96.xxx.xxx`（你的服务器 IP） | 600 |

**说明：**
- `@` — 代表根域名本身（`cowhorse.xyz`）
- `www` — 代表 `www.cowhorse.xyz`
- TTL — Time To Live（缓存时间），单位秒。600 表示 10 分钟。

### 10.3 验证 DNS 解析

```bash
# 在你的本地电脑上执行（不是在服务器上）
nslookup cowhorse.xyz
```

或者：

```bash
dig cowhorse.xyz
```

**正常结果：** 显示的 IP 地址与你的服务器 IP 一致。

**如果显示不同的 IP 或找不到：**
- DNS 修改后需要时间生效（通常几分钟到几小时）
- 你的本地 DNS 缓存可能还没更新。试试 `nslookup cowhorse.xyz 8.8.8.8`（使用 Google DNS 查询）
- TTL 设得越长，缓存更新越慢

### 10.4 关于 ICP 备案

**中国内地（大陆）服务器：**

如果你使用的是阿里云中国内地节点（如杭州、北京、深圳）的服务器，域名需要 **ICP 备案** 才能通过域名访问。备案流程通常需要 1-3 周。

**海外服务器：**

如果你使用的是阿里云香港、新加坡、美国等海外节点的服务器，**通常不需要中国内地 ICP 备案**。

**判断你的服务器是否需要备案：**
- 看服务器 IP 归属地
- 在备案前，中国内地服务器用域名访问 80/443 端口会被拦截
- 临时方案：在备案期间直接使用 IP 地址访问 `http://你的IP`

> ⚠️ **备案是域名解析到中国内地服务器时的法律要求，和 SSL 证书没有关系。** 不要把它们混为一谈。

---

## 11. HTTPS 和 SSL 证书

### 11.1 为什么需要 HTTPS

- **加密传输：** 数据在浏览器和服务器之间加密，中间人无法窃听
- **身份验证：** 证明你访问的确实是这个网站，不是伪造的
- **浏览器信任：** 没有 HTTPS 的网站会被浏览器标记为「不安全」
- **Agent 连接：** Agent 通过 WSS（WebSocket Secure）连接，需要有效的 SSL 证书

### 11.2 使用 Certbot 申请 Let's Encrypt 证书

Let's Encrypt 提供免费的 SSL 证书，有效期 90 天。Certbot 是申请和管理 Let's Encrypt 证书的工具。

**申请证书前必须满足：**
1. 域名 DNS 已正确解析到服务器 IP（第 10 章）
2. 80 端口对外开放（云防火墙 + 系统防火墙已配置）
3. Nginx 已安装并正在运行
4. 域名已经可以通过 HTTP 访问

**使用 Certbot 自动配置 Nginx：**

```bash
# 申请证书并自动配置 Nginx
sudo certbot --nginx -d cowhorse.xyz -d www.cowhorse.xyz
```

**这条命令做什么：**
1. 验证你对域名的控制权（通过 80 端口的 HTTP 验证）
2. 向 Let's Encrypt 申请证书
3. 自动修改 Nginx 配置（添加 ssl_certificate 等指令）
4. 自动配置 HTTP → HTTPS 重定向

**正常结果：**
```
Congratulations! You have successfully enabled HTTPS on https://cowhorse.xyz
```

### 11.3 证书续期

Let's Encrypt 证书有效期 90 天。Certbot 安装时会自动配置续期定时器：

```bash
# 测试续期是否正常（不会实际续期）
sudo certbot renew --dry-run
```

**正常结果：** 显示续期测试通过。

```bash
# 检查自动续期定时器
sudo systemctl status certbot.timer
```

如果定时器是 active 的，证书会在到期前自动续期，无需人工干预。

### 11.4 没有域名怎么办（使用 IP 证书）

如果你的服务器只有 IP 地址没有域名，也可以申请证书（需要 certbot 5.4+）：

```bash
sudo /snap/bin/certbot certonly --standalone --preferred-profile shortlived --ip-address 47.96.xxx.xxx
```

**注意：** IP 证书的有效期只有约 6 天（shortlived profile），需要更频繁地续期。建议尽量使用域名。

### 11.5 验证 HTTPS 配置

```bash
# 在服务器上测试
curl -I https://cowhorse.xyz

# 在你的本地浏览器中打开
https://cowhorse.xyz
```

如果浏览器地址栏显示 🔒 锁图标，说明 HTTPS 配置成功。

### 11.6 证书申请失败怎么办

**问题：** `connection refused` 或 `timeout` 连接 80 端口

**原因：** 80 端口未开放或 Nginx 未运行

**检查：**
```bash
sudo ss -lntp | grep :80
curl http://localhost
```

**修复：** 确保 Nginx 正在运行，80 端口在安全组/防火墙中已开放。

**问题：** DNS 验证失败

**原因：** 域名 DNS 还没生效，或 A 记录指向了错误的 IP

**检查：**
```bash
nslookup cowhorse.xyz
```

**修复：** 等 DNS 生效（几分钟到几小时），确认 A 记录正确。

---

## 12. 部署 Agent

### 12.1 Agent 的作用

Agent 是安装在被管理设备上的小程序。它：

- 上报设备信息（主机名、OS、CPU、内存、磁盘）
- 每 10 秒发送一次心跳（CPU%/内存%/磁盘%）
- 接收并执行服务端下发的命令
- 返回命令执行结果

### 12.2 编译 Agent 二进制文件

Agent 需要编译为二进制文件。在服务器上编译：

```bash
cd /opt/cowhorse/source/agent

# 编译 Linux amd64 版本（适用于大多数云服务器）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o labops-agent ./cmd/agent
```

**如果 Go 未安装：** 可以安装 Go 或从开发机编译后上传。

```bash
# 安装 Go（Alibaba Cloud Linux / dnf）
sudo dnf install -y golang

# 安装 Go（Ubuntu / apt）
sudo snap install go --classic
```

**编译参数说明：**
- `CGO_ENABLED=0` — 禁用 CGO，生成纯静态二进制（不依赖系统 C 库）
- `GOOS=linux GOARCH=amd64` — 目标平台
- `-ldflags="-s -w"` — 去掉调试信息，减小文件大小

### 12.3 生成设备接入码

1. 在浏览器中打开 LabOps Web 界面（`https://cowhorse.xyz`）
2. 登录管理员账号
3. 进入「**设备接入**」页面
4. 点击「**生成接入码**」
5. 设置有效期（建议 30 分钟）和最大使用次数（建议 1 次）
6. 复制生成的接入码（**只显示一次，请立即保存**）

### 12.4 使用安装脚本（推荐）

```bash
cd /opt/cowhorse/source

# 将接入码替换为你的实际接入码
sudo bash scripts/install-agent.sh \
  --server "https://cowhorse.xyz" \
  --enroll-code "你的接入码" \
  --name "$(hostname)" \
  --group "homelab" \
  --binary agent/labops-agent
```

**这条命令做什么：**
1. 创建专用的 `labops-agent` 系统用户（没有登录权限）
2. 创建配置目录 `/etc/labops-agent/` 和运行时目录 `/var/lib/labops-agent/`
3. 复制 Agent 二进制到 `/usr/local/bin/labops-agent`
4. 使用接入码注册设备（Agent 调用 `POST /api/agent/enroll`）
5. 保存设备凭据到 `/etc/labops-agent/credentials.json`
6. 安装并启动 systemd 服务

**正常结果：** 显示 systemd 服务状态，`Active: active (running)`。

### 12.5 验证 Agent 在线

在 Web 界面的「**设备管理**」页面，应该能看到新设备，状态为 **在线**。

### 12.6 systemd 服务详解

安装脚本创建的服务文件在 `/etc/systemd/system/labops-agent.service`。

**启动/停止/重启/查看状态：**

```bash
sudo systemctl start labops-agent     # 启动
sudo systemctl stop labops-agent      # 停止
sudo systemctl restart labops-agent   # 重启
sudo systemctl status labops-agent    # 查看状态
sudo systemctl enable labops-agent    # 设置开机自启
```

**查看 Agent 日志：**

```bash
sudo journalctl -u labops-agent -f    # 实时跟踪
sudo journalctl -u labops-agent -n 50 # 最近 50 行
```

### 12.7 手动安装 Agent（不使用脚本）

```bash
# 1. 创建目录
sudo mkdir -p /etc/labops-agent /var/lib/labops-agent

# 2. 复制二进制文件
sudo cp agent/labops-agent /usr/local/bin/

# 3. 执行接入（替换接入码）
sudo /usr/local/bin/labops-agent \
  --enroll-only \
  --enroll-code "你的接入码" \
  --server "https://cowhorse.xyz" \
  --name "$(hostname)" \
  --group "homelab" \
  --real \
  --credentials /etc/labops-agent/credentials.json

# 4. 运行 Agent（前台测试）
sudo /usr/local/bin/labops-agent \
  --server "https://cowhorse.xyz" \
  --name "$(hostname)" \
  --group "homelab" \
  --real
```

---

## 13. 部署验收清单

部署完成后，逐项检查以下内容。每一项都给出了验证命令和预期结果。

### 基础连接

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| SSH 能登录 | `ssh ${SERVER_USER}@${SERVER_HOST}` | 成功进入服务器 | 检查用户名、密码、端口、安全组 |
| 当前用户有 sudo 权限 | `sudo whoami` | 显示 `root` | 联系云厂商或切换到 root |

### Docker

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| Docker 已安装 | `docker --version` | 显示版本号 | 重新安装 Docker |
| Docker Compose 可用 | `docker compose version` | 显示版本号 | 安装 docker-compose-plugin |
| 能不使用 sudo 运行 docker | `docker ps` | 显示容器列表（可能为空） | `sudo usermod -aG docker $USER` 后重新登录 |
| 所有容器在运行 | `docker compose ps` | 3 个服务状态都是 Up (healthy) | `docker compose logs` 查看错误 |
| MySQL 健康 | `docker compose exec mysql mysqladmin ping -uroot -p"$MYSQL_ROOT_PASSWORD"` | `mysqld is alive` | 检查 MySQL 密码和网络 |

### Nginx

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| Nginx 已安装 | `nginx -v` | 显示版本号 | 重新安装 Nginx |
| Nginx 正在运行 | `sudo systemctl status nginx` | Active: active (running) | `sudo systemctl start nginx` |
| 80 端口在监听 | `sudo ss -lntp \| grep :80` | 显示 nginx 在监听 | 检查 Nginx 配置 |
| 配置语法正确 | `sudo nginx -t` | syntax is ok | 检查配置文件路径和语法 |

### 应用

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| 本机 API 正常 | `curl http://localhost:8080/api/health` | `{"status":"ok"}` | 检查 Server 容器日志 |
| 通过 Nginx API 正常 | `curl http://localhost/api/health` | `{"status":"ok"}` | 检查 Nginx 代理配置 |
| 前端页面正常 | 浏览器打开 `http://你的IP` | 显示登录页面 | 检查前端构建和 Nginx 配置 |

### 公网访问

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| 公网 IP 可访问 | 浏览器打开 `http://你的公网IP` | 显示登录页面 | 检查安全组/防火墙 |
| 域名解析正常 | `nslookup 你的域名` | 返回你的服务器 IP | 检查 DNS A 记录 |

### HTTPS

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| HTTPS 可访问 | 浏览器打开 `https://你的域名` | 显示 🔒 + 登录页面 | 检查证书和 Nginx 配置 |
| 证书有效 | 浏览器点击 🔒 → 证书 | 有效期 90 天，Let's Encrypt 签发 | 重新申请证书 |
| 证书可续期 | `sudo certbot renew --dry-run` | 续期测试成功 | 检查 80 端口是否开放 |

### Agent

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| Agent 服务运行 | `sudo systemctl status labops-agent` | Active: active (running) | `journalctl -u labops-agent` 查看错误 |
| Agent 凭据存在 | `sudo ls -la /etc/labops-agent/credentials.json` | 文件存在 | 重新使用接入码注册 |
| 设备在 Web 界面显示在线 | 浏览器打开 Web 界面 → 设备管理 | 设备状态：在线 | 检查 Agent 日志和网络连通性 |

### 持久化

| 检查项 | 验证命令 | 预期结果 | 失败了怎么办 |
|--------|---------|---------|-------------|
| 重启后服务自动恢复 | `sudo reboot` 后等 2 分钟再检查 | 所有服务重新启动 | 检查 Docker 和 systemd 的 enable 状态 |
| 重启后数据没丢失 | 重启后登录 Web 界面 | 设备和任务数据都在 | 检查 Docker Volume 是否被删除 |

---

## 部署完成

如果你通过了以上所有检查项，恭喜！LabOps 已经成功部署在你的服务器上了 🎉

**下一步：**

- [Agent 部署详解](agent-deployment.md) — 在其他设备上安装 Agent
- [快速开始](../user-guide/quick-start.md) — 学习如何使用系统
- [备份与恢复](../operations/backup-restore.md) — 配置自动备份
- [故障排查](../troubleshooting/) — 遇到问题来这里找答案
