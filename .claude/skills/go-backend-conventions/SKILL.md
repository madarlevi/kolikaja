---
name: go-backend-conventions
description: Use when writing or modifying Go code in apps/api: chi routing and middleware, pgx queries and transactions, service and handler layering, error mapping, logging, configuration and tests. Read before creating any new backend module.
---

# Go backend conventions

## Structure
Follow the layout in `apps/api/CLAUDE.md`. Each module under `internal/<name>` has:
- `handler.go` – parse and validate HTTP input, call the service, write the response. No SQL.
- `service.go` – business rules and authorization decisions. No `net/http` types.
- `repo.go` – SQL only, pgx, parameterized. Returns domain types.
- `types.go` – domain structs and request/response DTOs. DTOs are separate from DB rows.

## Router (chi)
- Global middleware order: request ID -> real IP (only behind the trusted proxy) -> structured logger -> panic recovery -> security headers -> body size limit -> CORS -> timeout.
- Route groups: `/api/v1/public`, `/api/v1/auth`, `/api/v1/me`, `/api/v1/dormitories/{dormitoryID}`, `/api/v1/admin`.
- Auth middleware injects a principal into the context; handlers read it with a typed helper.
- Rate limits: separate buckets for login/register (strict), refresh, reads, writes, and uploads. Keyed by user ID when authenticated, otherwise by IP.

## Database (pgx)
- One `pgxpool.Pool` created in `platform/db`. Pool size from config.
- Always pass `ctx`. Use `QueryRow(...).Scan` with explicit columns, never `SELECT *`.
- Multi-step writes use `pgx.BeginTxFunc`. Keep transactions short; no HTTP calls inside them.
- Map `pgconn.PgError` codes (23505 unique violation, 23503 FK violation) to domain errors in the repo layer.

## Errors
- Domain errors are sentinel values or typed errors (`ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrValidation`).
- The handler maps them to the error envelope from `api-contract`. Anything else becomes HTTP 500 with a generic message and a logged request ID.

## Configuration
- Read from environment variables once at startup in `internal/config`.
- Fail fast when a required secret is missing or too short. Never provide a default JWT secret or database password.
- Print the effective non-secret configuration at startup, never secret values.

## Logging
- `slog` with JSON handler. Include `request_id`, `user_id` (when known), route and status.
- Redact: `Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, passwords, tokens, email addresses in error paths where possible.

## Tests
- Table-driven unit tests for services with fake repos.
- Repo and migration tests against a real PostgreSQL when available.
- Handler tests with `httptest` for status codes and error envelope shape.
- Authorization tests per `authz-tenant-isolation`.
