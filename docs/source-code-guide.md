# LabOps 源码阅读指南

> **本书定位：** 这是一本面向开发者的源码阅读指南，以教科书的方式组织内容，帮助你系统地理解 LabOps 项目从架构设计到具体实现的每一个细节。全书采用严格的章节依赖关系 —— 每一章都建立在前一章的知识基础之上。建议按顺序阅读。

---

## 目录

**第一部分：入门准备**

- [第一章 项目概览](#第一章-项目概览) — 认识 LabOps：它是什么、为什么这样设计
- [第二章 环境搭建与启动](#第二章-环境搭建与启动) — 在本地运行项目，观察三服务协作

**第二部分：数据基础**

- [第三章 数据库设计与方言抽象](#第三章-数据库设计与方言抽象) — 12 张表的设计意图与 SQLite/MySQL 双后端

**第三部分：前端架构**

- [第四章 前端整体架构](#第四章-前端整体架构) — React + Vite + Ant Design 的项目骨架
- [第五章 前端页面深度拆解](#第五章-前端页面深度拆解) — 12 个页面逐页分析

**第四部分：通信协议**

- [第六章 WebSocket 通信协议](#第六章-websocket-通信协议) — Agent 与服务端的消息协议设计

**第五部分：后端核心**

- [第七章 服务端启动流程](#第七章-服务端启动流程) — 从 main.go 到 App 的完整初始化链路
- [第八章 HTTP 路由与中间件链](#第八章-http-路由与中间件链) — 请求如何经过层层处理到达处理器
- [第九章 REST API 处理器](#第九章-rest-api-处理器) — 每个端点背后的业务逻辑
- [第十章 用户认证与会话管理](#第十章-用户认证与会话管理) — Session Cookie、CSRF、RBAC 实现

**第六部分：Agent 系统**

- [第十一章 Agent WebSocket 服务端](#第十一章-agent-websocket-服务端) — 服务端如何管理 Agent 连接
- [第十二章 Agent 客户端实现](#第十二章-agent-客户端实现) — Agent 如何连接、心跳、执行命令

**第七部分：AI 运维**

- [第十三章 AI 运维分析引擎](#第十三章-ai-运维分析引擎) — 规则引擎与 LLM 增强分析

**第八部分：生产部署**

- [第十四章 生产部署架构](#第十四章-生产部署架构) — Docker Compose、Nginx TLS、Systemd

**第九部分：总结**

- [第十五章 完整学习路径](#第十五章-完整学习路径) — 项目架构决策回顾与技术栈全景图

---

## 前置知识

阅读本书需要具备以下基础知识：

| 领域 | 要求 |
|------|------|
| Go 语言 | 理解 struct、interface、goroutine、channel、`net/http` 基础 |
| TypeScript/React | 理解函数组件、Hooks、React Router、状态管理 |
| SQL | 理解表设计、索引、事务 |
| 网络协议 | 理解 HTTP/HTTPS、WebSocket、TLS |
| Docker | 理解镜像、容器、Docker Compose、网络 |

---

# 第一部分：入门准备

---

## 第一章 项目概览

### 1.1 LabOps 是什么

LabOps 是一个**轻量级运维平台**。它的核心能力可以概括为一句话：

> **让运维人员通过 Web 界面，实时监控和管理分布在多台机器上的 Agent 进程。**

为了实现这个目标，LabOps 构建了一个三层系统：

```
┌──────────────────────────────────────────────────────────┐
│                    LabOps 三层架构                         │
│                                                          │
│  ┌─────────────────┐                                     │
│  │   Web 控制台     │  ← 你在浏览器中看到的界面              │
│  │  (React + TS)    │    负责展示数据、发送指令              │
│  └────────┬────────┘                                     │
│           │ REST API (HTTP/HTTPS)                        │
│  ┌────────▼────────┐                                     │
│  │   服务端 (Go)    │  ← 核心大脑                          │
│  │                  │    接收 Web 请求、管理 Agent、        │
│  │                  │    执行业务逻辑、存储数据              │
│  └──┬──────────┬───┘                                     │
│     │          │ WebSocket (WSS)                         │
│     │ SQL      │                                         │
│  ┌──▼────┐  ┌──▼──────────┐                              │
│  │ MySQL │  │ Agent 进程   │  ← 被管理的目标机器             │
│  │ (8.0) │  │ (Go 二进制)   │    上报信息、执行命令           │
│  └───────┘  └─────────────┘                              │
└──────────────────────────────────────────────────────────┘
```

**关键设计理念：**

1. **真实数据闭环** — 没有模拟数据填充。仪表盘中的每一台设备都通过真实的 WebSocket 连接。
2. **最小依赖** — 后端只依赖 Go 标准库 + gorilla/websocket + 数据库驱动。不使用任何 Web 框架。
3. **一处部署** — 一条 `docker compose up -d` 命令启动全部服务。

### 1.2 项目目录总览

在看源码之前，先建立对目录结构的全局认知。下表按**阅读顺序**排列，记录了每个模块的职责：

```
LabOps/
│
├── 📁 server/                         ← 先读这里！核心逻辑所在
│   ├── cmd/server/main.go             ← 入口：从这里开始追踪代码
│   └── internal/core/
│       ├── types.go                   ← 所有数据结构定义
│       ├── dialect.go                 ← 数据库设计意图
│       ├── store.go                   ← 数据库操作
│       ├── app.go                     ← 路由 + 中间件 + 后台任务
│       ├── api.go                     ← REST 处理器
│       ├── agent.go                   ← WebSocket Hub
│       ├── auth_context.go            ← 认证中间件
│       ├── enrollment.go              ← 设备接入
│       ├── analyzer.go                ← AI 运维引擎
│       ├── llm.go                     ← LLM 客户端
│       ├── templates.go               ← 命令模板
│       ├── encryption.go              ← 加密工具
│       ├── security_store.go          ← 安全存储
│       ├── migrations.go              ← Schema 迁移
│       └── dialect_*.go               ← 数据库方言实现
│
├── 📁 agent/                          ← 次读这里！Agent 是独立进程
│   └── cmd/agent/main.go              ← Agent 全部逻辑（约 675 行）
│
├── 📁 web/                            ← 再读这里！前端界面
│   └── src/
│       ├── main.tsx                   ← 前端入口
│       ├── router.tsx                 ← 路由定义
│       ├── types.ts                   ← TypeScript 类型
│       ├── api/                       ← API 调用层
│       │   ├── client.ts              ← Axios 实例
│       │   └── labops.ts              ← 类型化 API 函数
│       ├── stores/auth.ts             ← 认证状态
│       ├── hooks/useLoadable.ts       ← 数据加载 Hook
│       ├── layouts/AppLayout.tsx      ← 布局框架
│       ├── pages/                     ← 12 个页面
│       └── utils/                     ← 工具函数
│
├── 📁 deploy/                         ← 部署配置
│   ├── systemd/                       ← 系统服务单元
│   └── README.md                      ← 部署说明
│
├── 📁 scripts/                        ← 运维脚本
├── 📁 docs/                           ← 项目文档
├── 📄 compose.yaml                    ← 生产 Compose
├── 📄 compose.dev.yaml                ← 开发 Compose
└── 📄 .env.example                    ← 环境变量模板
```

### 1.3 阅读路线图

如果你是第一次阅读这个项目，建议按以下路线推进：

```
第一章 (项目概览)
    │
    ▼
第二章 (环境搭建) ──► 动手运行，观察效果
    │
    ▼
第三章 (数据库设计) ──► 理解数据模型
    │
    ├──────────────────────────────────────┐
    ▼                                      ▼
第四章 (前端架构)                     第六章 (通信协议)
    │                                      │
    ▼                                      ▼
第五章 (页面拆解)                     第十一章 (Agent 服务端)
    │                                      │
    │                                      ▼
    │                               第十二章 (Agent 客户端)
    │                                      │
    ▼                                      │
第七章 (服务端启动) ◄───────────────────────┘
    │
    ▼
第八章 (路由与中间件)
    │
    ▼
第九章 (API 处理器)
    │
    ▼
第十章 (认证与会话)
    │
    ▼
第十三章 (AI 运维)
    │
    ▼
第十四章 (生产部署)
    │
    ▼
第十五章 (总结)
```

> **提示：** 虚线表示可以先独立阅读 Agent 相关的章节（第六、十一、十二章），不需要先理解前端。前后端章节之间没有严格依赖。

---

## 第二章 环境搭建与启动

### 2.1 启动开发环境

在阅读源码之前，先让项目跑起来：

**Windows（PowerShell）：**

```powershell
git clone https://github.com/cowhorse05/LabOps.git
cd LabOps
.\scripts\dev.ps1
```

**Linux / macOS（bash）：**

```bash
git clone https://github.com/cowhorse05/LabOps.git
cd LabOps
bash scripts/dev.sh
```

浏览器打开 `http://localhost:5173`，使用默认管理员密码登录（密码在 `compose.dev.yaml` 中配置）。

### 2.2 开发环境中有哪些服务

`compose.dev.yaml` 启动了三个服务和一个 MySQL：

```
┌────────────────────────────────────────────────────┐
│                  Docker 开发环境                      │
│                                                    │
│  mysql:8.0          →  localhost:3307               │
│  server (Go)        →  localhost:8080               │
│  web (Vite dev)     →  localhost:5173               │
│                                                    │
│  4 个模拟 Agent 在 server 容器内自动启动               │
│    ├── ubuntu      (Ubuntu Desktop 24.04)           │
│    ├── windows-lab (Windows 11 Pro)                 │
│    ├── server      (Ubuntu Server 24.04)            │
│    └── edge-node   (Debian Edge)                    │
└────────────────────────────────────────────────────┘
```

关键配置文件：

| 文件 | 作用 |
|------|------|
| `compose.dev.yaml` | 定义开发环境的三个服务和网络 |
| `compose.yaml` | 生产环境定义（多了 TLS、Nginx 反向代理） |
| `.env.example` | 生产环境变量模板 |

### 2.3 前端开发代理

Vite 开发服务器将 `/api` 请求代理到 Go 服务端：

```typescript
// web/vite.config.ts
export default defineConfig({
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://server:8080',  // Docker 内部 DNS
        changeOrigin: true,
      },
    },
  },
})
```

这意味着在浏览器中访问 `http://localhost:5173/api/health` 会被 Vite 转发到 `http://server:8080/api/health`。`server` 是 Docker Compose 的服务名，Docker 内部 DNS 会解析为容器的 IP。

### 2.4 观察系统运行

花 5 分钟在浏览器中浏览以下页面，观察实时更新的数据：

| 页面 | 观察点 |
|------|--------|
| 仪表盘 (Dashboard) | 4 个设备卡片、在线率仪表盘、最近的审计日志 — 每 10 秒自动刷新 |
| 设备管理 (Devices) | 每个设备的 CPU/内存/磁盘使用率在变化 — 模拟 Agent 每 10 秒发送心跳 |
| 设备详情 (Device Detail) | 点击设备，查看实时指标进度条和任务历史 |
| 审计日志 (Audit) | 能看到 agent_register、heartbeat 等操作记录 |

有了感性认识之后，我们开始深入源码。

---

# 第二部分：数据基础

---

## 第三章 数据库设计与方言抽象

> **前置章节：** 第一章
> **核心文件：** `server/internal/core/dialect.go`、`dialect_mysql.go`、`dialect_sqlite.go`、`migrations.go`、`store.go`

### 3.1 设计目标

LabOps 的数据库层有三个设计目标：

1. **开发用 SQLite，生产用 MySQL** — 同一套代码，零配置切换。默认驱动为 MySQL
2. **自动建表** — 首次启动时根据 Schema 定义自动创建所有表
3. **版本化迁移** — 后续版本能够修改表结构而不影响已有数据

### 3.2 方言抽象（Dialect Pattern）

实现双数据库支持的关键是**方言抽象**：

```
         ┌──────────────────┐
         │   Dialect 接口    │  ← 定义所有数据库操作的标准接口
         ├──────────────────┤
         │ DriverName()     │
         │ PreConnect()     │
         │ ConfigurePool()  │
         │ Validate()       │
         │ TypeMap()        │  ← 逻辑类型 → 物理类型的映射
         │ TableSuffix()    │
         │ Placeholder()    │
         └───┬──────────┬───┘
             │          │
    ┌────────▼──┐  ┌───▼─────────┐
    │ SQLite    │  │ MySQL       │
    │ Dialect   │  │ Dialect     │
    ├───────────┤  ├─────────────┤
    │ TEXT      │  │ VARCHAR(256)│
    │ INTEGER   │  │ BIGINT AUTO │
    │ BLOB      │  │ MEDIUMTEXT  │
    │ "" (无后缀) │  │ ENGINE=... │
    │ ?         │  │ ?           │
    └───────────┘  └─────────────┘
```

**阅读源码时的关键路径：**

打开 `dialect.go`，找到 `Dialect` 接口（第 29 行附近）：

```go
type Dialect interface {
    DriverName() string
    PreConnect(dsn string) error
    ConfigurePool(db *sql.DB, dsn string)
    Validate(db *sql.DB) error
    TypeMap() TypeMap
    TableSuffix() string
    // ...
}
```

然后打开 `dialect_sqlite.go` 和 `dialect_mysql.go`，对比它们对同一个接口的不同实现。例如，`TypeMap()` 方法：

- SQLite 将 `CT_String` 映射为 `TEXT`
- MySQL 将 `CT_String` 映射为 `VARCHAR(256)`

### 3.3 Schema 定义

所有 12 张表的定义集中在 `dialect.go` 的 `schema` 变量中。表定义包含三个部分：

```
表的定义结构：
┌─────────────────────────────────────┐
│ Table                              │
│ ├── Name: "users"                  │
│ ├── Columns: [...]                 │  ← 列定义
│ │   ├── {Name, Type, PK, Unique,   │
│ │   │   NotNull, Default, FK}      │
│ ├── Indexes: [...]                 │  ← 索引定义
│ └── Seeds: [...]                   │  ← 初始数据
└─────────────────────────────────────┘
```

**逻辑列类型** 是一组中间类型常量：

| 逻辑类型 | SQLite 物理类型 | MySQL 物理类型 | 用途 |
|----------|----------------|---------------|------|
| `CT_String` | TEXT | VARCHAR(256) | 通用字符串 |
| `CT_String32` | TEXT | VARCHAR(32) | 短字符串（时间戳 RFC3339） |
| `CT_String64` | TEXT | VARCHAR(64) | 中字符串（UUID、哈希） |
| `CT_Text` | TEXT | TEXT | 长文本 |
| `CT_MediumText` | BLOB | MEDIUMTEXT | 超长文本（stdout/stderr） |
| `CT_Int` | INTEGER | INTEGER | 整数 |
| `CT_BigIntAuto` | INTEGER PK AUTOINCREMENT | BIGINT AUTO_INCREMENT PK | 自增主键 |
| `CT_Double` | REAL | DOUBLE | 浮点数 |

### 3.4 12 张表的关系

```
                    ┌─────────────┐
                    │   users     │  用户（管理员 / 操作员 / 观察者）
                    └──┬──────┬───┘
                       │      │
          ┌────────────┘      └──────────────┐
          ▼                                  ▼
┌──────────────────┐              ┌──────────────────┐
│  web_sessions    │              │ enrollment_codes │  一次性接入码
│  (会话 Cookie)    │              └────────┬─────────┘
└──────────────────┘                       │
                                           ▼
┌──────────────────┐              ┌──────────────────┐
│  devices         │◄─────────────│ device_credentials│  每设备独立凭据
│  (设备清单)       │              └──────────────────┘
└──┬──────┬───────┘
   │      │
   │      └──────────────────┐
   ▼                         ▼
┌──────────────┐    ┌──────────────────┐
│agent_sessions│    │     tasks        │  命令任务
│(连接历史)     │    └────────┬─────────┘
└──────────────┘             │
                             ▼
                    ┌──────────────────┐
                    │  task_results    │  执行结果
                    └──────────────────┘

┌──────────────────┐    ┌──────────────────┐
│command_templates │    │   audit_logs     │  审计日志（独立）
│(命令模板)         │    │                  │
└──────────────────┘    └──────────────────┘

┌──────────────────┐    ┌──────────────────┐
│   llm_config     │    │schema_migrations │  系统表
│ (AI 运维配置)     │    │  (迁移记录)       │
└──────────────────┘    └──────────────────┘
```

### 3.5 阅读 store.go 时的理解框架

`store.go`（约 800 行）包含了所有数据库 CRUD 操作。阅读时按以下分组理解：

| 操作分组 | 涉及表 | 典型方法 |
|----------|--------|----------|
| 设备管理 | devices | `UpsertDevice()`, `ListDevices()`, `ExpireDevices()` |
| 任务管理 | tasks, task_results | `CreateTask()`, `CompleteTask()`, `ListTasks()` |
| 会话管理 | web_sessions | `CreateWebSession()`, `AuthenticateWebSession()` |
| 用户管理 | users | `CreateUser()`, `ListUsers()`, `UpdateUser()` |
| 接入管理 | enrollment_codes, device_credentials | `CreateEnrollmentCode()`, `ValidateEnrollmentCode()` |
| 审计管理 | audit_logs | `CreateAuditLog()`, `ListAuditLogs()` |

---

# 第三部分：前端架构

---

## 第四章 前端整体架构

> **前置章节：** 第二章
> **核心文件：** `web/src/main.tsx`、`router.tsx`、`api/client.ts`、`stores/auth.ts`、`hooks/useLoadable.ts`

### 4.1 技术选型理由

| 技术 | 为什么选它 |
|------|-----------|
| React 18 | 最广泛使用的 UI 框架，生态丰富 |
| TypeScript | 类型安全，减少运行时错误 |
| Vite | 比 Webpack 快 10 倍的开发服务器 |
| Ant Design 5 | 成熟的企业级组件库，中文支持好 |
| Zustand | 极简的状态管理（比 Redux 少 90% 样板代码） |
| Axios | 拦截器机制方便处理认证和错误 |

### 4.2 前端启动流程

当浏览器访问 `http://localhost:5173` 时，以下步骤按顺序发生：

```
main.tsx
  │
  ├─► ReactDOM.createRoot()
  ├─► ConfigProvider (zhCN locale, 蓝色主题 #2563eb)
  ├─► ErrorBoundary (捕获渲染错误)
  └─► RouterProvider
        │
        ▼
      router.tsx
        │
        ├─► /login      → LoginPage (不需要认证)
        └─► /            → RequireAuth
                              │
                              ├─► 检查 useAuthStore.user
                              │   ├─ 有 → 渲染 AppLayout
                              │   └─ 无 → 调用 authApi.me()
                              │       ├─ 成功 → 设置 user，渲染 AppLayout
                              │       └─ 失败 → 重定向到 /login
                              │
                              └─► AppLayout
                                    ├─► Layout.Sider (侧边栏 + 导航菜单)
                                    ├─► Layout.Header (顶栏 + 用户信息)
                                    └─► Layout.Content
                                          └─► <Outlet /> (子路由渲染位置)
```

### 4.3 状态管理：Zustand

```
┌─────────────────────────────────────┐
│          useAuthStore                │
│  ┌─────────────────────────────┐    │
│  │ user: User | null           │    │  ← 当前用户对象
│  │ mustChangePassword: boolean │    │  ← 是否需要改密
│  ├─────────────────────────────┤    │
│  │ setAuth(user, mustChange)   │    │  ← 登录时调用
│  │ setUser(user)               │    │  ← 改密后更新
│  │ clear()                     │    │  ← 登出 / 401 时调用
│  └─────────────────────────────┘    │
│                                     │
│  persist: true  ← 数据保存到        │
│                  localStorage       │
└─────────────────────────────────────┘
```

### 4.4 数据加载模式：useLoadable

LabOps 的前端不使用 React Query 或 SWR，而是实现了一个简洁的自定义 Hook：

```typescript
// 单数据源
function useLoadable<T>(
  fetcher: () => Promise<T>,    // 数据获取函数
  deps: any[],                   // 依赖数组（变化时重新获取）
  intervalMs?: number            // 自动刷新间隔（可选）
): {
  data: T | null,
  loading: boolean,
  error: string | null,
  refresh: () => void             // 手动刷新
}
```

使用示例：

```typescript
// DashboardPage: 自动每 10 秒刷新
const stats = useLoadable(() => labopsApi.stats(), [], 10000);
const devices = useLoadable(() => labopsApi.devices(), [], 10000);
```

**自动刷新的实现原理：**

```
useLoadable 被调用
  │
  ├─► 首次渲染：立即调用 fetcher() → 设置 data
  │
  └─► 如果 intervalMs > 0：
        └─► useEffect 中设置 setInterval
              │
              └─► 每隔 intervalMs 毫秒：
                    └─► 调用 fetcher() → 更新 data → 触发重渲染
```

### 4.5 API 客户端与 CSRF 处理

```
浏览器 ──请求──► Axios 实例
                  │
                  ├─► 请求拦截器
                  │   └─► 读取 labops_csrf cookie
                  │       └─► 设置 X-CSRF-Token 请求头
                  │
                  ├─► 发送 HTTP 请求到 /api/...
                  │
                  └─► 响应拦截器
                      ├─► 200: 返回 data
                      └─► 401: 清除 auth store → 重定向到 /login
```

---

## 第五章 前端页面深度拆解

> **前置章节：** 第四章
> **核心文件：** `web/src/pages/*.tsx`

### 5.1 页面分类

12 个页面按功能分为四组：

```
┌──────────────────────────────────────────────────────┐
│  设备与监控组                                          │
│  ├── DashboardPage    仪表盘（统计 + 最近活动）         │
│  ├── DevicesPage      设备列表                         │
│  ├── DeviceDetailPage 设备详情（实时指标 + 命令执行）    │
│  └── GroupsPage       分组管理                         │
├──────────────────────────────────────────────────────┤
│  任务与审计组                                          │
│  ├── TasksPage        任务创建 + 历史记录               │
│  └── AuditPage        审计日志                         │
├──────────────────────────────────────────────────────┤
│  管理与配置组                                          │
│  ├── EnrollmentPage   设备接入码管理                   │
│  ├── TemplatesPage    命令模板管理                     │
│  ├── UsersPage        用户管理                         │
│  └── AiOpsSettingsPage LLM 配置                       │
├──────────────────────────────────────────────────────┤
│  AI 与分析组                                           │
│  └── AiOpsPage        AI 运维（健康报告 + 推荐）        │
└──────────────────────────────────────────────────────┘
```

### 5.2 DeviceDetailPage 深度解析

`DeviceDetailPage` 是最复杂的页面之一，我们以此为例说明页面组件的一般模式：

```
DeviceDetailPage
│
├─► useParams() 获取设备 ID
├─► useLoadable(deviceApi, [id], 3000)  ← 每 3 秒刷新设备数据
│
├─► 渲染区域一：设备资产信息
│   └─► Descriptions 组件展示 hostname, OS, IP, CPU, 内存, 磁盘
│
├─► 渲染区域二：实时指标
│   └─► Progress 组件展示 CPU%, Memory%, Disk%
│       （数据来自 useLoadable 的自动刷新）
│
├─► 渲染区域三：临时命令执行
│   └─► TextArea + Button
│       └─► 点击执行 → api.createTask({deviceId, command})
│           └─► 需要参数 confirmation: "EXECUTE"  ← 防止误操作
│
└─► 渲染区域四：任务历史
    └─► Table 展示该设备的历史任务
        └─► 可展开行查看 stdout/stderr
```

### 5.3 DashboardPage 的多数据源加载

DashboardPage 同时加载 6 个独立的数据源：

```typescript
// useLoadableAll 并行加载，部分失败不影响其他
const [stats, devices, tasks, audits, report, groups] = useLoadableAll(
  [() => labopsApi.stats(),     // 设备统计
   () => labopsApi.devices(),   // 设备列表（前 6 个）
   () => labopsApi.tasks(),     // 最近任务
   () => labopsApi.auditLogs(), // 最近审计
   () => labopsApi.aiopsReport(), // AI 运维报告
   () => labopsApi.groups()],   // 分组统计
  [],      // deps = [] 表示仅在挂载时加载一次
  10000     // 每 10 秒自动刷新
);
```

### 5.4 任务创建与批量下发

TasksPage 支持两种任务类型：

```
创建任务
│
├─► 选择目标：
│   ├─► 按设备：选择一个 deviceId
│   └─► 按分组：选择一个 groupName → 自动为组内所有在线设备创建任务
│
├─► 选择类型：
│   ├─► 临时命令 (ad_hoc)：
│   │   └─► 需要 confirmation: "EXECUTE"
│   │   └─► 需要 commands:adhoc 权限
│   │
│   └─► 模板命令 (template)：
│       └─► 需要 templates:execute 权限
│       └─► 模板参数自动渲染并校验
│
└─► POST /api/tasks → 服务端创建并下发
```

---

# 第四部分：通信协议

---

## 第六章 WebSocket 通信协议

> **前置章节：** 第一章
> **核心文件：** `server/internal/core/types.go`（Payload 定义）、`agent/cmd/agent/main.go`（Agent 端实现）

### 6.1 为什么用 WebSocket

LabOps 的 Agent 需要满足两个需求：

1. **服务端能随时向 Agent 下发命令** — HTTP 做不到（HTTP 是请求-响应模式，服务端不能主动推送）
2. **Agent 需要持续上报心跳** — 轮询 HTTP 开销太大

WebSocket 解决了这两个问题：建立一个**持久的双向通道**，服务端和 Agent 都可以随时发送消息。

### 6.2 消息格式

所有消息都使用统一的 JSON 信封格式：

```json
{
  "type": "消息类型",
  "payload": { /* 具体内容 */ }
}
```

`type` 字段决定 `payload` 的结构。这是一个经典的**标签联合（Tagged Union）**模式。

### 6.3 消息类型完整定义

```
                    消息流向图

    Agent                                          Server
      │                                               │
      │──── register ──────────────────────────────►  │  连接后立即发送
      │     {agentId, name, hostname, os, ip,         │
      │      cpuCores, memoryMb, diskTotalGb}         │
      │                                               │
      │◄─── registered ─────────────────────────────  │  注册确认
      │     {}                                        │
      │                                               │
      │──── heartbeat ─────────────────────────────►  │  每 10 秒
      │     {cpuUsage, memoryUsage, diskUsage}        │
      │                                               │
      │◄─── command ────────────────────────────────  │  用户创建任务时
      │     {taskId, kind, command/executable,        │
      │      args, timeoutSeconds}                    │
      │                                               │
      │──── task_result ───────────────────────────►  │  命令执行完成
      │     {taskId, status, stdout, stderr,          │
      │      exitCode, durationMs}                    │
      │                                               │
      │◄─── error ──────────────────────────────────  │  出错时
      │     {message}                                 │
```

### 6.4 服务端如何处理消息

打开 `server/internal/core/agent.go`，找到 WebSocket 读循环：

```go
// 简化的消息处理逻辑
for {
    var msg AgentEnvelope
    conn.ReadJSON(&msg)           // 阻塞等待消息

    switch msg.Type {
    case "register":
        // 1. 解析 RegisterPayload
        // 2. UpsertDevice() — 创建或更新设备记录
        // 3. 发送 "registered" 确认
        // 4. dispatchPendingTasks() — 下发待处理任务

    case "heartbeat":
        // 1. 更新 device.cpu_usage, memory_usage, disk_usage
        // 2. 更新 device.last_seen
        // 3. 更新 agent_session

    case "task_result":
        // 1. CompleteTask() — 存储结果
        // 2. 创建审计日志
    }
}
```

### 6.5 Agent 如何处理消息

打开 `agent/cmd/agent/main.go`，找到 `run()` 函数中的读循环：

```go
for {
    var msg incomingEnvelope
    conn.ReadJSON(&msg)

    switch msg.Type {
    case "registered":
        log.Printf("registered with server")

    case "command":
        // 1. 解析 CommandPayload
        // 2. 验证 taskId 和 command 非空
        // 3. go executeAndReport(send, cmd)  ← goroutine 异步执行

    case "error":
        return fmt.Errorf("server sent error, reconnecting")
    }
}
```

---

# 第五部分：后端核心

---

## 第七章 服务端启动流程

> **前置章节：** 第三章、第六章
> **核心文件：** `server/cmd/server/main.go`、`server/internal/core/app.go`（NewApp 函数）

### 7.1 从 main.go 追踪初始化

打开 `server/cmd/server/main.go`，追踪启动流程：

```
main()
 │
 ├─► 1. 解析环境变量
 │   ├─► LABOPS_ADDR           (监听地址，默认 :8080)
 │   ├─► LABOPS_DB_DRIVER      (数据库驱动，默认 sqlite)
 │   ├─► LABOPS_DB_PATH / LABOPS_MYSQL_DSN
 │   ├─► LABOPS_BOOTSTRAP_ADMIN_PASSWORD
 │   ├─► LABOPS_ENCRYPTION_KEY
 │   ├─► LABOPS_ENV            (环境名)
 │   ├─► LABOPS_PUBLIC_ORIGIN  (CORS Origin)
 │   ├─► LABOPS_HEARTBEAT_TIMEOUT
 │   ├─► LABOPS_TASK_TIMEOUT
 │   └─► LABOPS_LLM_URL / LABOPS_LLM_API_KEY
 │
 ├─► 2. 选择数据库方言
 │   └─► if "mysql" → MySQLDialect, else → SQLiteDialect
 │
 ├─► 3. sql.Open() → dialect.Validate()
 │
 ├─► 4. 自动建表
 │   └─► for each table in schema:
 │       └─► db.Exec(CREATE TABLE IF NOT EXISTS ...)
 │
 ├─► 5. 运行迁移
 │   └─► for each migration version:
 │       └─► if not applied → run migration SQL
 │
 ├─► 6. 插入种子数据
 │   └─► INSERT OR IGNORE INTO llm_config ...
 │   └─► INSERT OR IGNORE INTO command_templates ... (5 个模板)
 │
 ├─► 7. 创建或更新管理员
 │   └─► if users 表为空 OR admin/admin 密码仍为旧值:
 │       └─► UpsertUser("admin", bootstrapPassword, roles=["admin"])
 │       └─► 设置 must_change_password = true
 │
 ├─► 8. 创建 App 实例
 │   └─► NewApp(store, config)
 │       ├─► 初始化 WebSocket upgrader
 │       ├─► 创建 Analyzer (AI 运维引擎)
 │       │   └─► analyzer.Start() → 启动 30 分钟定时分析
 │       └─► go maintenanceLoop() → 启动 10 秒维护循环
 │
 ├─► 9. 启动 HTTP 服务器
 │   └─► http.Server{Addr: LABOPS_ADDR, Handler: app.Handler()}
 │
 └─► 10. 等待信号 → 优雅关闭
     └─► signal.Notify(sigint, sigterm)
         └─► server.Shutdown() → app.Stop()
```

### 7.2 App 结构体：系统的中心

```go
type App struct {
    store    *Store              // 数据库操作
    config   Config              // 配置（从环境变量解析）
    upgrader websocket.Upgrader   // HTTP → WebSocket 升级器
    analyzer *Analyzer            // AI 运维分析引擎

    mu      sync.RWMutex
    clients map[string]*AgentClient  // deviceId → AgentClient
    // ↑ 这就是 Agent 连接池！
    //   每个活跃的 WebSocket 连接对应一个 AgentClient

    rateLimiters map[string]*rateLimiter  // IP → 速率限制器
}
```

---

## 第八章 HTTP 路由与中间件链

> **前置章节：** 第七章
> **核心文件：** `server/internal/core/app.go`（Handler、withCORS、withAuth、withRateLimit）

### 8.1 Go 1.22+ 路由模式

LabOps 使用 Go 1.22 引入的模式匹配路由：

```go
mux.HandleFunc("GET /api/devices/{id}", handler)
//               ↑ method  ↑ path with variable
```

`{id}` 是一个路径变量，在处理器中通过 `r.PathValue("id")` 获取。

### 8.2 完整的路由表

以下是 `Handler()` 方法中注册的全部路由（按类别排列）：

```
健康检查：
  GET  /api/health                          → handleHealth

认证：
  POST /api/auth/login                      → handleLogin
  POST /api/auth/logout                     → handleLogout
  POST /api/auth/change-password            → handleChangePassword
  GET  /api/auth/me                         → handleMe

用户管理：
  GET  /api/users                           → handleListUsers
  POST /api/users                           → handleCreateUser
  PUT  /api/users/{id}                      → handleUpdateUser

设备管理：
  GET  /api/stats                           → handleStats
  GET  /api/devices                         → handleListDevices
  GET  /api/devices/{id}                    → handleGetDevice
  GET  /api/devices/{id}/tasks              → handleListDeviceTasks
  POST /api/devices                         → handleCreateDevice
  DELETE /api/devices/{id}                  → handleDeleteDevice
  POST /api/devices/{id}/revoke             → handleRevokeDevice

接入管理：
  GET  /api/enrollment-codes                → handleListEnrollmentCodes
  POST /api/enrollment-codes                → handleCreateEnrollmentCode
  DELETE /api/enrollment-codes/{id}         → handleRevokeEnrollmentCode
  POST /api/agent/enroll                    → handleAgentEnroll

分组与任务：
  GET  /api/groups                          → handleGroups
  GET  /api/tasks                           → handleListTasks
  POST /api/tasks                           → handleCreateTask
  GET  /api/tasks/{id}                      → handleGetTask

命令模板：
  GET  /api/command-templates               → handleListCommandTemplates
  POST /api/command-templates               → handleCreateCommandTemplate
  PUT  /api/command-templates/{id}          → handleUpdateCommandTemplate

AI 运维：
  GET  /api/aiops/report                    → handleAiOpsReport
  GET  /api/aiops/llm-config                → handleGetLLMConfig
  PUT  /api/aiops/llm-config                → handleSaveLLMConfig
  POST /api/aiops/llm-test                  → handleTestLLM
  POST /api/aiops/recommendations/execute   → handleExecuteRecommendation
  GET  /api/aiops/auto-mode                 → handleGetAutoMode
  PUT  /api/aiops/auto-mode                 → handleSaveAutoMode

审计：
  GET  /api/audit-logs                      → handleAuditLogs

Agent WebSocket：
  GET  /api/agent/ws                        → handleAgentWS
```

### 8.3 中间件洋葱模型

请求经过的中间件顺序（从外到内）：

```
      HTTP 请求
          │
    ┌─────▼──────┐
    │ withRequestID │  ← 注入 X-Request-ID（或从请求头读取）
    └─────┬──────┘
          │
    ┌─────▼──────┐
    │  withCORS     │  ← Origin 校验 + CORS 响应头
    └─────┬──────┘
          │
    ┌─────▼──────┐
    │withRateLimit  │  ← IP 令牌桶限流
    └─────┬──────┘
          │
    ┌─────▼──────┐
    │  withAuth     │  ← Session 认证 + CSRF 校验 + 权限检查
    └─────┬──────┘
          │
    ┌─────▼──────┐
    │  Handler      │  ← 业务逻辑处理器
    └────────────┘
```

嵌套调用方式（Go 惯用的中间件模式）：

```go
return a.withRequestID(a.withCORS(a.withRateLimit(a.withAuth(mux))))
//                       ↑         ↑              ↑
//                       每一层返回一个包装了下一层的 http.Handler
```

### 8.4 认证中间件深度解析

`withAuth()` 是系统中最复杂的中间件。它的决策树：

```
请求进入 withAuth
  │
  ├─► 是 OPTIONS 请求？ → 放行（CORS 预检）
  ├─► 路径是公开端点？  → 放行
  │   (health, login, agent/enroll, agent/ws)
  │
  ├─► 有 WebToken（旧版，仅内部测试）？
  │   └─► 是 → 注入 admin 上下文，放行
  │
  ├─► 生产模式但无 WebToken？
  │   └─► 是（开发模式）→ 注入 admin 上下文，放行
  │
  └─► 正常认证流程：
      │
      ├─► 读取 labops_session Cookie
      │   └─► 无 → 401 AUTH_REQUIRED
      │
      ├─► AuthenticateWebSession(tokenHash)
      │   ├─► 会话不存在 → 401 SESSION_EXPIRED
      │   ├─► 空闲超时（8h）→ 401 SESSION_EXPIRED
      │   └─► 绝对过期（24h）→ 401 SESSION_EXPIRED
      │
      ├─► 是状态变更请求？（POST/PUT/DELETE）
      │   └─► 检查 CSRF：
      │       ├─► 读取 labops_csrf Cookie
      │       ├─► 读取 X-CSRF-Token Header
      │       ├─► Cookie 值 == Header 值？
      │       └─► SHA-256(Header) == 存储的 CSRF Hash？
      │           └─► 否 → 403 CSRF_INVALID
      │
      ├─► must_change_password？
      │   └─► 且路径不是 change-password/me/logout？
      │       └─► 是 → 403 PASSWORD_CHANGE_REQUIRED
      │
      ├─► 需要特定权限？
      │   └─► requiredPermission(r) 返回需要的权限
      │       └─► HasPermission(user, permission)？
      │           └─► 否 → 403 PERMISSION_DENIED
      │
      └─► 全部通过 → 注入用户上下文，放行
```

---

## 第九章 REST API 处理器

> **前置章节：** 第八章
> **核心文件：** `server/internal/core/api.go`

### 9.1 处理器的一般模式

每个 API 处理器的结构遵循统一的模式：

```
func (a *App) handleXxx(w http.ResponseWriter, r *http.Request) {
    // 1. 获取认证上下文
    ctx := GetAuthContext(r.Context())
    //    ↑ 由 withAuth 中间件注入

    // 2. 解析请求参数
    //    - 路径参数: r.PathValue("id")
    //    - 查询参数: r.URL.Query().Get("q")
    //    - JSON Body: json.NewDecoder(r.Body).Decode(&req)
    //    - Body 大小限制: io.LimitReader(r.Body, 1<<20) ← 1 MiB

    // 3. 调用 Store 层
    devices := a.store.ListDevices(r.Context())

    // 4. 记录审计日志（如有状态变更）
    a.store.CreateAuditLog(r.Context(), auditLog)

    // 5. 返回 JSON 响应
    writeJSON(w, http.StatusOK, devices)
    //    ↑ 自动设置 Content-Type: application/json
}
```

### 9.2 重点处理器分析：handleCreateTask

`handleCreateTask` 是最复杂、最核心的处理器。让我们详细拆解它的逻辑：

```
handleCreateTask(w, r)
  │
  ├─► 1. 获取当前用户
  │   └─► ctx := GetAuthContext(r.Context())
  │
  ├─► 2. 解析请求体
  │   └─► json.Decode(into CreateTaskRequest{
  │          DeviceID, GroupName, Command,
  │          Kind, TemplateID, Confirmation
  │       })
  │
  ├─► 3. 安全检查
  │   ├─► ad_hoc 任务：
  │   │   ├─► 需要 commands:adhoc 权限
  │   │   └─► 必须包含 "confirmation": "EXECUTE"
  │   │
  │   └─► template 任务：
  │       ├─► 需要 templates:execute 权限
  │       └─► 渲染模板参数（校验 enum/regex/range）
  │
  ├─► 4. 创建任务
  │   ├─► 按设备：
  │   │   └─► store.CreateTask(deviceId, command, ...)
  │   │
  │   └─► 按分组：
  │       └─► for each online device in group:
  │           └─► store.CreateTask(deviceId, command, ...)
  │
  ├─► 5. 记录审计日志
  │   └─► store.CreateAuditLog(action="command.create", ...)
  │
  ├─► 6. 下发到 Agent
  │   └─► for each created task:
  │       └─► app.dispatchTask(task)
  │           │
  │           ├─► 查找 AgentClient (deviceId → app.clients[deviceId])
  │           │
  │           ├─► Agent 在线：
  │           │   ├─► 发送 "command" envelope via WebSocket
  │           │   └─► 更新任务状态 → "running"
  │           │
  │           └─► Agent 离线：
  │               └─► 任务保持 "pending"（等 Agent 重连时下发）
  │
  └─► 7. 返回响应
      └─► writeJSON(w, 201, createdTasks)
```

### 9.3 Agent 重连时的任务下发

当 Agent 断线重连后，服务端在 `handleAgentWS` 的 register 处理中调用：

```go
func (a *App) dispatchPendingTasks(deviceId string) {
    tasks := a.store.ListPendingTasks(deviceId)
    for _, task := range tasks {
        a.dispatchTask(task)
    }
}
```

这确保了离线期间创建的任务不会丢失。

---

## 第十章 用户认证与会话管理

> **前置章节：** 第八章
> **核心文件：** `server/internal/core/auth_context.go`、`security_store.go`

### 10.1 为什么不使用 JWT

LabOps 使用 **Session Cookie** 而不是 JWT，原因是：

| 特性 | Session Cookie | JWT |
|------|:---:|:---:|
| 服务端可随时撤销 | ✅ 是 | ❌ 否（需黑名单） |
| 单次改密后所有设备失效 | ✅ 是 | ❌ 否 |
| 服务端存储开销 | ❌ 需要 | ✅ 不需要 |
| 水平扩展 | ❌ 需要共享存储 | ✅ 不需要 |

对于 LabOps 这种**单实例部署**的场景，Session Cookie 的「服务端可控」优势远大于 JWT 的「无状态」优势。

### 10.2 登录流程

```
POST /api/auth/login
  │  Body: {username, password}
  │
  ├─► 1. 查找用户 → store.GetUserByUsername(username)
  │   └─► 用户不存在 → 401 INVALID_CREDENTIALS
  │
  ├─► 2. 验证密码 → bcrypt.CompareHashAndPassword(hash, password)
  │   └─► 密码错误 → 401 INVALID_CREDENTIALS
  │
  ├─► 3. 检查用户状态
  │   └─► status = "disabled" → 403 ACCOUNT_DISABLED
  │
  ├─► 4. 生成令牌
  │   ├─► sessionToken = randomBytes(32)  ← 256 位随机
  │   └─► csrfToken = randomBytes(32)
  │
  ├─► 5. 存储会话 → store.CreateWebSession(...)
  │   └─► 存储的是 SHA-256(token)，不是原始 token
  │
  ├─► 6. 设置 Cookie
  │   ├─► Set-Cookie: labops_session=<sessionToken>; HttpOnly; Secure; SameSite=Strict
  │   └─► Set-Cookie: labops_csrf=<csrfToken>; Secure; SameSite=Strict
  │       ↑ 注意：labops_csrf 不是 HttpOnly，因为 JS 需要读取它
  │
  └─► 7. 返回用户信息
      └─► {user: {id, username, roles, permissions, mustChangePassword}}
```

### 10.3 密码强制修改流程

```
┌─────────────────────────────────────────────────────┐
│  首次登录 / 密码被重置                                 │
│      │                                              │
│      ▼                                              │
│  must_change_password = true                        │
│      │                                              │
│      ▼                                              │
│  用户访问任何页面 → 被重定向到改密页面                   │
│      │                                              │
│      ▼                                              │
│  POST /api/auth/change-password                      │
│  Body: {oldPassword, newPassword}                   │
│      │                                              │
│      ├─► 验证 oldPassword 正确                       │
│      ├─► 验证 newPassword ≥ 12 字符                  │
│      ├─► 验证 newPassword ≠ 最近使用过的密码            │
│      ├─► bcrypt 哈希新密码                           │
│      ├─► 设置 must_change_password = false           │
│      └─► 删除该用户的所有旧会话，创建新会话              │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 10.4 RBAC 权限模型

```
角色定义：
  admin     → 全部 8 个权限
  operator  → system:read + templates:execute
  viewer    → system:read

权限列表：
  system:read           → 查看设备、任务、审计日志
  system:users          → 管理用户（创建、修改角色）
  system:enrollment     → 管理接入码
  system:device-revoke  → 删除设备、吊销凭据
  templates:manage      → 管理命令模板（增删改）
  templates:execute     → 执行模板命令
  commands:adhoc        → 执行临时命令
  aiops:llm             → 管理 LLM 配置 + 执行推荐
```

权限检查在 `requiredPermission()` 函数中，通过请求的**路径和 HTTP 方法**确定需要的权限。例如：

- `DELETE /api/devices/{id}` → 需要 `system:device-revoke`
- `POST /api/command-templates` → 需要 `templates:manage`
- `GET /api/devices` → 需要 `system:read`（所有角色都有）

---

# 第六部分：Agent 系统

---

## 第十一章 Agent WebSocket 服务端

> **前置章节：** 第六章、第八章
> **核心文件：** `server/internal/core/agent.go`

### 11.1 AgentClient 结构体

```go
type AgentClient struct {
    deviceID string          // 设备 ID
    conn     *websocket.Conn  // WebSocket 连接
    send     chan []byte       // 发送通道（并发安全）
    hub      *App             // 反向引用
}
```

`send` 通道的设计目的是**避免并发写入 WebSocket 连接**。所有需要发送消息的地方都通过这个通道：

```
多个 goroutine
    │
    ├─► agentClient.send <- message  ← 线程安全！
    │
    └─► 专用写 goroutine
          │
          └─► conn.WriteMessage(websocket.TextMessage, msg)
              ↑ 只有一个 goroutine 在写 → 无竞态
```

### 11.2 连接生命周期

```
新 WebSocket 连接到达
  │
  ├─► handleAgentWS(w, r)
  │   │
  │   ├─► 1. 认证 Agent
  │   │   ├─► 读取 Authorization: Agent <deviceId>:<secret>
  │   │   └─► ValidateDeviceCredential(deviceId, secret)
  │   │       └─► 失败 → 关闭连接
  │   │
  │   ├─► 2. HTTP Upgrade → WebSocket
  │   │   └─► upgrader.Upgrade(w, r, nil)
  │   │       └─► CheckOrigin: 验证 Origin 匹配 PUBLIC_ORIGIN
  │   │
  │   ├─► 3. 注册客户端
  │   │   ├─► 检查是否已有同 deviceId 连接 → 关闭旧连接
  │   │   ├─► app.clients[deviceID] = agentClient
  │   │   └─► go agentClient.writePump()  ← 写 goroutine
  │   │
  │   └─► 4. 进入读循环
  │       └─► agentClient.readPump()
  │           │
  │           ├─► 读取消息...
  │           ├─► 处理 register/heartbeat/task_result...
  │           │
  │           └─► 连接断开
  │               └─► unregisterClient()
  │                   ├─► delete(app.clients, deviceID)
  │                   ├─► 标记设备 offline
  │                   └─► 记录 agent_session.disconnected_at
```

### 11.3 解注册时的处理

```go
func (a *App) unregisterClient(deviceID string) {
    // 1. 从连接池移除
    delete(a.clients, deviceID)

    // 2. 立即标记设备离线（不等心跳超时）
    a.store.SetDeviceStatus(deviceID, StatusOffline)

    // 3. 更新 agent_session
    a.store.DisconnectAgentSession(deviceID)
}
```

注意：Agent 正常断开时立即标记离线。但如果是网络瞬断（没有发送 close frame），服务端会在下一个 `maintenanceLoop` 轮次（最多 10 秒后）通过心跳超时检测到。

---

## 第十二章 Agent 客户端实现

> **前置章节：** 第六章、第十一章
> **核心文件：** `agent/cmd/agent/main.go`

### 12.1 Agent 启动流程

Agent 是一个独立的 Go 程序，编译为单个静态二进制文件。运行流程如下：

```
main()
  │
  ├─► parseFlags()
  │   ├─► 解析命令行参数和环境变量
  │   ├─► 尝试加载本地凭据文件 (credentials.json)
  │   └─► 返回 config 结构体
  │
  ├─► 有 --enroll-code？
  │   └─► 是 → enroll(cfg)
  │       ├─► POST /api/agent/enroll → 获得 deviceId + deviceSecret
  │       ├─► saveCredentials() → 写入本地文件
  │       └─► 如果 --enroll-only → 退出
  │
  ├─► 没有凭据也没有 enroll-code？
  │   └─► log.Fatal("run with --enroll-code first")
  │
  └─► 连接循环（带指数退避）
      │
      └─► for {
            ├─► run(cfg)    ← 核心连接逻辑
            │   └─► 成功或出错后返回
            │
            └─► 出错 → sleep(backoff)
                └─► backoff = min(backoff * 2, 60s)
                └─► 成功 → backoff 重置为 1s
          }
```

### 12.2 run() 函数详解

```go
func run(cfg config) error {
    // 1. 构建 WebSocket URL
    //    http://host → ws://host/api/agent/ws
    //    https://host → wss://host/api/agent/ws

    // 2. 拨号 WebSocket
    //    Header: Authorization: Agent <deviceId>:<secret>
    conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)

    // 3. 发送注册消息
    conn.WriteJSON(envelope{Type: "register", Payload: buildRegister(cfg)})

    // 4. 启动心跳 goroutine
    //    每 10 秒发送 CPU%/Memory%/Disk%
    go heartbeatLoop(ctx, send, cancel, collectMetrics)

    // 5. 读循环
    for {
        conn.ReadJSON(&msg)
        switch msg.Type {
        case "registered":  // 注册确认
        case "command":     // 执行命令
            go executeAndReport(send, cmd)
        case "error":       // 服务端错误 → 重连
            return err
        }
    }
}
```

### 12.3 命令执行详解

```
executeAndReport(send, cmd)
  │
  ├─► defer 恢复 panic → 发送 failed task_result
  │
  ├─► executePayload(cmd)
  │   │
  │   ├─► ad_hoc 命令：
  │   │   ├─► Windows: cmd /C <command>
  │   │   └─► Unix:    /bin/sh -c <command>
  │   │
  │   └─► template 命令：
  │       ├─► 验证 executable 是绝对路径
  │       └─► exec.CommandContext(ctx, executable, args...)
  │
  ├─► 超时控制：
  │   └─► context.WithTimeout(ctx, timeoutSeconds)
  │       └─► 超时 → 返回 exit code 124 + "timed out" 错误
  │
  ├─► 输出截断：
  │   ├─► stdout 截断到 256KB
  │   └─► stderr 截断到 256KB
  │
  └─► 发送 task_result
      └─► send(envelope{Type: "task_result", Payload: result})
```

### 12.4 真实指标收集

当 Agent 使用 `--real` 标志运行时，调用 `gopsutil/v4` 库收集真实的系统指标：

```
collectMetrics()
  │
  ├─► CPU:
  │   └─► cpu.Percent(0, false)  ← 需要先用 cpu.Percent(time.Second, false) 预热
  │
  ├─► Memory:
  │   └─► mem.VirtualMemory().UsedPercent
  │
  └─► Disk:
      └─► disk.Partitions(false)
          └─► disk.Usage(firstPartition.Mountpoint).UsedPercent
```

### 12.5 模拟指标生成

没有 `--real` 标志时，Agent 使用模拟数据：

```
mockHeartbeat(profileName)
  │
  ├─► 获取配置文件基线：
  │   ├─► ubuntu:       CPU 15%, Mem 38%, Disk 29%
  │   ├─► windows-lab:  CPU 22%, Mem 48%, Disk 61%
  │   ├─► server:       CPU 18%, Mem 41%, Disk 37%
  │   └─► edge-node:    CPU 35%, Mem 55%, Disk 44%
  │
  └─► 每个值加上随机抖动：base + rand(0,18) - 6
      └─► 截断到 [1, 99] 范围
```

---

# 第七部分：AI 运维

---

## 第十三章 AI 运维分析引擎

> **前置章节：** 第七章
> **核心文件：** `server/internal/core/analyzer.go`、`llm.go`

### 13.1 分析流水线

```
Analyzer 运行（每 30 分钟 + 启动时 + 配置变更时）
  │
  ├─► 阶段一：规则引擎分析
  │   │
  │   ├─► 遍历所有设备
  │   ├─► 计算健康分（初始 100 分）
  │   │   ├─► 离线     → -40
  │   │   ├─► CPU>80%  → -20
  │   │   ├─► CPU>60%  → -10
  │   │   ├─► Mem>80%  → -20
  │   │   ├─► Mem>60%  → -10
  │   │   ├─► Disk>85% → -15
  │   │   ├─► 任务失败率>50% → -20
  │   │   └─► 任务失败率>20% → -10
  │   │
  │   └─► 生成报告（含每个设备的评分和扣分原因）
  │
  ├─► 阶段二：LLM 增强分析（如果配置了 LLM）
  │   │
  │   ├─► 构建 Prompt（中文）
  │   │   └─► "你是一位经验丰富的实验室设备运维专家..."
  │   │
  │   ├─► 发送设备指标 + 任务统计 → LLM
  │   │
  │   ├─► 解析 LLM 响应 → 结构化推荐
  │   │   └─► {deviceId, command, reason, priority, isMutation}
  │   │
  │   ├─► 安全校验
  │   │   ├─► 验证 deviceId 是否真实存在
  │   │   └─► isDangerousCommand(command)？
  │   │       └─► 过滤 rm -rf, mkfs, dd, shutdown 等危险命令
  │   │
  │   └─► 添加到报告
  │
  └─► 阶段三：自动执行（如果启用了 auto-execute）
      │
      └─► for each recommendation:
          └─► if isMutation == false（只读命令）:
              └─► 自动 dispatchTask()
```

### 13.2 LLM 客户端设计

```go
// 支持两种 LLM 提供商
type LLMProvider string
const (
    ProviderOpenAI   LLMProvider = "openai"
    ProviderAnthropic LLMProvider = "anthropic"
)

// 统一的请求接口
func AnalyzeDevicesStructured(provider, url, apiKey, model string, devices []Device) (*AnalysisResult, error) {
    // 内部根据 provider 类型选择：
    // - openai:    POST {url}/v1/chat/completions
    //              格式: {messages: [{role: "user", content: prompt}]}
    // - anthropic: POST {url}/v1/messages
    //              格式: {messages: [{role: "user", content: prompt}], model: ...}
}
```

### 13.3 危险命令检测

`isDangerousCommand()` 函数维护了一个黑名单：

```go
var dangerousPatterns = []string{
    "rm -rf /", "rm -rf /*", "rm -rf ~",
    "mkfs", "mke2fs",
    "dd if=",
    "shutdown", "reboot", "halt", "poweroff",
    "fork bomb", ":(){ :|:& };:",
    "chmod 777 /",
    "> /dev/sda", "> /dev/nvme",
    // ... 更多
}
```

任何 LLM 推荐的命令如果匹配这些模式，将被自动丢弃。

---

# 第八部分：生产部署

---

## 第十四章 生产部署架构

> **前置章节：** 第七章
> **核心文件：** `compose.yaml`、`web/nginx/default.conf.template`、`deploy/systemd/*`

### 14.1 Docker Compose 服务拓扑

生产环境的 `compose.yaml` 定义了三个服务和三个网络：

```
                    ┌──────────────┐
      Internet ────►│ edge network │◄──── web 容器（Nginx :80/:443）
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │backend network│◄─── mysql + server + web（内部通信）
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │egress network│◄─── server（访问外部 LLM API）
                    └──────────────┘
```

关键安全设计：
- MySQL 的 3306 端口**不暴露到宿主机**
- server 的 8080 端口**不暴露到宿主机**
- 只有 web (Nginx) 的 80/443 端口对外
- 所有后端通信都在 Docker 内部网络中进行

### 14.2 Nginx 配置模板

`web/nginx/default.conf.template` 使用 envsubst 在容器启动时替换环境变量：

```nginx
# HTTP → HTTPS 重定向
server {
    listen 80;
    server_name ${SERVER_HOST};
    # ACME 验证路径
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }
    # 其他所有请求重定向到 HTTPS
    location / {
        return 301 https://$host$request_uri;
    }
}

# HTTPS
server {
    listen 443 ssl http2;
    server_name ${SERVER_HOST};

    # TLS 证书
    ssl_certificate     ${TLS_CERT_PATH};
    ssl_certificate_key ${TLS_KEY_PATH};

    # API 代理（含 WebSocket 升级）
    location /api/ {
        proxy_pass http://server:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # 静态文件 + SPA 回退
    location / {
        root /usr/share/nginx/html;
        try_files $uri $uri/ /index.html;
    }
}
```

### 14.3 Agent systemd 单元

`deploy/systemd/labops-agent.service` 的关键安全设置：

```
[Service]
User=labops-agent
NoNewPrivileges=true        ← 禁止提权
PrivateTmp=true             ← 独立 /tmp
ProtectSystem=strict        ← /usr、/boot、/etc 只读
ProtectHome=true            ← 禁止访问 /home、/root
ReadWritePaths=/var/lib/labops-agent  ← 唯一可写路径
RestrictSUIDSGID=true       ← 禁止 setuid
```

### 14.4 维护循环

`maintenanceLoop()` 每 10 秒运行一次：

```
每 10 秒：
  ├─► ExpireDevices()
  │   └─► 遍历 devices 表
  │       └─► last_seen < now - 35s → status = "offline"
  │
  ├─► TimeoutTasks()
  │   └─► 遍历 tasks 表
  │       └─► status IN ("pending","running") AND created_at < now - 5min
  │           └─► status = "timeout"
  │
  ├─► PruneExpiredWebSessions()
  │   └─► 删除 idle_expires_at 或 absolute_expires_at 已过期的会话
  │
  └─► PruneRateLimiters()
      └─► 删除 30 分钟无活动的限流器
```

---

# 第九部分：总结

---

## 第十五章 完整学习路径

### 15.1 已覆盖的知识体系

通过以上十四章，你已经系统性地学习了 LabOps 的：

```
┌─────────────────────────────────────────────────┐
│              知识体系全景图                        │
├─────────────────────────────────────────────────┤
│  数据库层：方言抽象 → Schema 设计 → 自动迁移       │
│  前端层：路由 → 状态 → 页面 → 数据加载             │
│  通信层：WebSocket 协议 → 消息类型 → 序列化         │
│  服务层：启动 → 路由 → 中间件 → 处理器              │
│  安全层：认证 → 会话 → CSRF → RBAC                │
│  Agent层：连接 → 心跳 → 命令 → 模拟/真实指标        │
│  AI运维：规则引擎 → LLM → 推荐 → 自动执行           │
│  部署层：Compose → Nginx → Systemd → 备份         │
└─────────────────────────────────────────────────┘
```

### 15.2 关键设计决策回顾

| 决策 | 理由 |
|------|------|
| 不使用 Web 框架（gin/echo/chi） | 最简依赖，Go 标准库已足够 |
| SQLite 用于开发，MySQL 用于生产 | SQLite 零配置，MySQL 可靠 |
| Session Cookie 替代 JWT | 会话可撤销，更适合单实例 |
| 每设备独立凭据（非共享 token） | 一台设备被攻破不影响其他 |
| 临时命令需要确认键 | 防止误操作执行危险命令 |
| Go 1.22+ 模式路由 | 标准库原生支持，无需第三方 |
| 单文件 Agent + 静态编译 | 部署只需复制一个二进制文件 |

### 15.3 进阶阅读建议

完成源码阅读后，可以：

1. **尝试添加一个新页面** — 例如添加一个「系统日志」页面，从零开始完成前后端
2. **添加一个新的 API 端点** — 例如 `GET /api/devices/{id}/metrics/history`
3. **扩展命令模板** — 添加一个 Windows 平台的模板
4. **实现 v0.3 的文件分发功能** — 设计文档在 `docs/features/file-distribution/design.md`
5. **为 Agent 添加真实的网络指标** — 使用 gopsutil 的网络 API

### 15.4 技术栈全景图

```
┌──────────────────────────────────────────────────────────┐
│                    LabOps v0.2 技术栈                      │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Web Console (TypeScript)                          │   │
│  │ React 18 ─► Ant Design 5 ─► Zustand ─► Axios     │   │
│  │ react-router-dom ─► Vite ─► dayjs ─► vitest      │   │
│  └──────────────────────┬───────────────────────────┘   │
│                         │ HTTPS                         │
│  ┌──────────────────────▼───────────────────────────┐   │
│  │ Nginx 1.27 (TLS + Reverse Proxy)                 │   │
│  │ Let's Encrypt ─► HTTP/2 ─► HSTS ─► SPA fallback  │   │
│  └──────────────────────┬───────────────────────────┘   │
│                         │ HTTP (internal)               │
│  ┌──────────────────────▼───────────────────────────┐   │
│  │ Go Server 1.25 (net/http)                         │   │
│  │ gorilla/websocket ─► bcrypt ─► AES-256-GCM       │   │
│  │ modernc.org/sqlite ─► go-sql-driver/mysql         │   │
│  └──────┬───────────────────────────────┬───────────┘   │
│         │ SQL                           │ WebSocket     │
│  ┌──────▼──────┐              ┌─────────▼──────────┐    │
│  │ MySQL 8.0   │              │ Agent (Go 1.24)     │    │
│  │ (生产)       │              │ gorilla/websocket   │    │
│  │ SQLite      │              │ gopsutil v4         │    │
│  │ (开发)       │              │ os/exec             │    │
│  └─────────────┘              └────────────────────┘    │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ DevOps                                            │   │
│  │ Docker Compose ─► GitHub Actions ─► Systemd       │   │
│  │ Certbot ─► mysqldump ─► NSSM (Windows)           │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘
```

---

> **全书完。** 如果你有任何问题或发现文档中的错误，欢迎提交 Issue 或 PR。
>
> 项目地址：[https://github.com/cowhorse05/LabOps](https://github.com/cowhorse05/LabOps)
> 在线演示：[https://cowhorse.xyz](https://cowhorse.xyz)
