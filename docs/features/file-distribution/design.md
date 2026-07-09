# File Distribution — Design SSOT

> Status: 🚧 Design · Version: v0.3 · Date: 2026-07-09

## 1. Summary

Add file upload, distribution, and SHA-256 verification to the LabOps control loop. A user uploads a file via the web console, selects target devices or groups, and the server distributes the file to agents. Agents download, verify, and report results back — all audited.

## 2. Data Flow

```
Web ──POST /api/files (multipart)──▶ Server
  │                                    │
  │  1. Upload                     2. Store file on disk
  │  3. Create FileTask per device 4. Notify online agents
  │                                    │
Agent ◀──WebSocket "file_push"───────Server
  │
  │  5. Download via HTTP GET /api/files/{id}/blob
  │  6. SHA-256 verify
  │  7. Report result via WS "file_result"
  │
Server ◀──WebSocket "file_result"────Agent
  │
  │  8. Update FileTask status
  │  9. Create audit log
```

## 3. API

### 3.1 REST

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| POST | `/api/files` | Bearer | Upload file (multipart) → create FileTasks |
| GET | `/api/files` | Bearer | List file tasks |
| GET | `/api/files/{id}` | Bearer | File task detail |
| GET | `/api/files/{id}/blob` | Agent | Download file blob (Agent token auth) |

### 3.2 POST /api/files request

```
Content-Type: multipart/form-data
Fields:
  file:       <binary>        (required, max 50 MiB)
  deviceId:   string          (optional — single device target)
  groupName:  string          (optional — group target)
  path:       string          (required — destination path on agent, e.g. /app/config.yml)
```

### 3.3 WebSocket Protocol

**Server → Agent** (new message type):

```json
{
  "type": "file_push",
  "payload": {
    "taskId": "file-abc123",
    "url": "http://server:8080/api/files/file-abc123/blob",
    "path": "/app/config.yml",
    "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "token": "dev-agent-token"
  }
}
```

**Agent → Server** (new message type):

```json
{
  "type": "file_result",
  "payload": {
    "taskId": "file-abc123",
    "status": "success",
    "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "error": ""
  }
}
```

## 4. Data Model

### file_tasks table

```sql
CREATE TABLE IF NOT EXISTS file_tasks (
    id              TEXT PRIMARY KEY,
    device_id       TEXT NOT NULL,
    file_name       TEXT NOT NULL,       -- original filename
    file_path       TEXT NOT NULL,       -- dest path on agent
    file_size       INTEGER NOT NULL,    -- bytes
    sha256          TEXT NOT NULL,       -- expected hash
    storage_path    TEXT NOT NULL,       -- server-side blob path
    status          TEXT NOT NULL,       -- pending/running/success/failed/verify_failed
    requested_by    TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    started_at      TEXT,
    finished_at     TEXT
);
```

Status constants: `pending`, `running`, `success`, `failed`, `verify_failed`.

## 5. Server Implementation Plan

### 5.1 Store layer (`store.go`)

New methods:
- `CreateFileTask(ctx, deviceID, fileName, filePath string, fileSize int64, sha256, storagePath, requestedBy string) (FileTask, error)`
- `GetFileTask(ctx, id string) (FileTask, bool, error)`
- `ListFileTasks(ctx) ([]FileTask, error)`
- `MarkFileTaskRunning(ctx, id string) error`
- `CompleteFileTask(ctx, id, status, sha256, error string) error`

### 5.2 API layer (`api.go`)

New handlers:
- `handleUploadFile` — parse multipart, compute SHA-256, store blob, create FileTasks per target device, return `{tasks, errors?}`
- `handleListFileTasks` — list all file tasks
- `handleGetFileTask` — single file task detail
- `handleDownloadFileBlob` — serve stored blob (Agent token auth, verify device ownership)

### 5.3 Agent handler (`agent.go`)

New WS message handler:
- `case "file_result"` — validate device ownership, verify SHA-256 match, update FileTask, create audit

### 5.4 Blob storage

- Store files under `data/blobs/<taskId>/<original-filename>`
- Cleanup policy: delete blobs older than 7 days via maintenance loop
- Max file size: 50 MiB (configurable via `LABOPS_MAX_FILE_SIZE`)

## 6. Agent Implementation Plan

### 6.1 New handler

In `agent/cmd/agent/main.go`, add `case "file_push"` in the WS read loop:

```
1. Receive file_push message
2. HTTP GET the blob URL with X-Agent-Token header
3. Write file to destination path (create parent dirs)
4. Compute SHA-256 of written file
5. Compare with expected SHA-256
6. Send file_result back via WebSocket
```

### 6.2 Safety constraints

- Max download size: 50 MiB (match server limit)
- Timeout: 120s per download
- Atomic write: write to `.tmp` file, then rename (prevents partial writes)
- Path traversal prevention: reject paths containing `..` or absolute paths outside allowed directory

## 7. Web Console

### 7.1 New page: FileDistributionPage (`/files`)

- Upload form (file picker + target device/group selector + destination path)
- File task table (columns: device, filename, path, size, status, SHA-256, time)
- Status tags: pending (blue), running (orange), success (green), failed (red), verify_failed (red)
- Expand row to see error details for failed tasks
- Auto-refresh: 5s

### 7.2 Navigation

- Add "文件分发" (File Distribution) to AppLayout sidebar, after "任务管理"

## 8. Audit Events

| Action | Trigger |
|--------|---------|
| `file.upload` | User uploads file, one per device task |
| `file.push` | Server notifies agent via WS |
| `file.download` | Agent begins blob download |
| `file.complete` | File written + SHA-256 verified OK |
| `file.failed` | Download/write/verify error |

## 9. Acceptance Criteria

- [ ] Upload a file via web → appears in file task table
- [ ] File distributed to single device → agent writes to correct path
- [ ] File distributed to group → one task per group member
- [ ] SHA-256 mismatch → task marked `verify_failed`
- [ ] Offline device → task stays `pending`, dispatched on reconnect
- [ ] Audit log records complete chain: upload → push → download → complete
- [ ] Blob cleanup removes files older than 7 days
- [ ] Path traversal attack rejected by agent

## 10. Non-Goals

- File versioning or delta sync
- Directory upload (zip only MVP)
- Agent → Server file upload (log collection is v0.3 separate feature)
- Encrypted file transfer (TLS is a deployment concern)
- Resumable downloads

## 11. Dependencies

- Server: `io.LimitReader` (stdlib), `crypto/sha256` (stdlib), `mime/multipart` (stdlib)
- Agent: `net/http` (stdlib), `crypto/sha256` (stdlib), `os.Rename` (stdlib)
- No new external Go dependencies required
- Web: Ant Design `Upload` component (already installed)
