# Secure API additions

All Web endpoints except health, login, Agent enrollment, and Agent WebSocket require the session cookie. POST/PUT/PATCH/DELETE requests also require `X-CSRF-Token` matching `labops_csrf`.

- `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`
- `GET|POST /api/users`, `PUT /api/users/{id}` — admin
- `GET|POST /api/enrollment-codes`, `DELETE /api/enrollment-codes/{id}` — admin
- `POST /api/agent/enroll` — one-time enrollment code
- `POST /api/devices/{id}/revoke` — admin
- `GET /api/command-templates` — authenticated; `POST|PUT` — admin
- `POST /api/tasks`:
  - template: `{deviceId|groupName, kind:"template", templateId, arguments}`
  - ad hoc: `{deviceId|groupName, kind:"ad_hoc", command, confirmation:"EXECUTE"}` — admin

Errors use `{error, code, message, requestId}`. `error` remains during the v0.x transition for older UI error handling.
