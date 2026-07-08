# Product Plan

## Audience

LabOps is for students, small labs, clubs, and homelab users who want a compact full-stack operations project they can run locally.

## MVP

The first version proves one complete loop:

1. Agent starts and connects to the server.
2. Agent registers with name, group, profile, and inventory.
3. Server stores the device and marks it online.
4. Web console lists devices and shows detail.
5. User creates a command task.
6. Server sends the task to the agent.
7. Agent executes the command and returns stdout, stderr, exit code, and duration.
8. Server stores task result and audit log.
9. Web console displays the result.

## Non-Goals

- No production RMM replacement in v1.
- No sandbox isolation in v1.
- No remote desktop in v1.
- No patch management or MDM in v1.
- No Kubernetes control plane in v1.

## Demo Scenario

The local demo uses Docker Compose to start:

- One Go server
- One React web console
- Several Go agent containers

The containers simulate a classroom lab, an Ubuntu server node, and an edge node. This makes the project demonstrable on a single Windows PC without pretending the implementation is only mock data.

## Default Environment

Use Windows + PowerShell + Docker Desktop. WSL is optional and not required for the MVP. Go can be installed later, but the repo also provides Docker-based Go build/test commands.
