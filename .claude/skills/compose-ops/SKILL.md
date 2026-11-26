---
name: compose-ops
description: Use when writing or changing Dockerfiles, docker-compose.yml, .env.example, reverse proxy config, health checks, volumes, backup or restore scripts, or deployment documentation. Covers non-root images, secret handling, migrations job and operator runbooks.
---

# Docker Compose and deployment

Docker may be unavailable in the working environment (see `docs/ENVIRONMENT.md`). When it is unavailable, validate statically (`docker compose config` if the CLI exists, YAML lint, `hadolint`) and record the runtime steps as "not verified".

## Images
- Multi-stage builds. Build stage with the toolchain; final stage distroless or alpine with only the binary or static assets.
- Go API: static binary (`CGO_ENABLED=0`), `USER 65532:65532`, `EXPOSE 8080`, `HEALTHCHECK` using a `/healthz` endpoint or the binary's own probe.
- Web: build with Node, serve static files with an unprivileged nginx or caddy image running as non-root on port 8080.
- Pin base image versions by tag; document how to pin by digest.
- `.dockerignore` excludes `.env*`, `node_modules`, `.git`, test artifacts.

## docker-compose.yml services
- `postgres` (persistent volume, health check with `pg_isready`, restart `unless-stopped`).
- `minio` (persistent volume, health check on `/minio/health/live`, bucket created by a one-shot `minio-init` job using `mc`).
- `migrate` (one-shot job running golang-migrate; the API depends on its successful completion).
- `api` (depends on healthy postgres and minio, and completed migrate).
- `web` (static frontend).
- `proxy` (optional reverse proxy with TLS termination configured by the operator).
- Networks: separate internal network for postgres and minio; only proxy exposed to host.
- No container runs privileged. Read-only root filesystem where possible.

## Secrets and configuration
- `.env.example` contains only placeholders such as `CHANGE_ME_GENERATE_WITH_scripts/gen-secrets.sh`.
- `scripts/gen-secrets.sh` generates random values for JWT keys, database and MinIO passwords into `.env` (not committed).
- Compose uses `${VAR:?message}` syntax so missing required values stop startup with a clear message.
- No default production credentials anywhere, including documentation examples.

## Backup and restore
- `deploy/scripts/backup.sh`: `pg_dump --format=custom` plus `mc mirror` for MinIO, timestamped, optional encryption with a passphrase from env.
- `deploy/scripts/restore.sh`: restores dump and mirror into an empty stack, with a confirmation prompt.
- Document the procedure in `deploy/README.md` with a test restore step.

## Documentation to produce
- `README.md` quick start: `git clone`, `cp .env.example .env`, `scripts/gen-secrets.sh`, `docker compose up -d --build`, first admin creation, health check URLs.
- Operator responsibilities: TLS, backups, updates, access control, data protection obligations (link to `PRIVACY.md`).
