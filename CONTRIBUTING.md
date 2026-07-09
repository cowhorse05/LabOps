# Contributing to LabOps

Thanks for your interest in contributing! LabOps is a lightweight open-source operations console for students, labs, clubs, and homelab devices. Whether you are fixing a bug, adding a feature, or improving documentation, we are glad to have you.

## Code of Conduct

Be kind, be constructive, and keep discussions focused on the project. We follow a simple rule: assume good intent and help each other learn.

## Getting Started

1. Fork the repository on GitHub.
2. Clone your fork:
   ```powershell
   git clone https://github.com/YOUR_USERNAME/LabOps.git
   cd LabOps
   ```
3. Add the upstream remote:
   ```powershell
   git remote add upstream https://github.com/cowhorse05/LabOps.git
   ```
4. Create a feature branch:
   ```powershell
   git checkout -b feature/your-feature-name
   ```
5. Make your changes.
6. Run the verification script to ensure everything passes:
   ```powershell
   .\scripts\test.ps1
   ```
7. Commit with a meaningful message and push to your fork.
8. Open a pull request against the `master` branch.

## Development Setup

### Prerequisites

- Windows + PowerShell (primary development platform)
- Docker Desktop
- Node.js 20+ (for web frontend development)
- Go 1.23+ (optional; tests can run inside Docker if Go is not installed locally)

### Running the Demo

The full demo runs with a single command:

```powershell
.\scripts\dev.ps1
```

This starts the Go API server, the React web console, and four simulated agents via Docker Compose. Open `http://localhost:5173` and log in with `admin / admin`.

To stop:

```powershell
.\scripts\compose-down.ps1
```

### Environment Variables

Copy `.env.example` to `.env` and adjust as needed. Key variables:

| Variable | Purpose | Default |
|---|---|---|
| `LABOPS_ADDR` | Server listen address | `:8080` |
| `LABOPS_DB_PATH` | SQLite database path | `data/labops.db` |
| `LABOPS_AGENT_TOKEN` | Agent WebSocket auth token | `dev-agent-token` |
| `LABOPS_WEB_TOKEN` | Web REST API auth token | `dev-token` |
| `VITE_PROXY_TARGET` | Vite dev server proxy target | `http://localhost:8080` |

## Running Tests

Use the verification script to run all checks at once:

```powershell
.\scripts\test.ps1
```

This script:

- Runs `npm run build` in `web/` to verify TypeScript compilation and Vite bundling
- Runs `go test ./...` in `server/` (50 tests) and `agent/` (7 tests)
- Falls back to Docker-based Go tests if Go is not installed locally

Individual module tests:

```powershell
# Web: type-check only (no test suite configured yet)
Push-Location web; npx tsc --noEmit; Pop-Location

# Server tests
Push-Location server; go test ./...; Pop-Location

# Agent tests
Push-Location agent; go test ./...; Pop-Location
```

## Code Style

### Go

- Follow standard Go conventions and effective Go guidelines.
- Run `go vet ./...` before committing.
- Keep packages focused and avoid unnecessary abstraction.
- Tests live alongside source files with `_test.go` suffix.
- Use `modernc.org/sqlite` for SQLite (no CGO dependency).

### TypeScript / React

- Strict mode is enabled in `tsconfig.json`.
- Run `npx tsc --noEmit` before committing to catch type errors.
- Use Ant Design components for UI consistency.
- Use Zustand for state management (keep stores small and focused).
- Place shared types in `web/src/types.ts`.

### Commit Messages

- Use the present tense: "add device filtering" not "added device filtering".
- Keep the first line under 72 characters.
- Reference related issues or PRs where applicable.

## Project Structure

```text
LabOps/
├── web/                # React + TypeScript + Vite + Ant Design + Zustand
│   ├── src/
│   │   ├── api/        # axios client and API functions
│   │   ├── layouts/    # AppLayout (Sider + Header + Content)
│   │   ├── pages/      # Login, Dashboard, Devices, DeviceDetail, Groups, Tasks, Audit
│   │   ├── stores/     # Zustand auth store
│   │   ├── styles/     # global.css
│   │   ├── utils/      # status helpers
│   │   ├── types.ts    # shared TypeScript interfaces
│   │   ├── router.tsx  # React Router configuration
│   │   └── main.tsx    # entry point
│   ├── Dockerfile
│   ├── vite.config.ts
│   └── package.json
├── server/             # Go API server
│   ├── cmd/server/
│   │   └── main.go     # entry point (env parsing, store init, HTTP serve)
│   ├── internal/core/
│   │   ├── types.go    # domain types, constants, wire protocol
│   │   ├── store.go    # SQLite CRUD (devices, tasks, audit, users, sessions)
│   │   ├── app.go      # App aggregate (route registration, CORS, auth, maintenance loop)
│   │   ├── api.go      # REST handlers (health, login, stats, devices, groups, tasks, audit)
│   │   ├── agent.go    # WebSocket handler (registration, heartbeat, command dispatch, results)
│   │   └── store_test.go
│   ├── Dockerfile
│   └── go.mod
├── agent/              # Go agent
│   ├── cmd/agent/
│   │   ├── main.go     # entry point (flag parsing, connect loop, heartbeat, command exec)
│   │   └── main_test.go
│   ├── Dockerfile
│   └── go.mod
├── deploy/             # deployment documentation
├── docs/               # research, product plan, logs, design specs
├── scripts/            # PowerShell utility scripts (dev, test, compose-down)
├── compose.yaml        # Docker Compose demo environment
├── .env.example        # environment variable template
└── .github/workflows/  # CI pipeline
```

## Pull Request Process

1. Ensure all tests pass (`.\scripts\test.ps1`).
2. Update or add documentation if your change affects user-facing behavior.
3. Add a changelog entry in `docs/log.md` describing your change.
4. Keep PRs focused on a single concern — small PRs are easier to review.
5. PRs require at least one approving review before merging.
6. Once approved, the maintainer will squash-merge your PR.

## Reporting Issues

Use GitHub Issues to report bugs or suggest enhancements. A good issue includes:

- **Steps to reproduce** — the exact sequence that triggers the problem.
- **Expected behavior** — what you expected to happen.
- **Actual behavior** — what actually happened (include error messages and logs).
- **Environment details** — OS version, Go version, Docker version, browser if applicable.

Before opening a new issue, check the existing list to avoid duplicates.

## Questions?

If you have questions that do not fit a GitHub Issue, feel free to start a Discussion on the repository. We are happy to help.

Thanks for contributing!
