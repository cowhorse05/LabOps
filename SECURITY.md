# Security Policy

## Supported Versions

LabOps is currently in early development (pre-1.0). Security updates are provided for the latest minor release.

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2    | :x:                |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

If you discover a security vulnerability in LabOps, please report it privately by emailing the project maintainer directly (contact information is available in commit history and GitHub profile). You should receive a response within 48 hours.

We follow a coordinated disclosure process:

1. You report the vulnerability privately.
2. We acknowledge receipt and begin investigating.
3. We develop and test a fix.
4. We release a patch and publish a security advisory.
5. You receive credit in the advisory (unless you prefer to remain anonymous).

## Security Considerations

LabOps is an MVP/demo project. The following points are important for anyone deploying it:

- **There are no production default credentials.** Production startup requires an operator-supplied bootstrap password and rejects the legacy `admin/admin` password.
- **Agent command execution runs with the agent process's permissions.** Commands dispatched to agents execute under the user account running the agent binary. Use OS-level sandboxing (containers, restricted service accounts, AppArmor/SELinux profiles) to limit the blast radius.
- **Network-exposed deployments require HTTPS/WSS.** The production Compose publishes only Nginx ports 80/443 and keeps MySQL/Server on an internal network.
- **SQLite has no built-in access control.** The database file is protected only by filesystem permissions. Ensure the database file (`data/labops.db`) is not served or exposed by the web server.
- **Device credentials are unique and revocable.** Enrollment codes and device secrets are only returned once; revoke a lost or decommissioned device immediately.

## Scope

The following are considered in-scope for security reports:

- Authentication and authorization bypasses
- Command injection or arbitrary code execution via the task API or agent channel
- SQL injection
- Path traversal in file operations (current and planned features)
- Information disclosure of sensitive data (tokens, audit logs, device details)

The following are considered out-of-scope:

- Denial of service against a local development setup
- Social engineering or phishing
- Issues in third-party dependencies that do not affect LabOps specifically (report those to the upstream project)

## Acknowledgments

We appreciate the security community's help in keeping LabOps safe. Researchers who report valid vulnerabilities will be acknowledged in the advisory and this document.
