# Local Demo

The root `compose.yaml` is the primary demo environment.

```powershell
.\scripts\dev.ps1
```

Services:

- `server`: Go API server on `http://localhost:8080`
- `web`: Vite web console on `http://localhost:5173`
- `agent-lab-pc-01`: simulated Windows lab PC
- `agent-lab-pc-02`: simulated Ubuntu lab PC
- `agent-server-01`: simulated homelab server
- `agent-edge-01`: simulated edge node

Login:

```text
admin / admin
```

Suggested demo commands:

```sh
hostname && date
uname -a
ls -la /app
```

To test offline status, stop one agent:

```powershell
docker compose stop agent-edge-01
```

The server marks it offline after the heartbeat timeout.
