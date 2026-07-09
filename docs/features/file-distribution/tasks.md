# File Distribution — Tasks

> Parent: [design.md](./design.md) · Version: v0.3

## Phase 1: Server Store & Types

- [ ] 1.1 Add `file_tasks` table to `store.go` Init() schema
- [ ] 1.2 Define `FileTask` struct in `types.go`
- [ ] 1.3 Implement `CreateFileTask` (INSERT with all fields)
- [ ] 1.4 Implement `GetFileTask` (SELECT by id, LEFT JOIN devices)
- [ ] 1.5 Implement `ListFileTasks` (SELECT all with device name, LIMIT 200)
- [ ] 1.6 Implement `MarkFileTaskRunning` (UPDATE status=running)
- [ ] 1.7 Implement `CompleteFileTask` (UPDATE status + finished_at)
- [ ] 1.8 Add `idx_file_tasks_device_status` index

## Phase 2: Server API

- [ ] 2.1 Implement `POST /api/files` handler (multipart parse, SHA-256, store blob, create FileTasks)
- [ ] 2.2 Implement `GET /api/files` handler (list all)
- [ ] 2.3 Implement `GET /api/files/{id}` handler (single detail)
- [ ] 2.4 Implement `GET /api/files/{id}/blob` handler (serve file, Agent token auth)
- [ ] 2.5 Register routes in `app.go`
- [ ] 2.6 Add file size limit env var (`LABOPS_MAX_FILE_SIZE`, default 50 MiB)
- [ ] 2.7 Add blob cleanup to maintenance loop (delete >7 days old)

## Phase 3: Agent

- [ ] 3.1 Add `case "file_push"` handler in WS read loop
- [ ] 3.2 Implement blob download (HTTP GET with X-Agent-Token)
- [ ] 3.3 Implement atomic file write (`.tmp` → rename)
- [ ] 3.4 Implement SHA-256 verification
- [ ] 3.5 Implement path traversal guard
- [ ] 3.6 Send `file_result` back via WebSocket
- [ ] 3.7 Add download timeout (120s) and size limit (50 MiB)

## Phase 4: Server WS Handler

- [ ] 4.1 Add `case "file_result"` in `agent.go` handleAgentWS loop
- [ ] 4.2 Validate device ownership (GetFileTask + DeviceID compare)
- [ ] 4.3 Call CompleteFileTask with result
- [ ] 4.4 Create audit log for file complete/failed

## Phase 5: Web Console

- [ ] 5.1 Create `FileDistributionPage.tsx` with upload form + task table
- [ ] 5.2 Add `fileApi` to `web/src/api/labops.ts` (uploadFile, fileTasks, fileTask)
- [ ] 5.3 Add `FileTask` type to `web/src/types.ts`
- [ ] 5.4 Add "文件分发" nav item to `AppLayout` sidebar
- [ ] 5.5 Add route in `router.tsx` for `/files`
- [ ] 5.6 Auto-refresh: 5s interval

## Phase 6: Testing

- [ ] 6.1 Server: store_test.go — TestFileTaskCRUD (create, get, list, running, complete)
- [ ] 6.2 Server: api_test.go — TestHandleUploadFile, TestHandleListFileTasks, TestHandleDownloadBlob
- [ ] 6.3 Server: agent_test.go — TestHandleAgentWS_FileResult
- [ ] 6.4 Agent: main_test.go — TestFileDownload, TestFileVerify
- [ ] 6.5 Web: tsc --noEmit + npm run build

## Phase 7: Docs & Integration

- [ ] 7.1 Update `docs/user-manual.md` with §4.9 File Distribution
- [ ] 7.2 Update `docs/log.md` with implementation round
- [ ] 7.3 Update `docs/master-plan.md` — mark 2.4 as ✅
- [ ] 7.4 Update `README.md` — add file distribution to feature list
- [ ] 7.5 Docker Compose integration test (upload → distribute → verify → audit)

## Task Conventions

| Mark | Meaning |
|------|---------|
| `[ ]` | Pending |
| `[~]` | In progress |
| `[x]` | Done |
| `[!]` | Blocked |
