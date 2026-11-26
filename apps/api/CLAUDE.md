# apps/api – Go backend rules

## Stack
Go (latest stable), chi router, pgx (pgxpool), golang-migrate, Argon2id (`golang.org/x/crypto/argon2`), JWT (EdDSA or HS256 with a secret from env, decided in ARCHITECTURE.md), OpenAPI 3.1 contract at `docs/api/openapi.yaml`.

## Layout
```
cmd/api/              main, wiring only
internal/config/      env parsing, fail-fast validation
internal/http/        router, middleware (auth, csrf, rate limit, security headers, request size limit, request id)
internal/<module>/    handler.go (HTTP), service.go (use cases), repo.go (SQL), types.go
internal/platform/    db pool, storage (MinIO), logging, errors
migrations/           NNNN_name.up.sql / NNNN_name.down.sql
tests/                integration and authorization tests
```

Dependencies point inward: handler -> service -> repo. Handlers never import repo. Services never import net/http.

## Rules
- Every handler validates input explicitly (length, format, allowed values). Never bind JSON straight into a database model.
- Limit request bodies with `http.MaxBytesReader`.
- Errors: wrap with `%w`; map to API error envelope at the handler boundary. Log internal details server-side only.
- Logging: `log/slog`, JSON in production. Never log passwords, tokens, cookies, Authorization headers, API keys, full refresh tokens.
- Context: every DB call takes `ctx`. Set statement timeouts.
- Transactions: use `pgx.BeginTxFunc` for multi-row writes (rating + recommendation, membership changes, token rotation).
- Time: store `timestamptz`, use UTC in code.
- IDs: UUIDv7 or UUIDv4 generated in Go; decided in ARCHITECTURE.md.
- Soft delete only where SPEC requires it; include `deleted_at` filters in every read query.

## Commands (run before reporting done)
- `gofmt -l .` must print nothing
- `go vet ./...`
- `go test ./...`
- `golangci-lint run` (if installed; otherwise record as not run)
- `go run ./cmd/api` only with a local database; otherwise record as not verified

## Migrations
- Use the `db-migration` skill.
- The migration runner is a separate command (`cmd/migrate`) that the Compose migrations job calls.

## Tests
- Unit tests next to code.
- Authorization tests in `tests/authz/`: for every dormitory-scoped endpoint, a user from dormitory A must get 403 or 404 on dormitory B resources. Use the `authz-tenant-isolation` skill.
- Integration tests need PostgreSQL; use `testcontainers-go` only if Docker is available (see `docs/ENVIRONMENT.md`).
