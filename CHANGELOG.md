# Changelog

Detailed change logs are maintained in [`docs/log.md`](docs/log.md). This file highlights notable user-facing changes and version milestones.

## Recent Highlights

### v0.2.0 (2026-07-09)

- SQL transaction safety for the full task lifecycle (create, claim, complete) with TOCTOU race protection
- Four database performance indexes for faster query patterns
- Dedicated device task API endpoint (`GET /api/devices/:id/tasks`)
- WebSocket handler integration tests for agent registration and heartbeat
- GitHub Actions CI workflow (Go tests, go vet, TypeScript build)
- v0.3 file distribution design specification (see `docs/features/file-distribution/`)
- Audit log comprehensive coverage

### v0.1.0 (2026-07-08)

Initial MVP release with the following capabilities:

- Agent registration, heartbeat, and online/offline status tracking
- Single-device and batch (group) command execution via WebSocket
- Task result tracking with stdout, stderr, and exit code capture
- React + Ant Design web console with 8 pages (Dashboard, Devices, Groups, Tasks, Audit, Login)
- Go API server with 14 REST endpoints + WebSocket agent channel
- SQLite storage with full CRUD for devices, tasks, audit log, users, and sessions
- Docker Compose demo environment with 4 simulated agents
- AI Ops health analysis engine (health scores, threshold alerts, group summaries)
- Audit trail with complete operation traceability

---

See [`docs/log.md`](docs/log.md) for the complete history with per-round details.
