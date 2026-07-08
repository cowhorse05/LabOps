# LabOps 变更日志

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

### 自检

**有什么没想到的？**
- 没考虑到 `useLoadableAll` hook 的 `fetchers` 数组类型推导在 TS 中的复杂度。当前实现使用 `Promise.all` + 每个单独 catch，但 TypeScript 类型推导可能不够精确。
- 没给 `ErrorBoundary` 加 `componentDidCatch` 的错误上报机制（如 Sentry）。

**有什么疏漏？**
- 审查报告指出 `newID` 的 fallback 路径存在 ID 碰撞风险，本轮未修复。需要在下一轮处理。
- Agent `executeAndReport` 中的 goroutine panic 无 recover，仍未处理。

**如何改进？**
- 下一轮应优先：
  1. 为 server 和 agent 补充核心路径单元测试
  2. 引入 `useLoadable` hook 重构所有页面，消除重复的 loading 模板
  3. 为 GroupsPage 和 AuditPage 添加自动刷新
