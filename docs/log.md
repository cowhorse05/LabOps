# LabOps 变更日志

## 2026-07-09 Round 12 — 协作者提交审查 + 主 agent 统筹

### 发现

后台 Docker 重建任务暴露构建失败：`go.mod requires go >= 1.25.0 (running go 1.23.12)`——但经排查，协作者已在 `cfbd94e` 修复。

### 协作者提交审查

**`f067fec` fix: 代码审计修复 batch 2 — MaxOpenConns 修复**
- [x] **实际改动**: 仅 `store.go` MaxOpenConns 条件化——`:memory:` 设 1，文件模式设 4（修复 8 个测试的"no such table"失败）
- [~] 其余 6 项标注早有代码，非新增变更

**`cfbd94e` fix: critical/high regressions from audit round 3** ⭐ (6 files, +52/-17)
- [x] `server/Dockerfile`: `golang:1.23-alpine` → `golang:1.25-alpine` (解除构建阻断)
- [x] `server/internal/core/app.go`: rate limiter `rlMu` 锁范围延伸至 `allow()`——修复数据竞争
- [x] `server/internal/core/analyzer_test.go`: `!= 86` → `!= 87`（断言值修正）
- [x] `server/internal/core/api.go`: `handleCreateTask` 响应统一为 `{tasks, errors?}`
- [x] `web/src/hooks/useLoadable.ts`: fetchers 存入 ref 保证 `useCallback` 稳定引用（修复 DashboardPage 无限 effect 循环）
- [x] `agent/cmd/agent/main.go`: 新增 read deadline pump（使 heartbeat cancel 能中断 ReadJSON）+ panic recovery 发送 failure result

### 清理

- [x] 删除损坏空目录 `server;D/`（文件系统残留，未被 git 追踪）

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (4.096s) |
| agent `go test ./...` | ✅ PASS (1.826s) |

### 自检

- **没想到**: 协作者在我这轮开始后 23 分钟又推了 `cfbd94e`——正好覆盖了我计划修复的 Dockerfile 和测试断言。我的子代理改动被抢先，变为空操作。协作模式有效运转
- **疏漏**: 初次 `git pull` 显示 "Already up to date"，但实际远端已有 `cfbd94e`——可能是 fetch/pull 之间恰好有推送，或本地缓存问题。后续应先用 `git fetch --all` 再 `git log`
- **改进**: 以后每轮开始先 `git fetch --all && git log --all --oneline -5` 确保看到所有远端分支

### 待处理（低优先级）

| 项 | 说明 |
|----|------|
| 硬编码 admin 密码 | `store.go` 种子密码 "admin"，无首次登录强制修改 |
| 静态 Token | 无过期/会话机制，共享同一 WebToken |
| CompleteTask TOCTOU | SELECT 和 UPDATE 之间有微小竞态窗口 |
| dispatchPendingTasks | 首个任务失败后跳过剩余任务，无部分失败收集 |
| 文档更新 | master-plan.md/user-manual.md/README.md 待同步新功能 |

## 2026-07-08 R7-R10 — 测试补全 + AI Ops + 使用手册 + 收尾

### R7: auth 中间件测试
- [x] api_test: +4 auth tests (NoToken/ValidToken/InvalidToken/SkipPaths)
- [x] api_test: 全部 19 测试添加 t.Parallel()

### R8: 边缘用例 + 无障碍
- [x] api_test: +5 tests (GetDevice/GetTask found+notFound, ByGroup, InvalidJSON)
- [x] AppLayout brand div→keyboard accessible
- [x] DevicesPage search aria-label

### Feature: AI Ops 智能分析
- [x] analyzer.go: 规则引擎 (offline/CPU/Mem/Disk/Task 8 维度评分)
- [x] 每 30min 自动分析, 设备健康分 0-100
- [x] Web AiOpsPage: 摘要+统计卡片+设备洞察+分组概览
- [x] API: GET /api/aiops/report

### R9: AI Ops 修复 + 文档
- [x] analyzer: Stop() + done channel 防 goroutine 泄漏
- [x] analyzer: ListTasks/Groups 错误日志
- [x] AiOpsPage: catch 块用户提示
- [x] analyzer_test: 8 test cases
- [x] docs/user-manual.md: 9 章节完整使用手册

### R10: 最终收尾
- [x] 文档更新, 项目指标汇总

### 项目最终状态

| 指标 | 值 |
|------|-----|
| Commits | 13 |
| Source files | 32 (Go + TSX/TS) |
| Tests | 5 files, 57+ cases |
| Docs | 9 markdown |
| API endpoints | 13 |
| Web pages | 8 |

## 2026-07-08 Round 6 — P2 收尾 + 测试验证

### 修复

**P2:**
- [x] server: maintenance loop 添加 5s context timeout (防止 DB 锁时 goroutine 永久阻塞)
- [x] server: CORS 添加 `Vary: Origin` 头 (防止缓存污染)
- [x] server: agent 注册失败时发送 error 消息再断连 (而非静默断开)

### 测试验证

- [x] `api_test.go` 质量审查：语法 ✅ 隔离 ✅ httptest ✅ JSON ✅ import ✅ 覆盖率部分通过 ⚠️ `t.Parallel()` ❌
- [x] 发现 7 个覆盖缺口：auth 中间件、GroupName 批量任务、GetDevice/GetTask 正常路径、无效 JSON body、CORS OPTIONS
- [x] go.sum 已补全（HTTP test agent 自动执行了 `go mod tidy`）

### 验证

- [x] `npm run build` — **通过**
- [x] git push — **通过** (37a4bb3, 4 files, +67)
- [ ] `go test ./...` — **待环境**

### 自检

- **没想到**: HTTP test agent 在后台运行时自动执行了 `go mod tidy`，补全了之前缺失的 `go.sum`——意外收获
- **疏漏**: 测试覆盖 7 个缺口未修复（最关键的：auth 中间件测试）
- **改进**: 下一轮补充 auth 中间件测试 + `t.Parallel()`

## 2026-07-08 Round 5 — P0 安全加固 + HTTP handler 测试

### 审查

定向审查发现 3 P0 + 3 P1 + 6 P2，焦点：
- 命令输出无限制 → agent OOM + DB 膨胀
- task_result 无设备归属校验 → 安全漏洞
- 批量任务部分失败无回滚 → 悬空任务
- WebSocket 错误静默丢弃 → 状态不一致

### 修复

**P0:**
- [x] agent: 命令输出大小限制 (stdout 256KB, stderr 64KB), io.LimitReader
- [x] server: task_result 设备归属校验 (GetTask + DeviceID 比对)
- [ ] ~~批量任务回滚~~ — deferred (需事务机制)

**P1:**
- [x] server: WebSocket 错误日志 (UpdateHeartbeat/CompleteTask/CreateAudit)
- [x] web: DeviceDetailPage/TasksPage onError 接入
- [x] web: TasksPage submit catch 块

**测试:**
- [x] api_test.go: 新增 15 个 HTTP handler 测试 (511 行)
  - handleHealth, handleLogin(valid/invalid), handleMe
  - handleStats, handleDevices(empty/withData), handleGetDevice(notFound)
  - handleGroups, handleCreateTask(4 cases), handleTasks, handleAudit

### 验证

- [x] `npm run build` — **通过**
- [x] `api_test.go` 语法检查 — **通过** (go vet)
- [x] git commit — **通过** (dbd1aed, 6 files, +589 -23)
- [ ] git push — 网络不可用 (不重试，1 pending commit)
- [ ] `go test ./...` — **待环境**

### 自检

- **没想到**: HTTP test agent 仍运行但文件已写入并被 R5 commit 包含——时间窗口恰好在 agent 写文件和 commit 之间
- **疏漏**: P0-2 (批量任务回滚) 未修复——需事务支持，复杂度超本轮范围
- **改进**: 下轮处理 P2 项 (maintenance timeout, Vary header, CORS Origin 白名单)

## 2026-07-08 Round 4 — 测试补充 + onError + 类型修复

### 变更

**Web:**
- [x] **Hook fix**: `useLoadable` 的 `onError` 之前只定义未调用——现已修复并在 load() 中调用
- [x] **onError 接入**: Dashboard/Devices/Groups/Audit 页面接入 onError Toast
- [x] **TypeScript**: 移除 `Device.status` 和 `Task.status` 的 `| string` 类型拓宽

**Server:**
- [x] **maintenance loop**: `ExpireDevices`/`TimeoutTasks` 错误改用 `log.Printf` 记录

**Tests:**
- [x] **store_test.go**: 新增 `TestStoreEdgeCases` 9 个 subtest (FindUser, UpsertDevice, Heartbeat, ExpireDevices, TimeoutTasks, ListDevicesByGroup, Groups, PendingTasks, ListTasks_Empty)
- [x] **agent_test.go**: 新增 19 个 subtest (sanitizeID×8, jitter×3, profileSpec×6, agentWSURL×2)

### 验证

- [x] `npm run build` (tsc + vite) — **通过**
- [x] `tsc --noEmit` — **通过** (移除 `| string` 无破坏)
- [x] git push — **通过** (385dd41)
- [ ] `go test ./...` — **待环境**

### 自检

- **没想到**: `onError` 在 Round 3 定义但从未调用——hook bug 直到本轮才被发现
- **疏漏**: `app.go`/`api.go`/`agent.go` 的 HTTP handler 测试仍为空
- **改进**: 后续用 `net/http/httptest` 添加 handler 层测试

## 2026-07-08 Round 3 — 完成 useLoadable 全页面重构

### 变更

- [x] **DashboardPage** — 5 源加载迁移到 `useLoadableAll` (173→151 行)
- [x] **TasksPage** — 双源加载迁移到 `useLoadable` + `useCallback` fetcher
- [x] **DeviceDetailPage** — device+tasks 加载迁移到 `useLoadable`
- [x] **useLoadable hook** — 新增 `onError?: (error: Error) => void` 回调

### 验证

- [x] `npm run build` — **通过**
- [x] git commit + push — **通过** (1f84692)

### 自检

- **没想到**: Linter 自动清理比手动 Edit 更彻底，TasksPage 残留旧代码被完全移除
- **疏漏**: `onError` 回调已添加但无页面实际接入——需要后续轮次接入 antd message
- **改进**: Web 7 个页面中 6 个已完成 useLoadable 迁移（仅 LoginPage 不需迁移），重构达成

### 迁移状态

| 页面 | 方式 | 自动刷新 | 状态 |
|------|------|---------|------|
| DashboardPage | useLoadableAll | 10s | ✅ R3 |
| DevicesPage | useLoadable | 10s | ✅ R2 |
| GroupsPage | useLoadable | 10s | ✅ R2 |
| AuditPage | useLoadable | 15s | ✅ R2 |
| TasksPage | useLoadable | 3s | ✅ R3 |
| DeviceDetailPage | useLoadable | 3s | ✅ R3 |
| LoginPage | N/A (mutation only) | - | ✅ R0 |

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
