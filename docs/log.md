# LabOps 变更日志

## 2026-07-08 Round 2 — useLoadable 重构 + 剩余 P1 修复

### 审查范围

基于 Round 1 的 80+ 问题审计结果，本轮聚焦 deferred 的 P1 项，不再做全量审查。

### 本轮修复清单

#### Web 重构 (useLoadable hook 应用)
- [x] **GroupsPage.tsx** — 手动 loading 模式 → `useLoadable(() => labopsApi.groups(), { intervalMs: 10000 })`，新增 10s 自动刷新
- [x] **AuditPage.tsx** — 同上 → `useLoadable(..., { intervalMs: 15000 })`，新增 15s 自动刷新
- [x] **DevicesPage.tsx** — 手动 setInterval → `useLoadable(..., { intervalMs: 10000 })`，消除 useEffect 样板
- [x] **status.ts** — 添加显式 `case 'offline': return 'default'`，提高代码自文档性

#### Agent (Go)
- [x] **main.go:78-93** — 重连固定 3s → 指数退避 (1s → 2s → 4s → ... → max 60s)，成功连接后重置
- [x] **main.go:191-196** — `executeAndReport` 新增 `defer recover()` 防止 panic 导致 agent 进程崩溃
- [x] **main.go:304** — `jitter()` 截断 → `math.Round()` 四舍五入

#### Server (Go)
- [x] **store.go:498** — `newID` fallback 时间戳 → `time.Now().UnixNano()`，消除秒级碰撞风险
- [x] **api.go:177-181** — `handleCreateTask` 中 `GetTask` 错误不再被忽略

#### 基础设施
- [x] **agent/Dockerfile:12** — 添加 `ca-certificates` 包，为未来 TLS/WSS 支持做准备
- [x] **server/Dockerfile:21** — 内建 `HEALTHCHECK` 指令，不依赖 compose.yaml

### 验证结果

- [x] `npm run build` (tsc + vite build) — **通过**
- [ ] `go test ./...` — **待环境**（同 Round 1 阻塞原因）
- [ ] Docker Compose 集成测试 — **待环境**

### 自检

**有什么没想到的？**
- `useLoadable` 返回值是 T|null，页面需要 `?? [] ` 守卫。GroupsPage/AuditPage/DevicesPage 已加上，但 DashboardPage 和 TasksPage 还没迁移。
- Agent 心跳修复引入 `cancel` 参数传递在 Round 1 就做了，Round 2 的 `run()` 返回 nil 时重置 backoff 依赖心跳错误触发 cancel——心跳失败 → cancel → run 返回 nil → backoff 重置。逻辑链中如果未来新增 ctx cancel 的其他触发源，可能意外重置 backoff。

**有什么疏漏？**
- DashboardPage 还没迁移到 `useLoadable`，仍用手动 `useState` x5 + `Promise.allSettled`
- DevicesPage 的搜索功能保留了手动 `useState` + `useMemo`，与 `useLoadable` 兼容良好
- `useLoadable` hook 里 console.error 还不够——没有用户可见的错误提示

**如何改进？**
- 下一轮（如需要）：迁移 DashboardPage 到 `useLoadableAll`，补 DeviceDetailPage + TasksPage
- 为 `useLoadable` 添加可选的 `onError` 回调，让页面可以展示错误 Toast

**Todolist 更新**：
- [x] Round 1 deferred: useLoadable 重构、auto-refresh、panic recover、backoff、newID、GetTask error、Dockerfile
- [ ] 待下一轮: DashboardPage 迁移、DeviceDetailPage 迁移、TasksPage 迁移
- [ ] 待环境: Go test、Docker Compose 集成验证

## 2026-07-08 Round 1 — 代码审计 + P0 修复

### 审查范围

四维度并行审查：
- **Server Go** (store.go, app.go, api.go, agent.go): 发现 30+ 问题
- **Agent Go** (main.go, main_test.go): 发现 20+ 问题
- **Web 前端** (全部 17 个 TS/TSX 文件): 发现 25+ 问题
- **Infra/配置** (Dockerfiles, compose.yaml, scripts, docs): 发现 20+ 问题

### 本轮修复清单

#### Server (Go)
- [x] **P0** store.go:165-167 — UpsertDevice ON CONFLICT UPDATE 遗漏 `cpu_usage`/`memory_usage`/`disk_usage` 三列（设备重注册时指标丢失）

#### Agent (Go)
- [x] **P0** main.go:127 — 心跳发送失败不再静默丢弃，触发 cancel() 立即重连
- [x] **P1** main.go:144-151 — 空命令/TaskID 校验，立即返回错误而不是静默丢弃
- [x] **P1** main.go:152-158 — 处理 server 的 `error` 消息类型，记录日志并触发重连
- [x] **P1** main.go:159-160 — 处理 `default` 未知消息类型，记录日志

#### Web 前端
- [x] **P0** main.tsx:7,12,29 — 新增 `ErrorBoundary` 组件包裹整个应用，防止渲染崩溃白屏
- [x] **P0** TasksPage.tsx:17-18 — 修复 `initialValues` 在 `groups` 异步加载前的 bug：使用 `form.setFieldsValue` 在 load() 完成时设置默认分组
- [x] **P0** DashboardPage.tsx:22-28 — `Promise.all` → `Promise.allSettled`，单个 API 失败不再级联阻断其他数据块
- [x] **P1** LoginPage.tsx:20 — 区分 401/5xx/网络错误，不再统一提示"用户名或密码不正确"
- [x] 新增 `hooks/useLoadable.ts` — 通用数据加载 hook（loading/error/reload + 可选自动刷新），为后续重构提供基础
- [x] 新增 `components/ErrorBoundary.tsx` — React Error Boundary

#### 基础设施
- [x] **P0** scripts/test.ps1 — 修复 `docker` 命令退出码未检测 bug（`$ErrorActionPreference = "Stop"` 不作用于原生 exe）。添加 `$LASTEXITCODE` 检查，并优先尝试本地 `go` 命令
- [x] **P0** LICENSE — 创建 MIT License 文件
- [x] **P1** .dockerignore — 创建，排除 `node_modules/`、`.git/`、`data/`、`*.db` 等
- [x] **P1** .env.example — 创建环境变量示例文件

### 验证结果

- [x] `npm run build` (tsc + vite build) — **通过**
- [ ] `go test ./...` for server — **待环境**（Go 本地未安装，Docker 镜像拉取阻塞）
- [ ] `go test ./...` for agent — **待环境**（同上）
- [ ] Docker Compose 集成测试 — **待环境**（同上）

### 已知未修复项（deferred）

| 优先级 | 模块 | 问题 | 原因 |
|--------|------|------|------|
| P0 | Server | 明文密码存储 | 需引入 bcrypt 依赖，作为独立 issue |
| P0 | Server | task_result 未验证设备所有权 | 需改协议层，影响较大 |
| P0 | Web | 全页面缺少 catch 块 | 需要大规模重构，本轮先提供 useLoadable hook 基础设施 |
| P1 | Web | DeviceDetailPage 加载全部任务 | 需后端新增 `/api/devices/{id}/tasks` 端点 |
| P1 | Web | auto-refresh 不一致（Groups/Audit） | 低优先级，待下一轮 |
| P1 | Server | 维护循环静默吞错误 | 需添加日志依赖 |
| P1 | Agent | 命令输出无大小限制 | 内存限制保护，待下一轮 |
| P2 | Agent | 重连无指数退避 | 待下一轮 |
