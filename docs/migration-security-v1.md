# Security migration v1

Startup creates `schema_migrations`, applies migration 1, and records it only after every required column exists. Migration failures stop startup.

Migration 1 adds Web sessions, enrollment codes, device credentials, command templates, task execution metadata, device credential state, and richer audit actor fields. Existing devices become `pending_reenrollment`; existing tasks and audits remain queryable.

Before upgrading MySQL, run `scripts/backup.sh` and verify gzip integrity. Changes are additive, so the previous application image can run during rollback; do not remove new columns in this milestone.
