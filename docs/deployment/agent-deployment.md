# Agent 部署详解

> **目标读者:** 需要接入被管理设备的管理员
> **前置阅读:** [服务器部署教程](server-deployment.md#12-部署-agent)

---

## Agent 是什么

Agent 是一个用 Go 编写的小型程序（约 3-5 MB 的静态二进制文件），安装在你需要管理的每台设备上。它的工作：

```
Agent 的工作循环：
  1. 连接服务器 → 2. 注册设备信息 → 3. 每 10 秒发送心跳
                                          ↓
                              4. 收到命令 → 5. 执行命令 → 6. 返回结果
```

---

## 编译 Agent

### 从源码编译

```bash
cd /opt/cowhorse/source/agent

# Linux amd64（云服务器、大多数 PC）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o labops-agent ./cmd/agent

# Linux arm64（树莓派、ARM 服务器）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o labops-agent ./cmd/agent

# Windows amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o labops-agent.exe ./cmd/agent
```

### 编译参数说明

| 参数 | 含义 | 效果 |
|------|------|------|
| `CGO_ENABLED=0` | 禁用 CGO | 生成纯 Go 静态二进制，不依赖系统 C 库，可在任何同架构 Linux 上运行 |
| `GOOS=linux` | 目标操作系统 | 在 Windows/Mac 上也能编译 Linux 程序（交叉编译） |
| `GOARCH=amd64` | 目标架构 | x86_64（Intel/AMD 64 位） |
| `-ldflags="-s -w"` | 链接器标志 | 去除调试信息，减小文件体积 |

### 二进制文件体积

编译后的 Agent 约 8-10 MB（根据 Go 版本和依赖略有差异）。

---

## 接入流程

### 完整流程图

```
管理员（Web 界面）                Agent（设备上）              服务器
     │                              │                       │
     │ 生成接入码                    │                       │
     │ POST /api/enrollment-codes   │                       │
     │ ──────────────────────────────────────────────────►  │
     │                              │                       │
     │ 复制接入码给 Agent            │                       │
     │ ──────────────────────────►  │                       │
     │                              │                       │
     │                              │ POST /api/agent/enroll│
     │                              │ （接入码 + 设备信息）    │
     │                              │ ────────────────────► │
     │                              │                       │ 验证接入码
     │                              │                       │ 生成设备密钥
     │                              │ ◄──────────────────── │
     │                              │ 收到 deviceId +       │
     │                              │ deviceSecret          │
     │                              │ 保存到本地文件          │
     │                              │                       │
     │                              │ WebSocket 连接          │
     │                              │ Authorization: Agent   │
     │                              │ <id>:<secret>          │
     │                              │ ────────────────────► │ 验证凭据
     │                              │                       │ 注册成功
```

---

## 使用安装脚本（推荐）

### 基本用法

```bash
sudo bash scripts/install-agent.sh \
  --server "https://cowhorse.xyz" \
  --enroll-code "接入码" \
  --name "$(hostname)" \
  --group "homelab" \
  --binary agent/labops-agent
```

### 参数说明

| 参数 | 必填 | 说明 |
|------|:--:|------|
| `--server` | ✅ | 服务器 URL，必须以 `https://` 或 `http://` 开头 |
| `--enroll-code` | 首次必填 | 从 Web 界面获取的一次性接入码 |
| `--name` | 否 | 设备显示名称，默认使用系统主机名 |
| `--group` | 否 | 设备分组，默认 `default` |
| `--binary` | 否 | Agent 二进制文件路径，默认 `./agent/labops-agent` |

### 脚本做了什么（逐行解读）

```bash
# 1. 检查 root 权限
[[ "${EUID}" -eq 0 ]] || { echo "Run as root" >&2; exit 1; }

# 2. 创建专用的系统用户（无登录 shell）
id -u labops-agent >/dev/null 2>&1 || \
  useradd --system --home /var/lib/labops-agent --shell /usr/sbin/nologin labops-agent

# 3. 创建目录
install -d -o root -g labops-agent -m 0750 /etc/labops-agent
install -d -o labops-agent -g labops-agent -m 0750 /var/lib/labops-agent

# 4. 安装二进制文件
install -o root -g root -m 0755 "$BINARY" /usr/local/bin/labops-agent

# 5. 执行接入（首次安装时）
/usr/local/bin/labops-agent \
  --server="$SERVER_URL" --name="$AGENT_NAME" --group="$AGENT_GROUP" \
  --real --enroll-code="$ENROLLMENT_CODE" --enroll-only \
  --credentials=/etc/labops-agent/credentials.json

# 6. 设置凭据文件权限
chown root:labops-agent /etc/labops-agent/credentials.json
chmod 0640 /etc/labops-agent/credentials.json

# 7. 写入环境配置
printf 'LABOPS_SERVER_URL=%q\nLABOPS_AGENT_NAME=%q\nLABOPS_AGENT_GROUP=%q\n' \
  "$SERVER_URL" "$AGENT_NAME" "$AGENT_GROUP" > /etc/labops-agent/agent.env

# 8. 安装并启动 systemd 服务
install -o root -g root -m 0644 deploy/systemd/labops-agent.service \
  /etc/systemd/system/labops-agent.service
systemctl daemon-reload
systemctl enable --now labops-agent
```

---

## Systemd 服务详细解读

```ini
[Unit]
Description=LabOps low-privilege device agent
After=network-online.target              # 等网络就绪后再启动
Wants=network-online.target

[Service]
Type=simple                               # 前台运行（不 fork）
User=labops-agent                         # 以专用用户身份运行
Group=labops-agent
EnvironmentFile=/etc/labops-agent/agent.env # 从文件加载环境变量
ExecStart=/usr/local/bin/labops-agent \
  --server=${LABOPS_SERVER_URL} \
  --name=${LABOPS_AGENT_NAME} \
  --group=${LABOPS_AGENT_GROUP} \
  --credentials=/etc/labops-agent/credentials.json \
  --real
Restart=on-failure                        # 异常退出时自动重启
RestartSec=5s                             # 等待 5 秒再重启

# 安全加固
NoNewPrivileges=true                      # 禁止进程及其子进程获取新权限
PrivateTmp=true                           # 提供独立的 /tmp
ProtectSystem=strict                      # /usr、/boot、/etc 只读
ProtectHome=true                          # 禁止访问 /home、/root
ProtectKernelTunables=true                # /sys、/proc/sys 只读
ProtectKernelModules=true                 # 禁止加载内核模块
ProtectControlGroups=true                 # cgroup 只读
RestrictSUIDSGID=true                     # 禁止 setuid/setgid
LockPersonality=true                      # 禁止修改执行域
RestrictRealtime=true                     # 禁止实时调度
ReadWritePaths=/var/lib/labops-agent      # 唯一可写路径

[Install]
WantedBy=multi-user.target                # 多用户模式下启动
```

### 每项安全设置的含义

| 设置 | 防止的攻击 |
|------|-----------|
| `NoNewPrivileges=true` | 防止进程通过 SUID 程序提权 |
| `PrivateTmp=true` | 防止通过共享 /tmp 与其他进程交互 |
| `ProtectSystem=strict` | 防止修改系统二进制和配置文件 |
| `ProtectHome=true` | 防止读取/修改用户数据 |
| `RestrictSUIDSGID=true` | 防止设置 SUID/SGID 位 |
| `RestrictRealtime=true` | 防止通过实时调度耗尽 CPU |

---

## 手动安装（Linux）

如果不使用安装脚本：

```bash
# 1. 复制二进制
sudo cp agent/labops-agent /usr/local/bin/

# 2. 创建凭据目录
sudo mkdir -p /etc/labops-agent

# 3. 执行接入（替换接入码）
sudo /usr/local/bin/labops-agent \
  --enroll-only \
  --enroll-code "你的接入码" \
  --server "https://cowhorse.xyz" \
  --name "$(hostname)" \
  --group "homelab" \
  --real \
  --credentials /etc/labops-agent/credentials.json
```

---

## Windows 安装

Windows 上 Agent 通过命令行运行。对于生产环境，推荐使用 NSSM 注册为 Windows 服务。

```powershell
# 创建目录
New-Item -ItemType Directory -Force -Path "$env:ProgramData\LabOps"

# 执行接入
.\labops-agent.exe `
  --enroll-only `
  --enroll-code "你的接入码" `
  --server "https://cowhorse.xyz" `
  --name $env:COMPUTERNAME `
  --group "homelab" `
  --real

# 运行 Agent
.\labops-agent.exe `
  --server "https://cowhorse.xyz" `
  --name $env:COMPUTERNAME `
  --group "homelab" `
  --real
```

---

## Agent CLI 完整参考

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `--server` | `LABOPS_SERVER_URL` | `http://localhost:8080` | 服务器 URL |
| `--token` | `LABOPS_AGENT_TOKEN` | (无) | 旧版共享令牌（不推荐） |
| `--device-secret` | `LABOPS_DEVICE_SECRET` | (无) | 设备密钥 |
| `--enroll-code` | `LABOPS_ENROLLMENT_CODE` | (无) | 一次性接入码 |
| `--credentials` | `LABOPS_AGENT_CREDENTIALS` | 平台默认路径 | 凭据文件路径 |
| `--name` | `LABOPS_AGENT_NAME` | 主机名 | 设备名称 |
| `--group` | `LABOPS_AGENT_GROUP` | `default` | 设备分组 |
| `--id` | `LABOPS_AGENT_ID` | `agent-<name>` | 稳定 Agent ID |
| `--mock-profile` | `LABOPS_MOCK_PROFILE` | `ubuntu` | 模拟配置 |
| `--real` | `LABOPS_AGENT_REAL` | false | 使用真实系统指标 |
| `--enroll-only` | (仅 CLI) | false | 只接入，不运行 |

---

## Agent 文件位置

| 文件 | Linux 路径 | Windows 路径 |
|------|-----------|-------------|
| 二进制 | `/usr/local/bin/labops-agent` | 任意位置 |
| 凭据 | `/etc/labops-agent/credentials.json` | `%ProgramData%\LabOps\credentials.json` |
| 环境配置 | `/etc/labops-agent/agent.env` | 环境变量 |
| 运行时数据 | `/var/lib/labops-agent/` | 无 |
| 日志 | `journalctl` | 控制台 |

---

## 卸载 Agent

### Linux

```bash
sudo bash scripts/uninstall-agent.sh
```

**这个脚本做了什么：**
1. 停止并禁用 systemd 服务
2. 删除 systemd 单元文件
3. 删除二进制 `/usr/local/bin/labops-agent`
4. **保留凭据目录** `/etc/labops-agent/`（如果你需要重新安装）

### 手动清理

```bash
sudo systemctl stop labops-agent
sudo systemctl disable labops-agent
sudo rm -f /etc/systemd/system/labops-agent.service
sudo systemctl daemon-reload
sudo rm -rf /etc/labops-agent /var/lib/labops-agent
sudo rm -f /usr/local/bin/labops-agent
```

---

## 撤销设备凭据

1. 在 Web 界面 → 设备管理 → 找到设备
2. 点击「吊销凭据」
3. 确认操作

吊销后，Agent 的密钥立即失效。下次 Agent 尝试连接时会被拒绝。被吊销的设备需要重新生成接入码并重新注册。

---

## 常见问题

### Agent 无法连接

1. 检查服务器 URL 是否正确（协议、域名、端口）
2. 检查凭据文件是否存在：`sudo ls -la /etc/labops-agent/credentials.json`
3. 检查环境配置：`sudo cat /etc/labops-agent/agent.env`
4. 查看日志：`sudo journalctl -u labops-agent -n 50`

### Agent 显示离线

- 检查 Agent 服务是否在运行：`sudo systemctl status labops-agent`
- 检查设备能否访问服务器（特别是 443 端口）
- 检查凭据是否被吊销

### 重新接入已吊销的设备

1. 在 Web 界面生成新的接入码
2. 在设备上重新执行接入流程
