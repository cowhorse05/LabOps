# LabOps Architecture & Internal Logic

This document describes the system architecture, data flow, database schema, authentication mechanisms, and core business logic of LabOps. It is intended for developers who want to understand the internals or contribute to the project.

---

## System Architecture

LabOps follows a three-tier architecture orchestrated by Docker Compose:

```
                        ┌──────────────────────────────────────┐
                        │          Docker Host                  │
                        │                                       │
  Browser ──HTTPS────▶  │  ┌──────────────────────────────┐    │
  (User)                │  │  web (Nginx :80/:443)         │    │
                        │  │  - Static file serving         │    │
                        │  │  - TLS termination (Let's Encrypt)│  │
                        │  │  - /api/* → proxy to server    │    │
                        │  │  - WebSocket upgrade support   │    │
                        │  └──────────┬───────────────────┘    │
                        │             │ internal (backend net) │
                        │  ┌──────────▼───────────────────┐    │
                        │  │  server (Go :8080)            │    │
                        │  │  - REST API (34 endpoints)    │    │
                        │  │  - WebSocket hub (agents)     │    │
                        │  │  - AI Ops analyzer            │    │
                        │  │  - Maintenance loop           │    │
                        │  └──────┬───────────┬───────────┘    │
                        │         │           │                │
                        │  ┌──────▼──────┐    │ (egress net)   │
                        │  │ mysql :3306 │    │ outbound only  │
                        │  │ (internal)  │    │                │
                        │  └─────────────┘    └──▶ LLM APIs   │
                        └──────────────────────────────────────┘
                                     ▲
  Agent Machines ──WSS─────┘ (direct WebSocket to server)
```

### Docker Network Topology

| Network | Type | Connected Services | Purpose |
|---------|------|--------------------|---------|
| `backend` | Internal bridge | mysql, server, web | Private service-to-service communication. MySQL and server ports are NOT exposed to the host. |
| `egress` | Internal bridge | server | Allows the server to reach external LLM APIs without exposing MySQL or internal services. |
| `edge` | Internal bridge | web | Allows Nginx to accept inbound connections on ports 80/443. |

### Service Versions & Images

| Service | Image | Purpose |
|---------|-------|---------|
| mysql | `mysql:8.0` (official) | Relational database. Health check via `mysqladmin ping`. |
| server | Built from `server/Dockerfile` | Go 1.25 API server. Multi-stage build: `golang:1.25-alpine` → `alpine:3.20`. Runs as non-root `labops` user. |
| web | Built from `web/Dockerfile` | Node 22 build → Nginx 1.27-alpine serve. Reverse proxy with TLS, SPA routing. |

---

## Data Flow

The core of LabOps is the **Agent → Server → Web Console** control loop:

```
                         Register (device profile)
  Agent ──────────────────▶  Server
                         ◀── registered (device ID assigned)

              Heartbeat every 10s (CPU/mem/disk %)
  Agent ──────────────────▶  Server  ──▶ UpsertDevice + UpdateHeartbeat
                                          Server ──▶ Web Console (REST polling)

              User creates task via Web Console
  Web Console ──POST /api/tasks──▶  Server  ──▶ CreateAudit ("command.create")

              Server dispatches to connected agent
  Server ──command──▶  Agent     ◀── Agent executes via os/exec

              Agent returns result
  Agent ──task_result──▶  Server  ──▶ CompleteTask + CreateAudit ("command.complete")

              Web Console polls for results
  Web Console ──GET /api/tasks──▶  Server  ──▶ Task with stdout/stderr/exit code
```

---

## WebSocket Protocol

Agent-server communication uses a persistent WebSocket connection with JSON envelope messages.

**Envelope Format:**
```json
{"type": "<message_type>", "payload": { ... }}
```

**Message Types:**

| Direction | Type | Payload | Frequency | Description |
|-----------|------|---------|-----------|-------------|
| Agent → Server | `register` | `RegisterPayload` | On connect | Device identity, hostname, OS, CPU cores, memory, disk |
| Agent → Server | `heartbeat` | `HeartbeatPayload` | Every 10s | CPU usage %, memory usage %, disk usage % |
| Agent → Server | `task_result` | `TaskResultPayload` | On completion | stdout, stderr, exit code, duration in ms |
| Server → Agent | `registered` | (none) | After register | Confirms registration complete |
| Server → Agent | `command` | `CommandPayload` | On task dispatch | Task ID, command string or executable+args, timeout |
| Server → Agent | `error` | `{message}` | On failure | Error notification, agent will reconnect |

**Payload Structures (from `server/internal/core/types.go`):**

- **RegisterPayload:** `agentId`, `name`, `groupName`, `version`, `profile`, `hostname`, `os`, `ip`, `cpuCores`, `memoryMb`, `diskTotalGb`
- **HeartbeatPayload:** `cpuUsage` (float64), `memoryUsage` (float64), `diskUsage` (float64)
- **TaskResultPayload:** `taskId`, `status` ("success"/"failed"), `stdout`, `stderr`, `exitCode`, `durationMs`
- **CommandPayload:** `protocolVersion` (int), `taskId`, `kind` ("ad_hoc"/"template"), `command` (shell string), `executable` (absolute path), `args` ([]string), `timeoutSeconds`

---

## Database Schema

LabOps supports **SQLite** and **MySQL 8.0+** via a dialect abstraction layer (`server/internal/core/dialect.go`). The default driver is MySQL (`LABOPS_DB_DRIVER=mysql`). Set `LABOPS_DB_DRIVER=sqlite` for development.

The schema is defined once in Go structs using logical column types (`CT_String32`, `CT_Text`, etc.) and translated to dialect-specific physical DDL. Types shown below are for **MySQL** (the production dialect). **All timestamp columns use VARCHAR(32)** and store RFC3339-formatted strings (e.g. `"2026-01-15T08:30:00Z"`).

### Table: `users`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `username` | VARCHAR(128) UNIQUE | Login name |
| `display_name` | VARCHAR(256) | Display name |
| `password` | VARCHAR(256) | bcrypt hash (cost 12) |
| `roles` | VARCHAR(512) | JSON array of roles: ["admin"], ["operator"], ["viewer"] |
| `must_change_password` | TINYINT (def: 0) | 1 = forced password change on next login |
| `status` | VARCHAR(32) (def: "active") | "active" or "disabled" |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `updated_at` | VARCHAR(32) | UTC, RFC3339 string |

### Table: `devices`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(128) PK | Agent ID |
| `name` | VARCHAR(256) | Human-readable name |
| `group_name` | VARCHAR(128) | Logical group |
| `profile` | VARCHAR(64) | OS profile or "real" |
| `version` | VARCHAR(32) | Agent version |
| `hostname` | VARCHAR(256) | OS hostname |
| `os` | VARCHAR(64) | OS name + version |
| `ip` | VARCHAR(64) | Reported IP address |
| `cpu_cores` | INTEGER | Number of CPU cores |
| `memory_mb` | INTEGER | Total memory in MB |
| `disk_total_gb` | INTEGER | Total disk in GB |
| `cpu_usage` | DOUBLE | Latest CPU % |
| `memory_usage` | DOUBLE | Latest memory % |
| `disk_usage` | DOUBLE | Latest disk % |
| `status` | VARCHAR(32) (def: "offline") | "online" / "offline" |
| `last_seen` | VARCHAR(32) | Last heartbeat, RFC3339 string |
| `credential_status` | VARCHAR(32) (def: "pending_reenrollment") | "pending_reenrollment" / "active" / "revoked" |
| `revoked_at` | VARCHAR(32) (nullable) | When credential was revoked |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `updated_at` | VARCHAR(32) | UTC, RFC3339 string |

### Table: `agent_sessions`

| Column | Type | Description |
|--------|------|-------------|
| `id` | BIGINT AUTO PK | Auto-increment ID |
| `device_id` | VARCHAR(64) FK | References devices.id |
| `remote_addr` | VARCHAR(128) | Agent IP |
| `connected_at` | VARCHAR(32) | Connection time, RFC3339 string |
| `disconnected_at` | VARCHAR(32) (nullable) | Disconnection time, RFC3339 string |

### Table: `tasks`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `device_id` | VARCHAR(64) (nullable) | Target device (null for group tasks) |
| `group_name` | VARCHAR(128) (nullable) | Target group |
| `command` | TEXT (nullable) | Shell command string |
| `kind` | VARCHAR(32) | "ad_hoc" or "template" |
| `template_id` | VARCHAR(64) (nullable) | Template reference |
| `executable` | VARCHAR(512) (nullable) | Absolute path for template tasks |
| `args_json` | TEXT (nullable) | JSON array of arguments |
| `timeout_seconds` | INTEGER | Max execution time |
| `status` | VARCHAR(32) | "pending" → "running" → "success"/"failed"/"timeout" |
| `requested_by` | VARCHAR(128) | Username who created the task |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `started_at` | VARCHAR(32) (nullable) | When execution began |
| `finished_at` | VARCHAR(32) (nullable) | When result was received |

### Table: `task_results`

| Column | Type | Description |
|--------|------|-------------|
| `task_id` | VARCHAR(64) PK FK | References tasks.id |
| `stdout` | MEDIUMTEXT | Standard output (truncated at 256KB) |
| `stderr` | MEDIUMTEXT | Standard error (truncated at 256KB) |
| `exit_code` | INTEGER | Process exit code |
| `duration_ms` | BIGINT | Execution duration in milliseconds |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |

### Table: `audit_logs`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `actor` | VARCHAR(128) | Username |
| `actor_id` | VARCHAR(64) (nullable) | User ID |
| `actor_role` | VARCHAR(64) (nullable) | User role |
| `remote_addr` | VARCHAR(128) (nullable) | Client IP |
| `request_id` | VARCHAR(64) (nullable) | X-Request-ID header |
| `action` | VARCHAR(128) | Action type: "command.create", "command.complete", "device.enroll", etc. |
| `device_id` | VARCHAR(64) (nullable) | Related device |
| `task_id` | VARCHAR(64) (nullable) | Related task |
| `status` | VARCHAR(32) | "success" / "failed" |
| `message` | TEXT (nullable) | Human-readable description |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |

### Table: `web_sessions`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `user_id` | VARCHAR(64) FK | References users.id |
| `token_hash` | VARCHAR(64) UNIQUE | SHA-256 of session token |
| `csrf_hash` | VARCHAR(64) | SHA-256 of CSRF token |
| `remote_addr` | VARCHAR(128) (nullable) | Browser IP |
| `user_agent` | VARCHAR(512) (nullable) | Browser User-Agent |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `last_seen_at` | VARCHAR(32) | Last activity, RFC3339 string |
| `idle_expires_at` | VARCHAR(32) | 8 hours from last activity |
| `absolute_expires_at` | VARCHAR(32) | 24 hours from creation |

### Table: `enrollment_codes`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `code_hash` | VARCHAR(64) UNIQUE | SHA-256 of enrollment code |
| `expires_at` | VARCHAR(32) | Max 1 hour from creation |
| `max_uses` | INTEGER (def: 1) | Maximum uses (1-20) |
| `used_count` | INTEGER (def: 0) | Current usage count |
| `created_by` | VARCHAR(128) | Admin username |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `revoked_at` | VARCHAR(32) (nullable) | When revoked |

### Table: `device_credentials`

| Column | Type | Description |
|--------|------|-------------|
| `device_id` | VARCHAR(64) PK FK | References devices.id |
| `secret_hash` | VARCHAR(64) | SHA-256 of 256-bit secret |
| `status` | VARCHAR(32) (def: "active") | "active" / "revoked" |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `last_used_at` | VARCHAR(32) (nullable) | Last WebSocket auth |
| `revoked_at` | VARCHAR(32) (nullable) | When revoked |

### Table: `command_templates`

| Column | Type | Description |
|--------|------|-------------|
| `id` | VARCHAR(64) PK | UUID |
| `name` | VARCHAR(256) | Template name |
| `description` | TEXT (nullable) | Human-readable description |
| `os` | VARCHAR(64) | Target OS: "linux", "windows", "any" |
| `executable` | VARCHAR(512) | Absolute executable path |
| `args_json` | TEXT (nullable) | JSON array of argument strings with `{{param}}` placeholders |
| `parameters_json` | TEXT (nullable) | JSON array of `TemplateParameter` definitions |
| `requires_privilege` | TINYINT (def: 0) | 1 = requires elevated privileges |
| `enabled` | TINYINT (def: 1) | 1 = active |
| `timeout_seconds` | INTEGER | Default execution timeout |
| `created_at` | VARCHAR(32) | UTC, RFC3339 string |
| `updated_at` | VARCHAR(32) | UTC, RFC3339 string |

### Table: `llm_config`

A single-row table (id=1) for AI Ops LLM configuration.

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK (def: 1) | Always 1 |
| `provider_url` | VARCHAR(1024) (nullable) | LLM API base URL |
| `api_key` | VARCHAR(512) (nullable) | Encrypted with AES-256-GCM |
| `model` | VARCHAR(128) (nullable) | Model name (e.g. "gpt-4o", "claude-sonnet-4-6") |
| `provider_type` | VARCHAR(32) (def: "openai") | "openai" or "anthropic" |
| `enabled` | TINYINT (def: 0) | 1 = LLM analysis active |
| `auto_execute_read_only` | TINYINT (def: 0) | 1 = auto-dispatch readonly recommendations |
| `updated_at` | VARCHAR(32) (nullable) | UTC, RFC3339 string |

### Table: `schema_migrations`

| Column | Type | Description |
|--------|------|-------------|
| `version` | INTEGER PK | Migration version number |
| `name` | VARCHAR(256) | Migration name |
| `applied_at` | VARCHAR(32) | When applied, RFC3339 string |

### Built-in Seed Data

5 command templates are seeded on first startup (all Linux, all in Chinese):
1. **主机名** (hostname) — `hostname`
2. **系统运行时间** (uptime) — `uptime`
3. **磁盘使用情况** (disk) — `df -h`
4. **内存使用情况** (memory) — `free -h`
5. **进程摘要** (processes) — `ps aux --sort=-%mem | head -20`

---

## Authentication & Authorization

### Web Authentication (Session Cookie)

LabOps uses **session cookies**, not JWT:

1. **Login** (`POST /api/auth/login`):
   - Validates username + bcrypt(password) against `users` table
   - Generates two 256-bit random tokens: session token + CSRF token
   - Stores SHA-256 hashes of both in `web_sessions`
   - Returns HTTP-only secure cookies:
     - `labops_session` — HttpOnly, Secure, SameSite=Strict, path=/
     - `labops_csrf` — readable by JavaScript, path=/

2. **Session Validation** (per-request in `withAuth` middleware):
   - Extracts `labops_session` cookie
   - Looks up session by token hash
   - Checks: not expired (idle 8h, absolute 24h), user not disabled
   - Checks `must_change_password` flag — if set, blocks all endpoints except `/auth/change-password`, `/auth/me`, `/auth/logout`
   - Updates `last_seen_at` on each request

3. **CSRF Protection** (double-submit cookie pattern):
   - All state-changing methods (POST/PUT/PATCH/DELETE) require `X-CSRF-Token` header
   - Header value must match `labops_csrf` cookie value
   - Hash of header value must match `csrf_hash` stored in `web_sessions`
   - Exempt: `/api/auth/login`, `/api/agent/enroll`

4. **Session Termination:**
   - Logout clears cookies
   - Password change deletes all sessions for that user, creates new session
   - Maintenance loop prunes expired sessions every 10 seconds

### Agent Authentication (Per-Device Credentials)

1. **Enrollment Flow:**
   - Admin creates enrollment code with TTL (max 1 hour) and max uses (1-20)
   - Code is hashed (SHA-256) and stored in `enrollment_codes`
   - Agent calls `POST /api/agent/enroll` with code + device profile
   - Server validates: hash match, not expired, not revoked, usage < max (optimistic concurrency via transaction)
   - Server generates 256-bit random device secret, stores hash in `device_credentials`
   - Agent saves `deviceId` + `deviceSecret` to local credentials file:
     - Linux: `/etc/labops-agent/credentials.json`
     - Windows: `%ProgramData%\LabOps\credentials.json`

2. **WebSocket Authentication:**
   - Agent connects to `GET /api/agent/ws` with header `Authorization: Agent <deviceId>:<secret>`
   - Server validates via constant-time comparison of SHA-256(secret) against stored hash
   - Legacy fallback: `X-Agent-Token` header (shared token, deprecated)

### Role-Based Access Control (RBAC)

| Role | Permissions |
|------|-------------|
| **admin** | All 8: `system:read`, `system:users`, `system:enrollment`, `system:device-revoke`, `templates:manage`, `templates:execute`, `commands:adhoc`, `aiops:llm` |
| **operator** | `system:read`, `templates:execute` |
| **viewer** | `system:read` |

Permission checks are enforced server-side in the `withAuth` middleware via `requiredPermission()`.

---

## Task Execution Lifecycle

The complete lifecycle of a task from creation to result:

```
Step 1: Web Console → POST /api/tasks
   Body: {deviceId?, groupName?, command, kind, confirmation?}
   - Ad-hoc tasks require confirmation: "EXECUTE" (safety guard)
   - Template tasks render {{parameters}} from param definitions with validation
   - Permissions: ad-hoc requires commands:adhoc, template requires templates:execute

Step 2: Server creates task record
   - Status = "pending"
   - Generates UUID task ID
   - Writes audit log: action="command.create"

Step 3: Server dispatches to agent
   - For single device: looks up AgentClient in app.clients map
   - For group: creates individual tasks for each online device in the group
   - If agent connected: sends "command" envelope via WebSocket
     - Task status → "running"
     - Audit log: action="command.dispatch"
   - If agent NOT connected: writes audit log action="command.queue"
     - Task stays "pending", will be dispatched on reconnect via dispatchPendingTasks()

Step 4: Agent receives command
   - Validates: taskId non-empty, kind matches payload format
   - Spawns goroutine for executeAndReport()

Step 5: Agent executes command
   - ad_hoc: cmd /C (Windows) or /bin/sh -c (Unix)
   - template: exec.Command(executable, args...) (executable must be absolute path)
   - Context with timeout (max 300s)
   - Captures stdout, stderr (truncated at 256KB each)
   - Exit code from process
   - Panic recovery → reports failure back to server

Step 6: Agent → Server task_result
   - Envelope type="task_result"
   - Payload: taskId, status, stdout, stderr, exitCode, durationMs

Step 7: Server processes result
   - Calls CompleteTask(): stores TaskResult in task_results table
   - Task status → "success" (exit 0) or "failed" (exit ≠ 0)
   - Writes audit log: action="command.complete"

Step 8: Web Console displays result
   - Frontend polls GET /api/tasks (auto-refresh 3s on TasksPage)
   - Expandable stdout/stderr in the tasks table
```

**Timeout Handling:**
- Server-side `maintenanceLoop()` (every 10s): marks tasks as "timeout" if `LABOPS_TASK_TIMEOUT` exceeded (default 5 min prod, 2 min dev)
- Agent-side: `context.WithTimeout` cancels execution at `timeoutSeconds`
- Agent sends result with exit code 124 and stderr message on timeout

**Reconnection & Pending Tasks:**
- On agent reconnect, server calls `dispatchPendingTasks(deviceId)` — sends all pending and running tasks
- Running tasks are re-dispatched because the agent may have lost the previous connection mid-execution

---

## AI Ops Analysis Pipeline

### Rule-Based Engine (`analyzer.go`)

Runs every 30 minutes (and at startup + on LLM config change). Calculates health scores (0-100) per device:

| Condition | Penalty |
|-----------|---------|
| Device offline | -40 |
| CPU usage > 80% | -20 |
| CPU usage > 60% | -10 |
| Memory usage > 80% | -20 |
| Memory usage > 60% | -10 |
| Disk usage > 85% | -15 |
| Task failure rate > 50% | -20 |
| Task failure rate > 20% | -10 |

Each device starts at 100. Penalties are cumulative (a device can score below 0, clamped to 0). A device with all thresholds exceeded would score: 100 - 40 - 20 - 10 - 20 - 10 - 15 - 20 - 10 = -45 → 0.

### LLM Enhancement (`llm.go`)

If an LLM is configured (via env vars or Settings UI):

1. **Provider Support:**
   - OpenAI-compatible: `POST {providerUrl}/v1/chat/completions`
   - Anthropic: `POST {providerUrl}/v1/messages`

2. **Prompt** (Chinese): Asks the LLM to act as a "laboratory equipment operations expert" and analyze device metrics
   - Sends: all device data (CPU/memory/disk usage, status, task statistics)
   - Requests: structured JSON with `textAnalysis` (Chinese report) and `recommendations` array

3. **Validation:**
   - Cross-references `deviceId` against actual connected devices
   - Runs `isDangerousCommand()` check — blocks destructive commands (rm -rf, mkfs, dd, shutdown, reboot, etc.)
   - Filters to only valid, safe recommendations

4. **Recommendation Format:**
   ```json
   {
     "deviceId": "agent-xxx",
     "deviceName": "lab-pc-01",
     "command": "systemctl restart nginx",
     "reason": "Nginx memory usage exceeding threshold",
     "priority": "high",
     "isMutation": true
   }
   ```

### Auto-Execute Mode

- If `autoExecuteReadOnly` is enabled in LLM config:
  - Recommendations with `isMutation=false` are automatically dispatched as tasks
  - Mutation recommendations (isMutation=true) require manual approval
  - Users can also manually execute any recommendation via `POST /api/aiops/recommendations/execute`

---

## Agent Internals

### Connection & Reconnection

```
main() → parseFlags() → run() loop with exponential backoff:
  - Success: reset backoff to 1s
  - Failure: 1s → 2s → 4s → 8s → ... → 60s max
```

### Heartbeat Loop

- Runs in a goroutine, every 10 seconds
- Sends `{"type":"heartbeat", "payload":{cpuUsage, memoryUsage, diskUsage}}`
- **Real metrics mode** (`--real` flag): uses gopsutil v4 to collect CPU %, memory %, disk %
  - CPU: `cpu.Percent(0, false)` (primed with 1s interval at startup)
  - Memory: `mem.VirtualMemory().UsedPercent`
  - Disk: `disk.Usage(firstPartition.Mountpoint).UsedPercent`
- **Mock mode** (default): generates jittered values around profile baselines using `base + rand(0,18) - 6` (range: base−6 to base+12)
  - 4 profiles: ubuntu (15/38/29), windows-lab (22/48/61), server (18/41/37), edge-node (35/55/44)

### Command Execution

- Context timeout: configurable per command, max 300 seconds
- Windows: `cmd /C <command>`
- Unix: `/bin/sh -c <command>`
- Template: absolute executable path with pre-defined args
- PATH set to standard system paths
- Output truncated at 256KB per stream (stdout, stderr)

### Credential Storage

- Linux: `/etc/labops-agent/credentials.json` (permissions: 0600, directory: 0750)
- Windows: `%ProgramData%\LabOps\credentials.json`

---

## Maintenance Loop

Runs every 10 seconds (`app.go:maintenanceLoop()`):

1. **Device Expiry:** Marks devices as "offline" if `last_seen < now - heartbeat_timeout` (default 35s)
2. **Task Timeout:** Marks tasks as "timeout" if pending/running longer than `task_timeout` (default 5 min)
3. **Session Pruning:** Deletes expired web sessions (`idle_expires_at` or `absolute_expires_at` passed)
4. **Rate Limiter GC:** Removes rate limiters inactive for 30+ minutes

---

## Security Model

| Layer | Mechanism |
|-------|-----------|
| Password hashing | bcrypt, cost factor 12 |
| Session tokens | 256-bit random, stored as SHA-256 hash |
| CSRF tokens | 256-bit random, double-submit cookie pattern |
| API key encryption | AES-256-GCM with configurable encryption key |
| Rate limiting | Per-IP token bucket: 60/s (general), 5/3min (login), 10/min (enrollment) |
| Password policy | Minimum 12 characters, no reuse, forced change on first login |
| Session security | HttpOnly + Secure + SameSite=Strict cookies, idle timeout 8h, absolute 24h |
| Agent security | Per-device 256-bit secrets, constant-time comparison, one-time enrollment codes |
| Command guard | Ad-hoc commands require explicit `"confirmation": "EXECUTE"`; `isDangerousCommand()` blocks destructive patterns |
| Container security | Non-root users, internal Docker networks, MySQL not exposed to host |
| CORS | Strict origin validation, `Access-Control-Allow-Credentials: true` only for matching origin |
| Request limits | Body size limit 1 MiB |

**Dangerous Command Patterns Blocked:**
`rm -rf /`, `mkfs`, `dd if=`, `shutdown`, `reboot`, `halt`, `poweroff`, `:(){ :|:& };:`, `chmod 777 /`, `> /dev/sda`, and more.

---

## Frontend Architecture

### Routing & Pages

12 pages organized under `AppLayout` (sidebar + header + content area):

| Route | Page | Auto-Refresh | Description |
|-------|------|:---:|-------------|
| `/login` | LoginPage | - | Username/password auth with forced password change |
| `/dashboard` | DashboardPage | 10s | Stats cards, device overview (top 6), online rate gauge, recent tasks/audits |
| `/devices` | DevicesPage | 10s | Searchable device table, create device, revoke with confirmation |
| `/devices/:id` | DeviceDetailPage | 3s | Asset info, live CPU/mem/disk bars, ad-hoc command, task history |
| `/groups` | GroupsPage | 10s | Group table with online counts and rates |
| `/tasks` | TasksPage | 3s | Batch command form (template/ad-hoc), tasks table with expandable output |
| `/audit` | AuditPage | 15s | Audit log table |
| `/aiops` | AiOpsPage | 30s | Health report, LLM analysis, recommendation cards, device insights |
| `/aiops/settings` | AiOpsSettingsPage | - | LLM provider config (OpenAI/Anthropic), model, API key, test connection |
| `/enrollment` | EnrollmentPage | - | One-time enrollment code generation and management |
| `/templates` | TemplatesPage | - | Command template CRUD |
| `/users` | UsersPage | - | User CRUD, role/status management |

### State Management

- **Zustand store** (`auth.ts`): `user`, `mustChangePassword`, persisted to `localStorage`
- **Custom hooks** (`useLoadable.ts`): Generic data fetching with loading/error/refresh states
  - `useLoadable<T>` — single data source with configurable interval
  - `useLoadableAll<T>` — parallel fetch with partial failure handling

### API Client

- Axios instance with `/api` base URL, 15s timeout, credentials enabled
- CSRF interceptor: reads `labops_csrf` cookie, attaches `X-CSRF-Token` header
- 401 interceptor: clears auth state on authentication failure

---

## CI/CD Pipeline

GitHub Actions (`.github/workflows/ci.yml`) runs on push/PR to `master`:

| Job | Runner | Steps |
|-----|--------|-------|
| **server** | ubuntu-latest | Checkout → Go 1.25 → `go vet` → `go test -race` |
| **agent** | ubuntu-latest | Checkout → Go 1.24 → `go vet` → `go test -race` |
| **web** | ubuntu-latest | Checkout → Node 20 → `npm ci` → `tsc --noEmit` → `npm test` → `npm audit --omit=dev` → `npm run build` |
| **containers** | ubuntu-latest | Checkout → `docker compose config --quiet` → `docker build` for all 3 images |

---

## Key Design Decisions

1. **No external Go web framework** — uses only `net/http` with Go 1.22+ pattern-based routing (`mux.HandleFunc("GET /api/devices/{id}", ...)`). Keeps dependency footprint minimal.

2. **Pure-Go SQLite** — `modernc.org/sqlite` eliminates CGO for cross-compilation. SQLite for development, MySQL for production, unified behind a `Dialect` interface.

3. **Single-binary agents** — `CGO_ENABLED=0` static compilation. No runtime dependencies beyond the OS shell and CA certificates.

4. **No mock data** — all data in the system comes from real or simulated agents connected via actual WebSocket. The dev compose launches 4 simulated agents.

5. **Session cookies over JWT** — sessions are revocable server-side (logout, password change invalidates all sessions). CSRF protection via double-submit cookie pattern.

6. **Enrollment codes for agent onboarding** — one-time, time-limited codes exchanged for per-device credentials. No shared secrets between agents.

7. **Chinese-first UI** — all UI labels, LLM prompts, and built-in templates use Chinese (Simplified). Ant Design configured with `zhCN` locale.
