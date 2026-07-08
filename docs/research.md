# Research Notes

## External Platforms

LabOps borrows product ideas from existing operations platforms but deliberately keeps the MVP smaller.

| Platform | Useful Reference | LabOps Decision |
| --- | --- | --- |
| MeshCentral | Agent-based remote management, terminal, file management, remote desktop | Reference the product breadth; do not implement remote desktop in MVP |
| Tactical RMM | RMM workflows, tasks, audit, agent control | Reference the operations model; avoid full patching/automation scope |
| Fleet | Device inventory and security visibility | Reference asset reporting and device detail views |
| Portainer | Local container demo and resource management UX | Use Docker to simulate devices; do not build a container management platform |
| Zabbix / Netdata | Heartbeat, metrics, and status-oriented dashboards | Add lightweight health/status views only |
| Apache Guacamole | Clientless remote desktop gateway | Keep as a later remote desktop research path |

Reference links:

- MeshCentral: https://github.com/Ylianst/MeshCentral
- Tactical RMM: https://docs.tacticalrmm.com/
- Fleet: https://fleetdm.com/
- Portainer: https://github.com/portainer/portainer
- Zabbix: https://www.zabbix.com/
- Netdata: https://www.netdata.cloud/open-source/
- Apache Guacamole: https://guacamole.apache.org/

## Local OpsService Reference

Local reference project:

```text
C:\Users\cowho\Desktop\code\Ops-Sys\OpsService
```

Observed structure:

- `client/`: C++17 Windows service-style agent, with modules for reporting, terminal, file push, logs, screenshots, remote desktop, and software management
- `server/`: Java Spring Boot backend with DDD-style layers
- `web/`: React + TypeScript + Vite + Ant Design console
- `docs/`: API contracts, PRD, architecture, schema, and feature design notes

What LabOps keeps:

- Product shape: dashboard, devices, detail page, command terminal, audit
- Frontend stack: React, TypeScript, Vite, Ant Design, Zustand
- Architecture habit: separate web/server/agent, API-first documentation, task/audit model
- Client capability direction: `device_report`, `terminal`, `file_push`, `log`

What LabOps does not copy:

- Closed-source frameworks
- Company private SDKs
- Private IoT platform integrations
- Internal domains or business-specific behavior
- Remote desktop implementation in MVP

## Positioning

LabOps is a local-first, student-friendly operations control plane. It should be easy to demonstrate on one PC by running several agent containers. The system must not be a screenshot-only mock: the server, agent protocol, command dispatch, persistence, and audit flow should all be real.
