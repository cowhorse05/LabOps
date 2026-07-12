# LabOps Project Report

## Project Summary

LabOps is a lightweight open-source operations system for lab and homelab devices. It implements a real control loop across a web console, Go backend, SQLite storage, and Go agents.

## Why This Scenario

A student usually does not own a lab full of machines. LabOps solves that by using Docker containers as simulated devices while keeping the important engineering parts real: registration, heartbeat, command dispatch, result reporting, and audit logging.

## Technical Highlights

- React + TypeScript + Ant Design web console
- Go HTTP API and WebSocket agent channel
- SQLite persistence
- Agent command execution
- Multi-agent Docker Compose demo
- Audit log for operations visibility

## Current Scope

Implemented for MVP:

- Device registration
- Heartbeats and inventory
- Online/offline status
- Command task creation
- Single-device and group command dispatch
- Task results
- Audit logs

Deferred:

- Sandbox
- Remote desktop
- File distribution
- Kubernetes simulation
- C++ agent

## Demo Flow

1. Start the demo with `.\scripts\dev.ps1`.
2. Open `http://localhost:5173`.
3. Log in with the bootstrap administrator password configured outside the repository and complete the forced password change.
4. View online Docker-simulated devices.
5. Execute a command on one device.
6. Check task output and audit logs.
