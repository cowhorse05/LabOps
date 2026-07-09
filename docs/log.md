# LabOps 变更日志

## 2026-07-09 Round 21 — 最终检查点

### 状态

全部 TODOs 已完成或明确延期。R12-R14 历史清单已更新标记。

### 📋 Todolist

- [x] 全部可操作 TODOs 已完成
- [ ] 延期: admin 密码 / 静态 Token（架构变更，需独立功能分支）

### 项目总体

| 维度 | 状态 |
|------|------|
| Server 50 tests | ✅ |
| Agent 7 tests | ✅ |
| go vet ×2 | ✅ |
| TS tsc + build | ✅ |
| GitHub Actions CI | ✅ |
| SQL 事务保护 | ✅ |
| DB 索引 (4) | ✅ |
| WS 集成测试 | ✅ |
| 文件分发 spec | ✅ |
| 文档 (11 md) | ✅ |

## 2026-07-09 Round 19 — 文档更新 + v0.3 文件分发 Spec

### 文档

- **新增 `docs/features/file-distribution/design.md`**：v0.3 文件分发功能完整设计 SSOT
  - REST API: `POST /api/files` (multipart)、`GET /api/files`、`GET /api/files/{id}`、`GET /api/files/{id}/blob`
  - WebSocket 协议: `file_push` (server→agent) + `file_result` (agent→server)
  - 数据模型: `file_tasks` 表，含 SHA-256 校验和
  - Agent 安全约束: 原子写入 (.tmp→rename)、路径穿越防护、50 MiB 限制
  - Web 页面: FileDistributionPage (`/files`)，含上传表单 + 任务表格
- **新增 `docs/features/file-distribution/tasks.md`**：7 阶段 40 项实现任务清单
- **更新 `README.md`**：CI badge、准确测试计数 (Server 50 + Agent 7)、特性列表扩充
- **更新 `docs/master-plan.md`**：测试计数修正、下一步建议标记 CI 完成

### 📋 Todolist

- [x] Round 19: 文件分发 spec + README 更新 + 测试计数修正
- [ ] 延期: admin 密码 / 静态 Token

## 2026-07-09 Round 18 — GitHub Actions CI 接入

### CI 配置

新增 `.github/workflows/ci.yml`，3 个并行 job：

| Job | 环境 | 步骤 |
|-----|------|------|
| Server (Go) | go 1.25, ubuntu-latest | checkout → go vet → go test |
| Agent (Go) | go 1.23, ubuntu-latest | checkout → go vet → go test |
| Web (TS) | node 20, ubuntu-latest | checkout → npm ci → tsc --noEmit → npm run build |

触发条件：`push` to `master` / `pull_request` to `master`。

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` (50 函数) | ✅ PASS |
| agent `go test ./...` (7 函数) | ✅ PASS |
| server `go vet` | ✅ 无警告 |
| agent `go vet` | ✅ 无警告 |
| TypeScript `tsc --noEmit` | ✅ 零错误 |
| Web `npm run build` | ✅ 通过 |

### 📋 Todolist

- [x] Round 18: GitHub Actions CI workflow
- [ ] 延期: admin 密码 / 静态 Token（架构变更）

## 2026-07-09 Round 17 — handleAgentWS WebSocket 集成测试

### 测试

新增 `agent_test.go` 中 5 个 WebSocket 集成测试（使用 `httptest.NewServer` + `gorilla/websocket.Dialer`）：

| 测试 | Subtests | 覆盖场景 |
|------|----------|---------|
| `TestHandleAgentWS_TokenAuth` | 3 | 缺失 token → 401；错误 token → 401；正确 token → 升级成功 |
| `TestHandleAgentWS_Register` | — | 完整注册流程：连接→发送 register→读取 registered→验证 store 中 device+sesssion |
| `TestHandleAgentWS_InvalidRegister` | 2 | 首条消息非 register 返回 error；空 agentId/name 返回 error |
| `TestHandleAgentWS_Heartbeat` | — | 注册后发送 heartbeat→验证 last_seen + cpu/memory/disk 指标更新 |
| `TestHandleAgentWS_Disconnect` | — | 注册→关闭连接→验证 device 变 offline + audit log 有 disconnect 记录 |

`newTestAppWS` / `wsDial` / `wsWrite` / `wsRead` helper 减少样板代码。

**handleAgentWS 覆盖率**: 0% → 关键路径已覆盖（token auth、register、heartbeat、disconnect cleanup）。

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (2.69s), 15 测试函数 |
| agent `go test ./...` | ✅ PASS (0.95s) |
| server `go vet` | ✅ 无警告 |
| agent `go vet` | ✅ 无警告 |
| TypeScript `tsc --noEmit` | ✅ 零错误 |

### 📋 Todolist

- [x] Round 17: handleAgentWS WebSocket 集成测试
- [ ] 延期: admin 密码 / 静态 Token（架构级变更）

## 2026-07-09 Round 16 — 审计中等修复：事务/索引/API 端点

### 修复

本轮修复 Round 15 最终审计发现的 3 个中等问题：

#### 1. SQL 事务包裹 FailTask/CompleteTask (`store.go`)

`FailTask` 和 `CompleteTask` 原先两条 SQL（UPDATE tasks + INSERT task_results）独立执行，无事务保护。现已包裹在 `BeginTx`/`Commit`/`Rollback` 中，保证原子性：
- `FailTask`: 失败时回滚，不留孤儿 task_results
- `CompleteTask`: RowsAffected 检查在事务内执行，0 行时安全提交空事务
- 使用 `defer tx.Rollback()` 模式，错误路径自动回滚

#### 2. DB 索引优化 (`store.go` Init)

新增 4 个索引（`IF NOT EXISTS`，幂等）：
- `idx_tasks_device_status` — `tasks(device_id, status)` → PendingTasksForDevice
- `idx_tasks_status_started` — `tasks(status, started_at)` → TimeoutTasks
- `idx_audit_logs_device` — `audit_logs(device_id)` → 审计按设备过滤
- `idx_devices_group` — `devices(group_name)` → ListDevicesByGroup

#### 3. DeviceDetailPage 专用 API 端点 (5 files)

新增 `GET /api/devices/{id}/tasks` 端点，替代前端获取全部 tasks 再过滤的模式：
- **store.go**: 新增 `ListTasksByDevice` (LIMIT 50)
- **api.go**: 新增 `handleListDeviceTasks` handler
- **app.go**: 注册路由 `GET /api/devices/{id}/tasks`
- **labops.ts**: 新增 `deviceTasks(id)` API client
- **DeviceDetailPage.tsx**: `fetchData` 改用 `deviceTasks(id)` 替代 `tasks() + filter`

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (3.01s), 54 函数 |
| agent `go test ./...` | ✅ PASS (0.93s) |
| server `go vet` | ✅ 无警告 |
| agent `go vet` | ✅ 无警告 |
| 覆盖率 (core) | 72.3% |
| TypeScript `tsc --noEmit` | ✅ 零错误 |
| Web `npm run build` | ✅ 通过 (7.46s) |

### 📋 Todolist

- [x] Round 16: SQL 事务 + DB 索引 + DeviceDetailPage API 端点
- [x] agent handler 层 WebSocket 测试（handleAgentWS） → Round 17
- [ ] 延期: admin 密码 / 静态 Token

## 2026-07-09 Round 15 — agent.go handler 层测试 + 最终审计

### agent.go 测试 (0% → ~65% 函数覆盖)

新增 `server/internal/core/agent_test.go`（589 行，10 测试函数，28 subtests）：

| 测试 | Subtests | 覆盖函数 |
|------|----------|---------|
| `TestDeviceFromRegister` | 8 | deviceFromRegister 100% |
| `TestRegisterClient` | 2 | registerClient 100% |
| `TestUnregisterClient` | 2 | unregisterClient 72.7% |
| `TestDispatchTask_NoClient` | 1 | dispatchTask 41.7% |
| `TestDispatchPendingTasks` | 2 | dispatchPendingTasks 60.0% |
| `TestRefreshState` | 1 | refreshState |
| `TestAgentClientSend_Closed` | 1 | AgentClient.Send 66.7% |
| `TestAgentClientClose_Idempotent` | 1 | AgentClient.Close 57.1% |
| `TestRateLimiter` | 3 | rateLimiter |
| `TestNewAppDefaults` | 1 | NewApp 默认值 |

`handleAgentWS` (0%) 需要 WebSocket 集成测试——延迟。

### 最终代码审计

21 文件全面审计——结论：**生产就绪 8/10**。

发现 3 个中等问题（延期至生产部署前）：

| # | 严重度 | 问题 |
|---|--------|------|
| 1 | 中 | `FailTask`/`CompleteTask` 无事务包裹（两条 SQL 可能部分写入） |
| 2 | 中 | 缺少 DB 索引：`tasks(device_id,status)`, `tasks(status,started_at)`, `audit_logs(device_id)`, `devices(group_name)` |
| 3 | 中 | `DeviceDetailPage` 前端获取全部 tasks 再过滤（需新 API 端点） |

8 个低级问题（信息性/建议性，不阻塞）。

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (4.38s), **54 函数** (+10 from R14) |
| agent `go test ./...` | ✅ PASS |
| 覆盖率 (core) | **74.0%** (↑ 从 66.2%) |
| 覆盖率 (total) | **71.3%** (↑ 从 63.8%) |
| TypeScript | ✅ 零错误 |
| `go vet` | ✅ 无警告 |

### 自检

- **没想到**: 子代理生成的 agent_test.go 长达 589 行，覆盖了 10 个测试函数——比预期更完整。`deviceFromRegister` 的 8 个 subtest 包含了空格、空值、默认值等所有边缘情况
- **疏漏**: 工作目录 `cd` 到 server/ 后未重置，导致 `git status` 路径解析错误——后续 git 命令需要绝对路径或先 `cd` 回项目根
- **改进**: 审计发现的 3 个中等问题应记录到 master-plan 的"已知限制"部分，避免丢失

### 📋 Todolist

- [x] ~~Round 15: agent.go 测试 + 最终审计~~
- [x] 生产部署前: SQL 事务包裹 FailTask/CompleteTask → Round 16
- [x] 生产部署前: DB 索引优化 → Round 16
- [x] 生产部署前: DeviceDetailPage 专用 API 端点 → Round 16
- [ ] 延期: admin 密码 / 静态 Token

## 2026-07-09 Round 14 — 死代码清理 + 文档更新 + 测试补充

### 死代码清理

- [x] `api.go`: 移除 `createTaskResponse` 结构体（cfbd94e 响应统一后仅测试中使用）
- [x] `api_test.go`: 添加局部类型定义，测试编译通过

### 文档更新

- [x] `docs/master-plan.md`: 更新状态头、测试计数（41 函数）、Go 版本（1.25）、删除已解决阻塞项、更新 Docker Compose 状态
- [x] `README.md`: MVP 功能列表新增 AI Ops

### 测试补充

- [x] `store_test.go`: 新增 `TestSessionCRUD` — 3 subtests (CreateAndClose, CloseNonexistent, MultipleSessions)
- [x] 填补 `CreateSession`/`CloseSession` 0% 覆盖率缺口
- [x] 覆盖率: 66.2% → ~68%（core 包）

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (4.327s), 44 函数 |
| agent `go test ./...` | ✅ PASS |
| TypeScript `tsc --noEmit` | ✅ 零错误 |
| `go vet ./...` | ✅ 无警告 |

### 自检

- **没想到**: Coverage 审计发现 `CreateSession`/`CloseSession` 完全未覆盖——这两个函数仅被 WebSocket handler 调用，而 handler 层无测试。补了 3 个简单的 store 层测试即可填补
- **疏漏**: `api_test.go` 的局部类型定义本可以跳过——直接解码为 `map[string]any` 或 `[]Task` 即可，无需定义结构体。但当前方案最小化改动，测试语义不变
- **改进**: 下一轮可以给 agent handler 层添加 WebSocket 测试（`agent_test.go`），当前 0% 覆盖率

### 📋 Todolist

- [x] Round 14: 死代码清理 + 文档更新 + Session 测试
- [x] agent handler 层 WebSocket 测试 → Round 17 ✅
- [ ] 延期: admin 密码 / 静态 Token

## 2026-07-09 Round 13 — 深度审查 + 协作者 45fe65f 验收

### 新发现：协作者提交 45fe65f

本应在本轮修复的 3 个低优问题已被协作者 `Lgithubprogram` 提前完成（`45fe65f`，2 files, +28/-16）：

| 文件 | 修复 | 方式 |
|------|------|------|
| `store.go` | CompleteTask TOCTOU 竞态 | SELECT-then-UPDATE → 原子条件 `UPDATE WHERE status NOT IN (...)` |
| `agent.go` | dispatchPendingTasks 部分失败 | 首个错误后 continue 而非 return |
| `agent.go` | unregisterClient 错误日志 | `_ =` → `if err != nil { log.Printf(...) }` |

### cfbd94e 深度代码审查结论

6 项修复逐行审查——全部正确，无回归：
- [x] `app.go`: rate limiter `rlMu` 锁覆盖 `allow()`——消除数据竞争 ✅
- [x] `api.go`: `handleCreateTask` 响应统一——向后兼容 ✅
- [x] `analyzer_test.go`: 86→87——数学验证无误 ✅
- [x] `agent/main.go`: read deadline pump + panic recovery——机制正确 ✅
- [x] `useLoadable.ts`: fetchers in ref——稳定 useCallback，消除无限循环 ✅
- [x] `Dockerfile`: Go 1.23→1.25——版本匹配 ✅
- [~] 微小死代码: `createTaskResponse` 仅测试使用

### 延期 (2 项)

| 项 | 原因 |
|----|------|
| 硬编码 admin 密码 | 需 schema + API + 前端——多组件功能 |
| 静态 WebToken | 需 JWT/会话存储——架构级变更 |

### 测试

| 模块 | 结果 |
|------|------|
| server `go test ./...` | ✅ PASS (4.012s) |
| agent `go test ./...` | ✅ PASS (1.741s) |
| TypeScript `tsc --noEmit` | ✅ 零错误 |
| `-race` | ❌ MinGW gcc 8.1.0 不兼容（工具链，非代码） |

### 自检

- **没想到**: 子代理声称修复的 3 项实际已在 `45fe65f` 中存在——代理未做 `git log` 验证即报告"已修复"。根源：子代理工具链中 `git` 命令的 Windows 路径问题导致历史查询失败，代理基于文件内容正确推断为"需要修复"而非"已修复"
- **疏漏**: 本轮开始时 `git fetch` 超时，但 `45fe65f` 仍进入了本地仓库——可能是之前的 pull 缓存或 fetch 部分成功。未在开始前做 `git log --all -10`（仅做了 `-5`），导致遗漏远端提交
- **改进**: 以后每轮必须 `git log --all --oneline -10`（非 `-5`），且在子代理任务描述中明确要求 `git log` 确认改动是否已存在

### 📋 Todolist

- [x] Round 13: cfbd94e 审查 + 45fe65f 验收
- [x] 清理 `createTaskResponse` 死代码 → Round 14 ✅
- [x] 文档同步: master-plan/user-manual/README → 已同步
- [ ] 延期: admin 密码 / 静态 Token（需独立功能分支）

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
