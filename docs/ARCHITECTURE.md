# KoliKaja – architecture

Status: written in phase 1 (2026-11-26). Sections about later phases describe the planned design;
they become binding when the phase is implemented, and this document is updated whenever the design changes.
Order of authority: `docs/DECISIONS.md` > `docs/SPEC.md` > this document.

Contents
1. Overview
2. Repository layout
3. Backend structure
4. Authentication and sessions
5. Authorization and tenant isolation
6. Data model
7. Menus, ratings and recommendations
8. Photo pipeline
9. Notifications and announcements
10. Moderation, reputation and audit
11. Rate limits, request limits and security headers
12. API contract and client generation
13. Deployment topology
14. Clients
15. Logging, privacy and telemetry
16. Licensing notes
17. Extension points (out of scope for this version)
18. Verification status and open questions

---

## 1. Overview

KoliKaja is a self-hostable, multi-tenant platform. Each dormitory is a tenant. One Go API is the only
source of truth; the web app, the Android app and the iOS app use the same versioned REST API (`/api/v1`).

```
                 ┌──────────────────────── docker compose ───────────────────────────┐
 Browser ──HTTPS─┤► web (Caddy, non-root)                                            │
 (PWA)           │    ├─ /            static React build                             │
                 │    ├─ /api/*  ───► api (Go, distroless, non-root) ──► postgres     │
 Android ─HTTPS──┤    ├─ /healthz, /readyz ─► api                     └──► minio      │
 iOS ─────HTTPS──┤    └─ /storage/* ─► minio (presigned POST uploads only)            │
                 │ migrate (one-shot, same image as api) ──► postgres                 │
 Integrations ───┤ minio-init (one-shot, creates bucket, least-privilege user)        │
 (X-API-Key)     └────────────────────────────────────────────────────────────────────┘
 TLS: terminated by the operator's proxy or by Caddy (operator choice, documented in phase 3).
```

Design goals, in order: security and tenant isolation, low resource use (single small VM), maintainability,
user experience. No microservices: one API process, one database, one object store.

## 2. Repository layout

```
apps/api/                  Go backend (module github.com/madarlevi/kolikaja/apps/api)
  cmd/api/                 HTTP server (wiring only); `api healthcheck` subcommand for Docker
  cmd/migrate/             migration runner (golang-migrate, migrations embedded with go:embed)
  cmd/admin/               operator CLI: promote first global admin, issue password reset token
  internal/config/         environment parsing, fail-fast validation
  internal/http/           router, middleware/ (request id, logging, recovery, security headers,
                           body limit, CORS, auth, CSRF, rate limit)
  internal/platform/       db (pgxpool), storage (S3 client), logging (slog), apierror, clock
  internal/authz/          principal, membership lookup, role checks (shared by all modules)
  internal/<module>/       auth, users, dormitories, memberships, invitations, menus, ratings,
                           recommendations, comments, photos, announcements, notifications,
                           moderation, apikeys, audit, stats, admin
                           each: handler.go, service.go, repo.go, types.go (+ _test.go)
  migrations/              NNNN_name.up.sql / NNNN_name.down.sql
  tests/authz/             cross-tenant authorization tests (one table per endpoint)
  tests/integration/       repository and HTTP tests against PostgreSQL
apps/web/                  React + TypeScript + Vite (src/app, src/features, src/api, src/components/ui,
                           src/i18n, src/lib)
apps/android/              Kotlin + Compose, package io.github.madarlevi.kolikaja
apps/ios/                  SwiftUI app (KoliKaja/App, Core/Network, Core/Auth, Features, Resources)
deploy/                    docker/ (Dockerfiles), proxy/ (Caddyfile), scripts/ (backup, restore, secrets)
docs/                      SPEC, DECISIONS, ARCHITECTURE, PROGRESS, ENVIRONMENT, api/openapi.yaml
scripts/                   repository tooling (client generation, license audit)
redocly.yaml               OpenAPI lint configuration (+ .redocly.lint-ignore.yaml)
```

## 3. Backend structure

### 3.1 Layers
`handler -> service -> repo`, dependencies point inward. Handlers parse and validate HTTP input and map
errors; services hold business rules and **all authorization decisions**; repos contain parameterized SQL
only (pgx placeholders, explicit column lists, no string building). DTOs are separate from DB rows.

### 3.2 Middleware order
request ID (server-generated) → real IP (only from `TRUSTED_PROXIES`) → structured logger → panic recovery →
security headers → body size limit → CORS → timeout → authentication (one credential type per request) →
CSRF (cookie-authenticated unsafe methods) → rate limit → handler.

### 3.3 Route groups
`/api/v1/meta`, `/api/v1/auth/*` (public or refresh-only), `/api/v1/me/*`, `/api/v1/avatars/*`,
`/api/v1/dormitories/*` (directory plus `/{dormitoryId}/...` tenant routes), `/api/v1/invitations/*`,
`/api/v1/admin/*` (global administrators only), plus `/healthz` and `/readyz` at the root.
The full list is the OpenAPI contract (§12).

### 3.4 Configuration (environment variables)
Read once at startup in `internal/config`. Missing or weak secrets stop the process with a clear message.
There are no default secrets anywhere.

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `KOLIKAJA_ENV` | no | `production` | `development` relaxes only `AUTH_COOKIE_SECURE` |
| `HTTP_ADDR` | no | `:8080` | |
| `PUBLIC_BASE_URL` | yes | – | e.g. `https://kolikaja.example`; used for Origin checks, JWT `iss`/`aud` |
| `DATABASE_URL` | yes | – | pgx DSN; built by Compose from `POSTGRES_*` |
| `DB_MAX_CONNS` | no | `10` | |
| `DB_STATEMENT_TIMEOUT` | no | `5s` | set per connection |
| `JWT_SECRET` | yes | – | ≥ 32 random bytes (base64); HS256 signing key |
| `JWT_SECRET_PREVIOUS` | no | – | accepted for verification only, for key rotation |
| `CSRF_SECRET` | yes | – | ≥ 32 random bytes (base64); HMAC key for CSRF tokens |
| `ACCESS_TOKEN_TTL` | no | `10m` | |
| `REFRESH_TOKEN_TTL` | no | `720h` (30 days) | idle lifetime of each refresh token |
| `SESSION_MAX_AGE` | no | `2160h` (90 days) | absolute session lifetime |
| `AUTH_COOKIE_SECURE` | no | `true` | `false` allowed only with `KOLIKAJA_ENV=development` |
| `S3_ENDPOINT` | yes | – | internal endpoint, e.g. `minio:9000` |
| `S3_PUBLIC_URL` | yes | – | URL that clients POST uploads to, e.g. `https://kolikaja.example/storage` |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | yes | – | least-privilege MinIO user, never the root user |
| `S3_BUCKET` | no | `kolikaja-media` | private bucket |
| `S3_REGION` | no | `us-east-1` | fixed so signing needs no network call |
| `PHOTO_MAX_BYTES` | no | `10485760` | upper bound 25 MiB (DB check) |
| `CORS_ALLOWED_ORIGINS` | no | empty | cross-origin access is off by default |
| `TRUSTED_PROXIES` | yes in Compose | empty | CIDRs whose `X-Forwarded-For` is honoured. Compose sets it to the fixed subnet of the `frontend` network; the API uses the right-most address that is not a trusted proxy. When empty in production the API logs a warning, because every client would then share the proxy's IP for rate limits (§11). |
| `DORMITORY_CREATION` | no | `global_admins` | or `all_users` |
| `INSTANCE_NAME` | no | `KoliKaja` | |
| `SOURCE_CODE_URL` | no | upstream repository | **must** point at the modified source if the operator changes the code (§16) |
| `PRIVACY_POLICY_URL`, `TERMS_URL`, `TRADEMARKS_URL`, `OPERATOR_CONTACT` | no | empty | `TODO(operator)`; returned by `GET /meta` |
| `ARGON2_MAX_CONCURRENCY` | no | `4` | bounds memory used by password hashing |
| `AUDIT_LOG_RETENTION_DAYS` | no | `365` | minimum 30 (the database rejects deleting younger rows) |
| `SOFT_DELETE_RETENTION_DAYS` | no | `30` | purge of soft-deleted content |
| `LOG_LEVEL` | no | `info` | |

### 3.5 Errors
Domain errors (`ErrNotFound`, `ErrForbidden`, `ErrConflict`, `ErrValidation`, …) are mapped at the handler
boundary to the `Error` envelope of the contract (`code`, `message`, `details`, `request_id`). Anything
unexpected becomes `500 internal_error` with the request ID; the cause is logged server-side only.
JSON bodies are decoded with `DisallowUnknownFields` (unknown field → `400`), then validated explicitly (`422`).

### 3.6 Background jobs
Run inside the API process on tickers. Each job takes a PostgreSQL advisory lock
(`pg_try_advisory_lock`) so running several API replicas stays safe.
- expire abandoned uploads (`pending_upload` past `upload_expires_at`, objects deleted);
- delete expired or revoked sessions and refresh tokens 30 days after expiry;
- flush API key usage counters (`last_used_at`, `use_count`) every minute;
- retention purge: audit logs, read notifications older than 90 days, soft-deleted content after
  `SOFT_DELETE_RETENTION_DAYS` (only content without open reports, see §6.4).

### 3.7 Time and IDs
All timestamps are `timestamptz`, handled in UTC in Go. Dormitory-local dates use the dormitory's IANA time
zone (`time/tzdata` is embedded so the distroless image needs no zoneinfo). IDs are **UUIDv7**
(`github.com/google/uuid`), generated in Go: time-ordered for index locality and cursor pagination; the
embedded creation time is not sensitive because `created_at` is returned anyway.

## 4. Authentication and sessions

### 4.1 Passwords
Argon2id (`golang.org/x/crypto/argon2`), parameters m = 19 456 KiB (19 MiB), t = 2, p = 1, 16-byte salt,
32-byte key (OWASP minimum, chosen for low memory). Stored as PHC string `$argon2id$v=19$m=19456,t=2,p=1$…`.
At most `ARGON2_MAX_CONCURRENCY` hashes run at once (peak ≈ 76 MiB). Hashes with outdated parameters are
upgraded on the next successful login. Length 12–128 code points; the most common passwords and the e-mail
address itself are rejected. Login with an unknown e-mail verifies against a dummy hash so timing does not
reveal registered addresses. Login, registration and password reset are rate limited (§11).

### 4.2 Access tokens
JWT HS256 signed with `JWT_SECRET` (with `kid` to support `JWT_SECRET_PREVIOUS` during rotation).
Claims: `sub` (user ID), `sid` (session ID), `iat`, `exp` (10 minutes), `iss` and `aud` (= `PUBLIC_BASE_URL`),
`typ` = `access`. The verifier pins the algorithm and rejects `none` and all others.
HS256 instead of EdDSA because only the API both signs and verifies; a symmetric key is one secret fewer
for operators to manage. On every authenticated request the API also loads the session by primary key and
rejects revoked or expired sessions, so logout, "log out everywhere", password change and bans take effect
immediately rather than after up to 10 minutes. Only `POST /auth/logout` accepts an access token whose
`exp` has passed (signature, `iss`, `aud` and `typ` still verified), solely to find the session to revoke;
it always clears the web cookies.

### 4.3 Sessions and refresh-token rotation
A row in `user_sessions` is one refresh-token family. Refresh tokens are 32 random bytes (`crypto/rand`,
base64url). Only their SHA-256 hash is stored in `refresh_tokens`.

Refresh (one transaction):
1. Look up the token hash together with its session. Unknown → `401 invalid_token`.
2. The session's `client_type` must match the endpoint (`web` for `/auth/web/refresh`, `native` for
   `/auth/native/refresh`), otherwise `401 invalid_token`. A web refresh token therefore cannot be
   exchanged on the native endpoint to skip the CSRF and Origin checks.
3. Token already used (`used_at IS NOT NULL`) → **reuse**, regardless of its expiry: revoke the session
   (`revoke_reason = 'refresh_token_reuse'`), write `auth.refresh_token_reused` to the audit log,
   return `401 session_revoked`.
4. Token past its own `expires_at` (idle timeout) → `401 invalid_token`.
   Session revoked or past its `expires_at` (absolute lifetime) → `401 session_revoked`.
5. `UPDATE refresh_tokens SET used_at = now() WHERE id = $1 AND used_at IS NULL AND expires_at > now()`.
   If no row changed, a concurrent request used it first → treat as reuse (step 3).
6. Insert the successor token (expires at `min(now + REFRESH_TOKEN_TTL, session.expires_at)`), update
   `last_used_at`, issue a new access token.
Phase 2 adds tests for each branch (expired token, wrong client type, reuse, concurrent use).

The partial unique index `refresh_tokens_session_active_key` (at most one unused token per session) makes
two concurrent refreshes with the same token impossible even if application logic had a bug. There is no
grace period, so clients must serialize refreshes: the web app uses the Web Locks API across tabs, Android
a `Mutex`, iOS a single shared refresh `Task`.

### 4.4 Web: cookies and CSRF
| Cookie | Content | Attributes |
|--------|---------|------------|
| `__Host-kk_access` | access JWT | HttpOnly, Secure, SameSite=Lax, Path=/ |
| `__Secure-kk_refresh` | refresh token | HttpOnly, Secure, SameSite=Strict, Path=/api/v1/auth/web/refresh |
| `__Host-kk_csrf` | CSRF token | Secure, SameSite=Strict, Path=/ (readable by JavaScript) |

CSRF uses the **signed double-submit cookie** pattern: token = `random.HMAC-SHA256(CSRF_SECRET, sid || random)`.
Every unsafe cookie-authenticated request (POST, PUT, PATCH, DELETE) must send `X-CSRF-Token` equal to the
cookie and with a valid HMAC for the session; it is checked in constant time before the handler runs. In
addition the `Origin` header must match `PUBLIC_BASE_URL` (or `CORS_ALLOWED_ORIGINS`); a missing `Origin`
or the value `null` is rejected (`403 csrf_failed`). Login has no session yet, so `POST /auth/web/login`
relies on the `Origin` check plus SameSite. JavaScript never sees access or
refresh tokens. In development without TLS (`AUTH_COOKIE_SECURE=false`) the `__Host-`/`__Secure-` prefixes are
dropped because browsers require `Secure` for them.

### 4.5 Native apps
`POST /auth/native/login` and `POST /auth/native/refresh` return tokens in the body. Access token in
`Authorization: Bearer`; refresh token only in the request body, never in a URL. Tokens are stored in the
Android Keystore (AES-GCM, ciphertext in DataStore) and the iOS Keychain
(`kSecAttrAccessibleWhenUnlockedThisDeviceOnly`). Certificate pinning is not used, because every
self-hosted instance has its own certificate; the platform trust store and ATS / network security config apply.

### 4.6 Credentials per request
A request may carry exactly one of: API key, bearer token, auth cookie. More than one → `400
ambiguous_credentials`. This prevents confused-deputy mixes such as a cookie-authenticated request
borrowing an API key.

### 4.7 API keys (server-to-server)
Format `kk_<prefix>_<secret>`: `prefix` = 12 random base32 characters (public, unique, used for lookup),
`secret` = 32 random bytes base64url. The base64url secret may itself contain `_`, so the key is split into
at most three parts at the first two underscores. The full key is shown once; only `SHA-256(full key)` is
stored and compared in constant time. Keys have scopes, a mandatory expiry (≤ 730 days), revocation, a per-key rate
limit and batched usage counters; creation, revocation and authentication failures are audit-logged.
- **Dormitory keys** (created by dormitory administrators) act on exactly one dormitory. Scopes:
  `menus:read`, `menus:write`, `stats:read`, `announcements:write`, `dormitories:read`.
- **Instance service keys** (created by global administrators, `dormitory_id` NULL) may only read the
  public dormitory directory (`dormitories:read`); a database check enforces this.
An API key never reaches `/me`, `/admin`, ratings, comments, photos, reports or member data, and on
dormitory routes the key's dormitory must equal the path dormitory, otherwise `404`. Keys never go into
the web, Android or iOS apps. Meal objects returned to keys contain only aggregate statistics; the
per-user fields are null.

**Credentials do not outlive their creator's authority.** When a user stops being an administrator of a
dormitory (demotion, removal, ban, leaving, account deletion, dormitory deletion), the API keys and
invitation codes that user created for that dormitory are revoked in the same transaction, audit-logged,
and the remaining administrators can issue new ones. Instance service keys are revoked when their creator
loses global administrator rights.

### 4.8 Password reset, account deletion, first administrator
- There is no e-mail delivery in this version. A global administrator issues a one-time reset token
  (`POST /admin/users/{id}/password-reset-tokens`, 32 random bytes, 1 hour, shown once, hash stored) and hands
  it over out of band; `POST /auth/password-reset` sets the new password and revokes all sessions.
  Restrictions: no tokens for oneself or for other global administrators (their recovery goes through the
  operator CLI); the account owner gets an `account.password_reset_issued` notification; the action is
  audit-logged; outstanding tokens are consumed when the password changes or the account is deleted.
  A global administrator can still take over an ordinary account this way; that power is inherent to the
  feature and is made visible rather than prevented (operators also control the database).
- Account deletion requires the password, deletes all sessions and refresh tokens (including user agent and
  network), consumes reset tokens, ends memberships, removes avatar and photos
  (objects deleted), clears comment texts, deletes ratings, recommendations and notifications, and scrubs
  e-mail, password hash and display name. The `users` row stays as a tombstone so foreign keys from reports,
  sanctions and audit rows remain valid; it no longer contains personal data. The only administrator of a
  dormitory with other members must hand over the role first (`409 last_admin`).
- The first global administrator is created by the operator:
  `docker compose exec api /app/admin promote --email <address>` (the account must already be registered).

## 5. Authorization and tenant isolation

### 5.1 Principals
`User` (from a verified JWT and an active session), `ApiKey` (from a verified key), or anonymous. The
principal is put into the request context by middleware; the user ID never comes from a request body,
header or query string.

### 5.2 Roles and permissions
Dormitory roles live in `dormitory_members.role` (`member` < `moderator` < `admin`). Global administrators
are flagged in `users.is_global_admin`.

| Action | member | moderator | admin | global admin (`/admin/*`) | API key scope |
|--------|:------:|:---------:|:-----:|:-------------------------:|---------------|
| Read published menus, meals, statistics, comments, photos | ✓ | ✓ | ✓ | – | `menus:read`, `stats:read` (menus and statistics only) |
| Rate, recommend, comment, upload photos, report | ✓ | ✓ | ✓ | – | – |
| Edit or delete own content | ✓ | ✓ | ✓ | – | – |
| Manage menus, meals, categories; see drafts | – | ✓ | ✓ | – | `menus:write` |
| Publish announcements | – | ✓ | ✓ | – | `announcements:write` |
| Review reports, remove content, escalate | – | ✓ | ✓ | escalated reports | – |
| Suspend or ban | – | members | members, moderators | instance-wide | – |
| Trusted reviewer flag, member list | – | ✓ | ✓ | – | – |
| Roles, invitations, settings, API keys, audit log, delete dormitory | – | – | ✓ | delete, restore, assign admin | – |
| Create a dormitory | `DORMITORY_CREATION=all_users` | | | ✓ | – |

Nobody can act on a user with an equal or higher role, change their own role or trusted reviewer flag,
sanction themselves, or see or handle a report about themselves (§10).

### 5.3 Enforcement layers
1. **Route**: every tenant resource is under `/dormitories/{dormitoryId}/…`, or the dormitory is derived from
   the owning resource on the server.
2. **Service**: `authz.RequireMembership(ctx, principal, dormitoryID, minRole)` runs
   `SELECT role, is_trusted_reviewer FROM dormitory_members m JOIN dormitories d ON d.id = m.dormitory_id
   WHERE m.dormitory_id = $1 AND m.user_id = $2 AND m.left_at IS NULL AND d.deleted_at IS NULL`
   (index-only via `dormitory_members_active_key`). Write operations also check active sanctions.
3. **Query scoping**: every read and write includes the dormitory, e.g.
   `WHERE id = $1 AND dormitory_id = $2 AND deleted_at IS NULL`. `WHERE id = $1` alone is never used on
   tenant tables. Reads of meals and their children (ratings, recommendations, comments, photos, meal
   statistics) also join `menus` and require `menus.deleted_at IS NULL` and, for callers who may not see
   drafts, `menus.status = 'published'`; otherwise `404`. Client-supplied IDs in bodies (`category_id`,
   `target_id`, `user_id`, `report_id`) are resolved with the same dormitory condition.
4. **Database backstop**: child tables reference their parent with composite foreign keys
   `(parent_id, dormitory_id)` (meals → menus/categories, ratings/comments/photos/recommendations → meals,
   reports → comments/photos, sanctions → reports, members → invitations). A bug cannot attach a rating to a
   meal of another dormitory; the migration tests prove these rejections (§18).
5. **Audit**: moderator and administrator actions write `audit_logs` rows in the same transaction.

**404 versus 403**: a caller who is not an active member gets `404` for anything inside a dormitory, so
existence is not revealed. A member lacking the role gets `403`. Private dormitories return `404` to
non-members even on `GET /dormitories/{id}`.

### 5.4 Sanctions
`user_sanctions` rows are keyed by user and dormitory (not by membership), so leaving and rejoining does not
lift them. Dormitory `suspension` = read-only in that dormitory; dormitory `ban` = membership ended and
rejoining blocked; instance `suspension` = read-only everywhere; instance `ban` = login refused and sessions
revoked. Active means `revoked_at IS NULL AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())`.

### 5.5 Global administrators
They are not implicit members of any dormitory and cannot read dormitory content through dormitory routes.
They act only through `/admin/*`: user search, instance-wide sanctions, password reset tokens, dormitory
list (metadata and counts), delete/restore dormitories, assign a dormitory administrator, escalated reports,
instance API keys, instance audit log and statistics. Every `/admin/*` write is audit-logged.

Assigning a dormitory administrator is limited so that it cannot become a back door into private
dormitories: the target must already be an active, unbanned member; the caller cannot assign themselves;
no membership is ever created; a reason is required and is sent, with the assignment, to the promoted user
and to all current administrators and moderators of the dormitory.

### 5.6 Authorization tests (phase 2)
`apps/api/tests/authz/` holds table-driven tests for every dormitory-scoped operation, following the
`authz-tenant-isolation` skill: anonymous → 401; non-member → 404; member of dormitory B using A's IDs → 404;
member below the required role → 403; required role → 2xx; forged `role`/`dormitory_id` in body or query →
ignored. The `x-required-role` and `x-api-key-scopes` extensions in the contract are the expected values,
so the test tables can be checked against the contract automatically.

## 6. Data model

### 6.1 Conventions
- PostgreSQL ≥ 15 (`ON DELETE SET NULL (column)` is used); target and tested version 17.
- No extensions. Primary keys `uuid` from Go. `created_at timestamptz NOT NULL DEFAULT now()`;
  `updated_at` maintained by the application.
- Enumerations are `text` with named `CHECK` constraints (easier to evolve than PostgreSQL enums).
- Every constraint has a stable name (`<table>_<what>_check|key|fkey|idx`) so the repo layer can map
  violations (23505, 23503, 23514) to domain errors.
- Secrets are stored only as hashes in columns named `*_hash`.
- `users`, `dormitories` and `api_keys` are never hard-deleted (tombstone, soft delete, revocation), so
  foreign keys to them use `RESTRICT`; `audit_logs` stays append-only.
- `*_by` actor columns (`created_by`, `revoked_by`, `deleted_by`, `resolved_by`, …) are not indexed: no query
  filters on them and the referenced rows are never deleted, so no foreign-key check needs the index.

### 6.2 Tables
| Table | Migration | Purpose | Tenant key | Deletion | Notable constraints |
|-------|-----------|---------|------------|----------|---------------------|
| `users` | 0001 | accounts | – | tombstone (scrubbed, `deleted_at`) | unique `lower(email)`; only `$argon2id$` hashes; deleted rows hold no personal data |
| `user_sessions` | 0002 | session = refresh-token family | via user | `revoked_at` + purge | revoke reason enum; truncated `ip_network` |
| `refresh_tokens` | 0002 | rotating refresh tokens | via session | purge | unique 32-byte hash; ≤ 1 unused per session |
| `password_reset_tokens` | 0002 | admin-issued one-time tokens | via user | `consumed_at` | ≤ 1 outstanding per user |
| `dormitories` | 0003 | tenants and settings | itself | soft | unique slug; private ⇒ invite-only |
| `invitations` | 0004 | invitation codes (hash + hint) | `dormitory_id` | `revoked_at` | unique hash; `use_count ≤ max_uses` |
| `dormitory_members` | 0005 | membership history and roles | `dormitory_id` | `left_at` | one active row per (dormitory, user); invitation of same dormitory |
| `meal_categories` | 0006 | dormitory-defined categories | `dormitory_id` | soft | unique name per dormitory |
| `menus` | 0006 | menu of one date and meal period | `dormitory_id` | soft | one per (dormitory, date, period); published ⇔ `published_at` |
| `meals` | 0007 | meals of a menu | `dormitory_id` | soft | composite FKs to menu and category; allergen codes from a fixed list |
| `ratings` | 0008 | 1–5 ratings | `dormitory_id` | soft | one active per (meal, user); `value BETWEEN 1 AND 5` |
| `comments` | 0009 | comments | `dormitory_id` | soft; text cleared on author/account deletion | composite FK to meal |
| `meal_photos` | 0010 | meal photo metadata | `dormitory_id` | status `removed` | composite FK to meal; status machine checks |
| `user_avatars` | 0010 | profile pictures | via user | status `removed` | one `ready` per user |
| `recommendations` | 0011 | recommended / not recommended | `dormitory_id` | hard delete | one per (meal, user) |
| `announcements` | 0012 | dormitory announcements | `dormitory_id` | soft | priority enum |
| `notifications` | 0012 | in-app notifications | user (+ optional `dormitory_id`) | hard | one unread row per grouping key |
| `reports` | 0013 | reports of comments, photos, users | `dormitory_id` | never (status) | target in same dormitory; `subject_user_id` (who the report is about) ≠ reporter; one open report per reporter and target |
| `user_sanctions` | 0013 | suspensions and bans | `dormitory_id` or NULL (instance) | `revoked_at` | cited report in same dormitory and about the sanctioned user |
| `api_keys` | 0014 | server-to-server keys | `dormitory_id` or NULL (instance) | `revoked_at` | unique prefix and hash; instance keys only `dormitories:read` |
| `audit_logs` | 0015 | append-only audit trail | `dormitory_id` (optional) | retention purge only | triggers reject UPDATE, TRUNCATE and DELETE of rows younger than 30 days |

The 15 tables required by SPEC §6 are all present; the six additional tables (`refresh_tokens`,
`password_reset_tokens`, `meal_categories`, `user_avatars`, `announcements`, `user_sanctions`) are recorded
in `docs/DECISIONS.md`. Detailed column comments are in the migration files.

### 6.3 Aggregates
Average, count and distribution of ratings and recommendation counts are computed on read. The unique
indexes include the value (`INCLUDE (value)`), so these are index-only scans over at most a few hundred rows
per meal. Counter tables can be added later if measurements require them.

### 6.4 Retention and purge
Soft-deleted content is invisible immediately and purged after `SOFT_DELETE_RETENTION_DAYS`. Reports keep a
`content_snapshot`; their foreign keys to purged content become NULL (`ON DELETE SET NULL (column)`), and a
check constraint prevents purging content that an **open** report still points at. Image objects are deleted
from storage as soon as a photo is removed. Audit logs are kept `AUDIT_LOG_RETENTION_DAYS`.

## 7. Menus, ratings and recommendations
- A menu is one meal period (`breakfast`, `lunch`, `dinner`, `all_day`) on one date. The weekly menu is the
  seven days of an ISO week; `GET …/menus/today` and `GET …/menus/week` compute "today" in the dormitory's
  time zone. Ranges are limited to 31 days.
- Only published menus are visible to members; moderators and `menus:write` keys can include drafts.
- Ratings, recommendations and photo uploads are accepted only when the menu is published, the menu date is
  today or earlier in the dormitory's time zone, and (if `rating_window_days` is set) the window has not
  closed. Otherwise `409 rating_closed`. Comments only require a published menu.
- `PUT …/my-rating` upserts with `INSERT … ON CONFLICT (meal_id, user_id) WHERE deleted_at IS NULL DO UPDATE`.
- Individual ratings are private: other users see only aggregates (privacy and less harassment). Each user
  sees their own rating history. Because trusted reviewers are known to members, the trusted reviewers'
  average is returned only when at least 3 trusted reviewers rated the meal.
- Allergens: codes of the 14 groups in Annex II of Regulation (EU) No 1169/2011. `NULL` = not provided,
  `[]` = declared none. All clients state that allergen data comes from the dormitory and must be verified
  with the food provider (DISCLAIMER, phase 7).

## 8. Photo pipeline
1. `POST …/meals/{id}/photos` with `content_type` and `byte_size`: the API checks membership, sanctions,
   availability and limits, inserts `meal_photos` (`pending_upload`) and returns a **presigned POST policy**
   for `incoming/meal-photos/<photoId>`, restricted by policy conditions to that key, the declared content type
   and `content-length-range` up to the declared size, valid 5 minutes. The client never receives storage
   credentials; the bucket is private.
2. The client uploads directly to object storage through `S3_PUBLIC_URL` (multipart form POST). The policy
   sets `success_action_status=204`, so storage returns no body.
   **Storage route hardening.** By default the storage endpoint shares the application origin
   (`/storage/<bucket>`), where the JavaScript-readable CSRF cookie lives, so the reverse proxy exposes as
   little as possible: only `POST` to exactly `/storage/<bucket>` is forwarded (every other method and path
   returns 404, so no object reads, listings or MinIO admin paths); `Cookie` and `Authorization` request
   headers are removed before forwarding; responses get `Content-Security-Policy: sandbox; default-src 'none'`,
   `X-Content-Type-Options: nosniff` and `Content-Disposition: attachment`. Operators who can run a second host
   name should set `S3_PUBLIC_URL` to a separate storage origin instead (documented in phase 3).
3. `POST …/photos/{id}/complete`: the API reads the object with a single size-limited GET (no separate
   HEAD, so there is no check-then-use gap), re-checks the size, verifies the magic bytes
   (JPEG, PNG, WebP), reads the header to reject images over 40 megapixels or 10 000 px per side before full
   decoding (decompression-bomb protection), decodes, applies the EXIF orientation, and **re-encodes** to JPEG
   (quality 82) at `thumb` 320 px, `medium` 1024 px and `large` 2048 px (longest edge). Re-encoding drops all
   metadata (EXIF, GPS, XMP). Variants are written to `meal-photos/<dormitoryId>/<photoId>/<variant>.jpg`; the
   incoming object is deleted. Concurrent processing is bounded by a semaphore.
4. Images are served by the API (`GET …/photos/{id}/images/{variant}`) after the membership check, with
   `Cache-Control: private, max-age=31536000, immutable` and an ETag. No public or presigned GET URLs are used,
   so photos of private dormitories never leak through shareable links.
Avatars use the same pipeline (`incoming/avatars/<id>`, square crops `small` 128 px and `medium` 512 px) and are
visible to users who share an active dormitory with the owner. Object keys are always derived from server IDs.
When a photo with an open report is deleted by its author, it disappears for members at once, but its
`large` variant is kept as evidence until the report is resolved and is deleted afterwards.
A bucket lifecycle rule expires `incoming/` after one day as a backstop to the cleanup job.
The iOS and Android apps downscale and convert to JPEG before upload (HEIC is not accepted by the server).

## 9. Notifications and announcements
- In-app only. Kinds: `comment.created` (earlier commenters of the same meal, grouped per meal while unread),
  `announcement.published` (priority `important`, every active member with announcement notifications on),
  `moderation.*` (content removed, sanction applied or lifted, report resolved) and `membership.role_changed`.
- Notifications are written in the same transaction as the event. Fan-out uses one `INSERT … SELECT`.
- Dormitory notifications are listed only while the user is an active member and are deleted on leaving;
  moderation notifications about the user's own account have `dormitory_id = NULL`.
- **Extension point**: services call a `notifications.Dispatcher` interface; the in-app channel is the only
  implementation. A push channel (and a `push_subscriptions` table) can be added later without changing callers.

## 10. Moderation, reputation and audit
- Members report comments, photos or users of their dormitory; one open report per reporter and target.
  Each report stores its **subject** (the reported user, or the author of the reported content). Routing
  by the subject's current role prevents anyone from handling a report about themselves:
  subject `member` → moderators and administrators; subject `moderator` → administrators only; subject
  `admin` → escalated to the global administrators when filed. The subject never sees the report or the
  reporter's identity.
- Handlers work the queue (oldest first), resolve with `content_removed`, `user_sanctioned`, `no_action` or
  `dismissed` (removal, sanction, notifications and audit row in one transaction), or escalate to global
  administrators. A sanction that cites a report must target the report's subject (database-enforced).
- **Reputation** (per dormitory, computed on read): `ratings + recommendations + 2 × comments + 3 × photos −
  10 × contributions removed by moderators`, minimum 0. Levels: `newcomer` < 10 ≤ `regular` < 50 ≤ `active`
  < 200 ≤ `veteran`.
- **Trusted reviewer** is a per-dormitory flag set by moderators (suggested for `active` and `veteran`
  members). Meal statistics show the trusted reviewers' average separately.
- **Audit log** actions use dotted names (`auth.login_failed`, `auth.refresh_token_reused`, `comment.removed`,
  `member.role_changed`, `sanction.created`, `api_key.created`, `dormitory.deleted`, …). Metadata never contains
  passwords, tokens, keys, cookies or e-mail addresses. IP addresses are stored truncated.

## 11. Rate limits, request limits and security headers

In-memory token buckets (single API instance; a shared store would be needed for horizontal scaling and is
not planned). Keys: user ID when authenticated, API key ID for keys, otherwise the client IP. Because all
traffic arrives through the reverse proxy, the client IP is taken from `X-Forwarded-For` only when the direct
peer is in `TRUSTED_PROXIES`, using the right-most address that is not a trusted proxy; Caddy replaces
(not appends to) client-supplied `X-Forwarded-For`. With an empty `TRUSTED_PROXIES` every client would share
one bucket, so Compose always sets it.

| Bucket (`x-rate-limit`) | Key | Default limit |
|-------------------------|-----|---------------|
| `login` | IP + e-mail, and IP | 5/min per IP+e-mail, 20/min per IP; exponential back-off per account **and IP** after 5 failures (max 15 min), so an attacker cannot lock a victim out from other networks; an additional per-account ceiling of 100 failures/hour across all IPs slows distributed guessing |
| `register` | IP | 5/hour |
| `refresh` | session (or IP) | 30/min |
| `read` | user, key or IP | 300/min |
| `write` | user or key | 60/min |
| `sensitive` | user | 10/hour (password, e-mail, deletion, API keys, admin writes) |
| `upload` | user | 30/hour |
| `image` | user | 600/min |
| `invitation` | user and IP | 10/hour (protects 80-bit codes against guessing) |
| `export` | user | 5/hour |
| per API key | key | `rate_limit_per_minute` (default 60) in addition to the bucket above |

Responses carry `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`; `429` adds `Retry-After`.

Request limits: JSON bodies 256 KiB (`http.MaxBytesReader`), server read/write timeouts, per-request
context timeout, DB statement timeout. Uploads never pass through the API.

Security headers (API): `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`,
`X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Cross-Origin-Resource-Policy: same-origin`,
`Cache-Control: no-store` (except image variants), `Strict-Transport-Security` when served over TLS.
Web (served by Caddy): CSP `default-src 'self'; img-src 'self' blob: data:; connect-src 'self';
frame-ancestors 'none'; base-uri 'self'; form-action 'self'`, `X-Frame-Options: DENY`,
`Permissions-Policy` (camera only for self), `Cross-Origin-Opener-Policy: same-origin`.
CORS is off unless `CORS_ALLOWED_ORIGINS` lists origins; native apps do not need CORS.

## 12. API contract and client generation
- Source of truth: `docs/api/openapi.yaml` (OpenAPI 3.1, 112 operations). Lint:
  `npx @redocly/cli lint docs/api/openapi.yaml` with `redocly.yaml`.
- Custom extensions: `x-required-role` (minimum role), `x-api-key-scopes` (scopes that unlock the operation),
  `x-rate-limit` (bucket). Security requirements model CSRF precisely: unsafe cookie operations require
  `cookieAuth` **and** `csrfToken`.
- Lists: `{ "data": [...], "next_cursor": "…" | null }`. Cursors are opaque base64url-encoded sort keys;
  they only move within the caller's already-authorized result set, so they are not signed.
- Client generation (`scripts/gen-clients.sh`, phases 4–6; generated files are committed and never edited):
  TypeScript `openapi-typescript` + `openapi-fetch` → `apps/web/src/api/generated/`;
  Kotlin `openapi-generator` (`kotlin`, `jvm-retrofit2`, `kotlinx_serialization`) →
  `apps/android/.../core/network/generated/`; Swift `openapi-generator` (`swift5`, as the `api-contract` skill
  says; Apple's `swift-openapi-generator` will be evaluated in phase 6) → `apps/ios/KoliKaja/Core/Network/Generated/`.
- Compatibility: `oasdiff breaking` against the previous released contract; breaking changes need `/api/v2`.

## 13. Deployment topology (phase 3)
Compose services: `postgres` (17-alpine, volume, `pg_isready`), `minio` (volume, health check; see the open
question in §18), `minio-init` (one-shot: bucket, lifecycle rule, least-privilege user and policy), `migrate`
(one-shot, API image, `/app/migrate up`), `api` (depends on healthy postgres and minio and on migrate
having completed; distroless static `nonroot` image, read-only root filesystem, `HEALTHCHECK` via
`/app/api healthcheck`), `web` (Caddy serving the static build and proxying `/api`, `/healthz`, `/readyz`,
`/storage`; non-root, port 8080). Networks: `backend` (internal: postgres, minio, api, migrate) and `frontend`
(web, api) with a fixed subnet that Compose passes to the API as `TRUSTED_PROXIES`. Only `web` publishes a
port. Caddy overwrites `X-Forwarded-For` and applies the `/storage` restrictions of §8. Every required secret uses `${VAR:?message}` so Compose stops with a
clear message; `deploy/scripts/gen-secrets.sh` fills `.env` with random values. Backup: `pg_dump --format=custom`
plus `mc mirror`; restore script with confirmation; documented test restore.
The database uses one role owned by the application in this version; splitting into a migration owner and
a DML-only runtime role is a documented hardening option for phase 3.

## 14. Clients
- **Web**: React, TypeScript (strict), Vite, Tailwind, React Router, TanStack Query, i18next (Hungarian
  default, English), Radix UI primitives, `vite-plugin-pwa` (app shell cached, API responses not cached;
  offline view shows last data with a stale marker). Auth via cookies only; one refresh in flight across tabs
  (Web Locks API). Dark mode default.
- **Android**: Kotlin, Compose, Material 3, Hilt, Navigation Compose, Retrofit + OkHttp + kotlinx.serialization,
  Coil, DataStore + Keystore AES-GCM for tokens, system photo picker. minSdk 26. Strings in `values/`
  (English) and `values-hu/` (Hungarian). Application ID `io.github.madarlevi.kolikaja` (see §18).
- **iOS**: Swift, SwiftUI, minimum iOS 17 so view models use the Observation framework (`@Observable`);
  `URLSession` async/await, an actor-isolated API client with a single refresh task, Keychain storage,
  `PhotosPicker`, `Localizable.xcstrings` (Hungarian and English).
- All clients: the server URL is configurable (any compatible self-hosted instance), an About / Legal screen
  shows the copyright, the AGPL notice, `source_code_url` from `GET /meta` and a link to `TRADEMARKS.md`.

## 15. Logging, privacy and telemetry
- `log/slog` JSON logs with request ID, user ID (when known), route, status and duration. Redacted:
  `Authorization`, `Cookie`, `Set-Cookie`, `X-API-Key`, `X-CSRF-Token`, request bodies of auth endpoints,
  passwords, tokens, e-mail addresses in error paths.
- No telemetry, analytics or third-party calls of any kind. The web app loads no external resources.
- Personal data categories and retention will be described in `PRIVACY.md` (phase 7); operators complete it.

## 16. Licensing notes
The project is licensed AGPL-3.0-only (DECISIONS D-04). Because users interact with the software over a
network, AGPL-3.0 section 13 requires that users of a **modified** version are offered its source. The design
supports this: `GET /meta` returns `source_code_url` (configurable with `SOURCE_CODE_URL`), and every client
shows it in the About / Legal screen. Operators who modify the code must point it at their modified source.
This is a technical aid, not legal advice; the legal documents are written in phase 7.

## 17. Extension points (out of scope for this version)
PDF import, OCR and menu scraping: a future importer produces the same `CreateMenuRequest` payloads through a
`menus:write` API key, so no schema change is needed. Push notifications: new `notifications.Dispatcher`
channel. AI recommendations, n8n and external automation: separate services consuming the API with scoped keys.
Payments and subscriptions: not designed for. E-mail (verification, password reset): optional SMTP channel
behind the same dispatcher interface.

## 18. Verification status and open questions
Phase 1 verification (details in `docs/PROGRESS.md`):
- Migrations: applied to a real PostgreSQL 17.5 (embedded binaries) with golang-migrate v4.20.1: up, down 1,
  up, all steps down, step-wise up, identical schema fingerprints (10 checks), and 42 constraint tests
  (duplicate rating, cross-tenant inserts, report subject rules, sanction backstops, append-only audit log,
  scrubbed deleted accounts, …). PostgreSQL in Docker Compose: not verified.
- Security review: the `security-reviewer` subagent reviewed this design; its seven medium findings and the
  low findings are addressed in this document, the migrations and the contract (see `docs/PROGRESS.md`).
- Contract: `@redocly/cli` 2.84.0 lint passes (two documented ignores for health probes); project-rule checks
  (CSRF on unsafe cookie operations, scopes, 401/403/404 presence, bounded request schemas) pass.

Open questions (also listed in `docs/PROGRESS.md`):
1. **MinIO distribution**: the availability and license terms of maintained MinIO server images must be checked
   in phase 3. The API uses only the S3 API, so another S3-compatible server can replace MinIO if needed.
2. **Android application ID** `io.github.madarlevi.kolikaja` is derived from the GitHub account in SPEC §11 so no
   domain is invented. It becomes permanent after the first store release; the maintainer should confirm it.
3. Presigned POST through the `/storage` path of the reverse proxy must be verified at runtime (phase 3).
