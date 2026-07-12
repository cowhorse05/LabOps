# LabOps 使用手册

> 版本: v0.2 · 最后更新: 2026-07-08

## 目录

1. [项目简介](#1-项目简介)
2. [快速开始](#2-快速开始)
3. [系统架构](#3-系统架构)
4. [功能指南](#4-功能指南)
   - 4.1 [登录](#41-登录)
   - 4.2 [仪表盘](#42-仪表盘)
   - 4.3 [设备管理](#43-设备管理)
   - 4.4 [设备详情与命令执行](#44-设备详情与命令执行)
   - 4.5 [分组管理](#45-分组管理)
   - 4.6 [任务系统](#46-任务系统)
   - 4.7 [审计日志](#47-审计日志)
   - 4.8 [AI Ops 智能分析](#48-ai-ops-智能分析)
5. [API 参考](#5-api-参考)
6. [配置说明](#6-配置说明)
7. [演示场景](#7-演示场景)
8. [故障排查](#8-故障排查)
9. [项目结构速查](#9-项目结构速查)

---

## 1. 项目简介

LabOps 是一个面向学生、实验室、Homelab 的轻量开源运维平台。通过 Docker 容器模拟多台设备，实现真实的 **Agent 注册 → 心跳上报 → 命令下发 → 结果回传 → 审计入库** 闭环。

| 特性 | 说明 |
|------|------|
| 真实闭环 | 非纯 Mock 数据，Agent 真实执行命令 |
| Docker 模拟 | 单机运行 4 种设备 profile (Windows/Ubuntu/Server/Edge) |
| 批量命令 | 按设备分组批量下发 |
| AI Ops | 每 30 分钟自动分析设备健康状况 |
| 审计追溯 | 所有操作记录可查 |
| 最小依赖 | SQLite 单文件存储，无需 MySQL/Redis |

### 技术栈

| 层 | 技术 |
|----|------|
| 前端 | React 18 + TypeScript + Vite + Ant Design 5 + Zustand |
| 后端 | Go 1.23 + net/http + gorilla/websocket |
| 存储 | SQLite (modernc.org/sqlite, 纯 Go 实现) |
| Agent | Go 1.23 + gorilla/websocket |
| 部署 | Docker Compose |

---

## 2. 快速开始

### 环境要求

- Windows 10/11 + PowerShell
- Docker Desktop (或 Podman)
- Node.js 20+ (仅前端开发需要)
- Go 1.23 (可选，Go 测试可通过 Docker 运行)

### 启动

```powershell
# 克隆仓库
git clone https://github.com/cowhorse05/LabOps.git
cd LabOps

# 一键启动全部服务
.\scripts\dev.ps1
```

等待 Docker 构建完成（首次约 2-3 分钟），然后打开浏览器：

```
http://localhost:5173
```

**默认登录**: `admin` / `admin`

### 停止

```powershell
.\scripts\compose-down.ps1
```

### 运行检查

```powershell
.\scripts\test.ps1
```

---

## 3. 系统架构

```
┌──────────────────────────────────────────────────────────┐
│                    Docker Compose                         │
│                                                          │
│  ┌──────────┐   REST /api    ┌──────────┐               │
│  │  Web     │───────────────▶│  Server  │               │
│  │  :5173   │◀──────────────│  :8080   │               │
│  │  React   │    JSON        │  Go      │               │
│  └──────────┘                └────┬─────┘               │
│                                   │                      │
│                    WebSocket       │  SQLite              │
│              ┌────────────────────┼──────┐              │
│              │                    │      │              │
│         ┌────▼───┐ ┌────▼───┐ ┌──▼──┐ ┌─▼────┐       │
│         │Agent   │ │Agent   │ │Agent│ │Agent │        │
│         │lab-pc  │ │lab-pc  │ │srv  │ │edge  │        │
│         │Win     │ │Ubuntu  │ │Srv  │ │Edge  │        │
│         └────────┘ └────────┘ └─────┘ └──────┘        │
└──────────────────────────────────────────────────────────┘
```

### 数据流

```
Agent ──WebSocket──▶ Server ◀──REST API──▶ Web Console
  │                     │                       │
  │  register           │  UpsertDevice          │  GET /api/devices
  │  heartbeat/10s      │  UpdateHeartbeat       │  POST /api/tasks
  │  task_result         │  CompleteTask          │  GET /api/aiops/report
  │                     │  CreateAudit           │
                          │
                          ▼
                      SQLite
```

### WebSocket 协议

所有消息格式: `{"type": "<type>", "payload": {...}}`

| 方向 | type | 说明 | 频率 |
|------|------|------|------|
| Agent→Server | `register` | 设备注册 | 连接时 |
| Agent→Server | `heartbeat` | 心跳+指标上报 | 10s |
| Agent→Server | `task_result` | 命令执行结果 | 按需 |
| Server→Agent | `registered` | 注册确认 | 注册后 |
| Server→Agent | `command` | 下发命令 | 按需 |
| Server→Agent | `error` | 错误通知 | 异常时 |

---

## 4. 功能指南

### 4.1 登录

- 访问 `http://localhost:5173` 自动跳转登录页
- 默认凭据: `admin` / `admin`
- 登录成功后 token 存储在 `localStorage`，页面刷新不丢失
- 401 自动踢回登录页

### 4.2 仪表盘

首页展示系统全局状况：

| 卡片 | 内容 |
|------|------|
| 设备总数/在线/离线 | 实时统计数字 |
| 设备概览表 | 前 6 台设备，含 CPU 进度条和在线状态 |
| 在线率 | 仪表盘进度环 + 分组在线/总数标签 |
| 最近任务 | 最新 5 条任务记录 |
| 最近审计 | 最新 5 条审计日志 |

自动刷新: **每 10 秒**。手动刷新按钮在页面右上角。

### 4.3 设备管理

**设备列表** (`/devices`)

- 显示全部已注册设备
- 搜索框: 按设备名、分组、系统、IP、主机名实时过滤
- 列: 设备名/主机名、分组、系统、IP、状态、CPU、最后心跳
- 点击"详情"进入单设备页面
- 自动刷新: 10 秒

**设备状态说明**:

| 状态 | 颜色 | 含义 |
|------|------|------|
| 在线 | 绿色 | Agent 持续发送心跳 |
| 离线 | 灰色 | 心跳超时 (35s) 或 Agent 主动断开 |

### 4.4 设备详情与命令执行

**设备详情** (`/devices/:id`)

- 资产信息: 系统、主机名、IP、Agent 版本、CPU/内存/磁盘规格
- 实时指标: CPU/内存/磁盘 使用率进度条
- 最后心跳时间

**命令执行**:

1. 在"命令执行"卡片的文本框中输入命令
2. 点击"执行"按钮
3. 命令通过 WebSocket 下发到 Agent
4. Agent 真实执行命令，返回 stdout、stderr、exit code、耗时
5. 结果在"最近任务"表格中实时显示

示例命令:
```bash
uname -a && echo hello-from-labops
hostname && date
ls -la /app
cat /etc/os-release
df -h
free -m
```

> **注意**: Agent 命令输出限制 stdout 256KB、stderr 64KB，超限自动截断。命令超时 30 秒。

### 4.5 分组管理

**分组页** (`/groups`)

- Agent 注册时通过 `--group` 参数指定分组
- 当前 Demo 分组: `classroom-a`、`homelab`、`edge`
- 表格: 分组名、设备数、在线数、在线率进度条
- 自动刷新: 10 秒

### 4.6 任务系统

**任务页** (`/tasks`)

**批量命令下发**:

1. 选择目标分组（下拉框显示各分组的在线/总数）
2. 输入命令
3. 点击"下发"——Server 自动为该分组**每台设备**创建独立任务
4. 在线设备立即收到命令并执行，离线设备标记为 pending

**任务记录表**:

| 列 | 说明 |
|----|------|
| 设备 | 目标设备名 |
| 分组 | 设备所属分组 |
| 命令 | 执行的命令文本 |
| 状态 | pending/running/success/failed/timeout |
| 退出码 | 0=成功, 非0=失败, 124=超时 |
| 耗时 | 毫秒 |
| 创建时间 | 任务创建时刻 |

点击行展开可查看 **stdout** 和 **stderr** 完整输出。

自动刷新: **3 秒**。

### 4.7 审计日志

**审计页** (`/audit`)

记录所有关键操作事件:

| 动作类型 | 触发时机 |
|----------|---------|
| `agent.register` | Agent 注册 |
| `agent.disconnect` | Agent 断连 |
| `command.dispatch` | 命令已下发 |
| `command.queue` | 命令已排队（设备离线） |
| `command.complete` | 命令执行完成 |
| `command.failed` | 命令执行失败 |

表格列: 时间、操作者、动作、设备、状态、说明。

自动刷新: **15 秒**。

### 4.8 AI Ops 智能分析

**AI Ops 页** (`/aiops`)

系统每 **30 分钟** 自动分析所有设备状态，生成健康报告。

**分析维度**:

| 检测项 | 阈值 | 严重程度 |
|--------|------|---------|
| 设备离线 | status != online | ⚠️ 扣 40 分 |
| CPU 偏高 | > 80% | ⚠️ 扣 20 分 |
| CPU 较高 | > 60% | ℹ️ 扣 10 分 |
| 内存偏高 | > 80% | ⚠️ 扣 20 分 |
| 内存较高 | > 60% | ℹ️ 扣 10 分 |
| 磁盘偏高 | > 85% | ⚠️ 扣 15 分 |
| 任务失败率 | > 50% | ⚠️ 扣 20 分 |
| 部分任务失败 | > 20% | ℹ️ 扣 10 分 |

**页面展示**:

- 顶部摘要卡片: 总体评估一句话
- 统计卡片: 设备总数、在线、离线、**健康评分** (0-100)
- 设备洞察: 每台设备一张卡片，含图标（✅正常/ℹ️注意/⚠️告警）、标题、详情、健康分数
- 分组概览表: 各分组的在线率、平均健康分、告警数量

自动刷新: **30 秒**。

---

## 5. API 参考

### REST API

Base URL: `http://localhost:8080/api`

| Method | Path | Auth | 说明 |
|--------|------|:----:|------|
| GET | `/health` | - | 健康检查 → `{"status":"ok"}` |
| POST | `/auth/login` | - | 登录并设置安全会话 Cookie |
| GET | `/auth/me` | Session | 当前用户、角色与权限 |
| GET | `/stats` | Session | 设备统计 `{total, online, offline}` |
| GET | `/devices` | Bearer | 设备列表 |
| GET | `/devices/{id}` | Bearer | 设备详情 |
| GET | `/groups` | Bearer | 分组列表 (含 online/total) |
| GET | `/tasks` | Bearer | 任务列表 (含 result, LIMIT 200) |
| POST | `/tasks` | Session+CSRF | 创建模板任务或管理员临时命令 |
| GET | `/tasks/{id}` | Bearer | 任务详情 |
| GET | `/audit-logs` | Bearer | 审计日志 (LIMIT 200) |
| GET | `/aiops/report` | Bearer | AI Ops 分析报告 |
| POST | `/agent/enroll` | 注册码 | Agent 首次登记 |
| GET | `/agent/ws` | Agent credential | WSS 升级 |

### 认证

Web API 使用可撤销会话 Cookie；所有写请求同时验证 CSRF Cookie 与 `X-CSRF-Token`。Agent 使用一次性注册码换取每设备独立凭据，WSS 通过 `Authorization: Agent <deviceId>:<secret>` 认证。系统不提供通用默认 Token。

---

## 6. 配置说明

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LABOPS_ADDR` | `:8080` | Server 监听地址 |
| `LABOPS_DB_PATH` | `data/labops.db` | SQLite 数据库路径 |
| `LABOPS_BOOTSTRAP_ADMIN_PASSWORD` | 无 | 空数据库管理员初始化密码 |
| `LABOPS_ENCRYPTION_KEY` | 无 | Base64 编码的 32 字节加密密钥 |
| `LABOPS_PUBLIC_ORIGIN` | 生产必填 | 精确 Web HTTPS Origin |
| `VITE_PROXY_TARGET` | `http://localhost:8080` | Vite 开发代理目标 |

### Agent 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--server` | `http://localhost:8080` | Server 地址 |
| `--enroll-code` | 无 | 首次登记的一次性注册码 |
| `--credentials` | `/etc/labops-agent/credentials.json` | 独立设备凭据文件 |
| `--id` | 服务端签发 | 稳定 Agent ID |
| `--name` | `hostname` | 设备显示名称 |
| `--group` | `default` | 设备分组 |
| `--mock-profile` | `ubuntu` | 模拟设备类型 |

### Mock Profile

| Profile | 系统 | CPU | 内存 | 磁盘 |
|---------|------|-----|------|------|
| `ubuntu` | Ubuntu Desktop 24.04 | 4C | 4GB | 64GB |
| `windows-lab` | Windows 11 Pro | 8C | 16GB | 512GB |
| `server` | Ubuntu Server 24.04 | 4C | 8GB | 128GB |
| `edge-node` | Debian Edge Node | 2C | 2GB | 32GB |

---

## 7. 演示场景

### 场景 1: 查看全设备在线

1. `.\scripts\dev.ps1` 启动
2. 登录后进入仪表盘，看到 4 台设备在线
3. 进入设备列表，确认 4 台设备状态为绿色"在线"

### 场景 2: 单设备执行命令

1. 设备列表 → 点击 `lab-pc-01` 的"详情"
2. 输入命令 `uname -a && hostname`
3. 点击"执行"
4. 下方任务表格出现新行，展开查看 stdout

### 场景 3: 批量命令

1. 进入任务页 `/tasks`
2. 选择分组 `classroom-a`（含 2 台在线设备）
3. 输入命令 `date && whoami`
4. 点击"下发"——提示"已创建 2 个任务"
5. 任务表格显示 2 条新记录，各有独立结果

### 场景 4: 模拟设备离线

1. 新终端执行: `docker compose stop agent-edge-01`
2. 等待约 35 秒（心跳超时）
3. 仪表盘离线数变为 1，设备状态变灰
4. AI Ops 页面显示 edge-node-01 为告警（下次分析时）

### 场景 5: AI Ops 分析

1. 确保系统运行超过 30 分钟，或重启 Server 触发首次分析
2. 进入 AI Ops 页面
3. 查看摘要、健康评分、各设备洞察卡片
4. 如果某设备离线或 CPU 偏高，会显示 ⚠️ 告警

### 场景 6: 审计追溯

1. 执行几条命令后进入审计页
2. 看到 `agent.register` → `command.dispatch` → `command.complete` 的完整链条
3. 每条记录包含操作者、设备、状态、详细信息

---

## 8. 故障排查

### 前端构建失败

```powershell
cd web
rm -r -Force node_modules
npm install
npm run build
```

### Docker 镜像拉取失败

如果 `golang:1.23-alpine` 无法拉取：

```powershell
# 方案 1: 配置 Docker 镜像加速
# Docker Desktop → Settings → Docker Engine → 添加 registry-mirrors

# 方案 2: 本地安装 Go 后直接测试
cd server && go test ./...
cd agent && go test ./...
```

### Server 无法启动

1. 检查端口占用: `netstat -ano | findstr 8080`
2. 检查 `data/` 目录权限
3. 查看 Docker 日志: `docker compose logs server`

### Agent 无法连接

1. 确认 Server 已完全启动（healthcheck 10 次重试）
2. 检查 token 是否匹配: server 的 `LABOPS_AGENT_TOKEN` 和 agent 的 `--token`
3. Agent 自动重连: 1s → 2s → 4s → ... → 最大 60s，成功连接后重置

### Web 页面空白

1. 确认 Server 在运行: `curl http://localhost:8080/api/health`
2. 打开浏览器 DevTools → Network，检查 API 请求
3. 检查 localStorage 中 token 是否存在

### 数据清空

```powershell
# 删除数据库和持久化卷
docker compose down -v
Remove-Item -Force data\labops.db -ErrorAction SilentlyContinue
```

---

## 9. 项目结构速查

```text
LabOps/
├── web/                    # React 前端
│   └── src/
│       ├── api/            # axios 客户端 + API 函数
│       ├── components/     # 通用组件 (ErrorBoundary)
│       ├── hooks/          # 自定义 hooks (useLoadable, useLoadableAll)
│       ├── layouts/        # 布局 (AppLayout: 侧边栏+头部+内容)
│       ├── pages/          # 8 个页面
│       │   ├── LoginPage       # 登录
│       │   ├── DashboardPage   # 仪表盘
│       │   ├── DevicesPage     # 设备列表
│       │   ├── DeviceDetailPage# 设备详情+命令
│       │   ├── GroupsPage      # 分组
│       │   ├── TasksPage       # 批量命令+任务记录
│       │   ├── AuditPage       # 审计日志
│       │   └── AiOpsPage       # AI Ops (新增)
│       ├── stores/         # Zustand 状态 (auth)
│       ├── styles/         # 全局 CSS
│       ├── utils/          # 工具函数 (statusColor/Text)
│       └── types.ts        # TypeScript 类型定义
├── server/                 # Go 后端
│   └── internal/core/
│       ├── types.go        # 领域类型+常量+协议
│       ├── store.go        # SQLite CRUD (6 表)
│       ├── app.go          # HTTP 路由+中间件+维护循环
│       ├── api.go          # REST handler (12 端点)
│       ├── agent.go        # WebSocket handler
│       ├── analyzer.go     # AI Ops 分析引擎 (新增)
│       ├── store_test.go   # 存储层测试 (9 cases)
│       ├── api_test.go     # HTTP 测试 (24 cases)
│       └── analyzer_test.go# 分析器测试 (新增)
├── agent/                  # Go Agent
│   └── cmd/agent/
│       ├── main.go         # Agent 主逻辑
│       └── main_test.go    # Agent 测试 (~19 cases)
├── docs/                   # 文档
│   ├── master-plan.md      # 总体计划
│   ├── product-plan.md     # 产品定位
│   ├── research.md         # 调研对比
│   ├── roadmap.md          # 版本路线图
│   ├── dev-log.md          # 开发日志
│   ├── log.md              # 变更日志
│   ├── report.md           # 汇报材料
│   └── user-manual.md      # 本手册 (新增)
├── scripts/                # PowerShell 脚本
├── compose.yaml            # Docker Compose 编排
└── README.md               # 项目首页
```
