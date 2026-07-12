# Security model

- Web authentication uses revocable opaque sessions. Session cookies are `Secure`, `HttpOnly`, and `SameSite=Strict`; state-changing requests require the matching CSRF cookie/header.
- Roles are fixed to `admin`, `operator`, and `viewer`. Backend permission checks are authoritative; hidden buttons are only a UX aid.
- Enrollment codes are hashed, expire after ten minutes by default, and can be single-use. Each device receives a separate 256-bit secret; only its SHA-256 hash is stored.
- Agents authenticate WSS with `Authorization: Agent <deviceId>:<secret>`. Revocation closes the active socket and blocks reconnects.
- Templates execute absolute binaries with an argument array. Only admins can submit ad-hoc shell commands, and the UI requires confirmation.
- The Linux service runs as `labops-agent` with systemd hardening and no automatic privilege elevation.
- LLM keys are encrypted with AES-256-GCM using `LABOPS_ENCRYPTION_KEY`. API responses never return the full key.
- Production startup fails on an empty database without a 12+ character bootstrap password, on a legacy `admin/admin` account without a replacement password, or without the public origin/encryption key.

Known boundary: an administrator-approved ad-hoc command remains equivalent to remote shell access under the Agent service account. Restrict administrator accounts and prefer templates.
