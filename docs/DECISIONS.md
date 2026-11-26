# KoliKaja – decision log

These decisions override `docs/SPEC.md` where they conflict. Add new decisions at the bottom with a date and a reason.

## Confirmed by the maintainer

| # | Topic | Decision | Source |
|---|-------|----------|--------|
| D-01 | Docker | Not available in the current working environment. Runtime Docker steps are recorded as "not verified". | Maintainer answer |
| D-02 | Mobile builds | No Android SDK or macOS/Xcode available yet. Android and iOS code is written to be buildable, but builds are not verified until the maintainer lists the required tools in `docs/ENVIRONMENT.md`. | Maintainer answer |
| D-03 | UI language | Hungarian is the default language. English is supported through i18n resource files in every client (web, Android, iOS). No hardcoded user-facing strings in components. | Maintainer answer |
| D-04 | License version | GNU AGPL-3.0 (single version, `AGPL-3.0-only`) for now. Replace `AGPL-3.0-only` with `AGPL-3.0-or-later` in all files only if the maintainer asks. | Maintainer answer |
| D-05 | Relicensing right | The copyright holder (madarlevi) must be able to change the project license later. This is stated in `COPYRIGHT` and `CONTRIBUTING.md`, **not** inside `LICENSE`, because `LICENSE` must stay the unmodified official text. See the open question below. | Maintainer answer |

## Open question (needs legal review, not a technical choice)
- D-05 is only fully effective for code that madarlevi owns. Contributions from others remain under the
  inbound terms in `CONTRIBUTING.md`. A relicensing right for contributions requires either a contributor
  license grant or a CLA, which the project spec forbids unless justified and documented. Until this is
  decided, `CONTRIBUTING.md` states that contributors keep their copyright and license their contributions
  under the project license, and that the maintainer can relicense only code they own. This is not legal advice.

## Technical decisions (made by the agent, to be reviewed)
- Language of code, comments and API identifiers: English.
- API version prefix: `/api/v1/`.
- Web authentication: HttpOnly, Secure, SameSite cookies with CSRF double-submit token.
- Mobile authentication: short-lived bearer access token plus rotating refresh token, both stored in Keystore / Keychain.
- Refresh tokens are opaque random values, stored only as SHA-256 hashes, rotated on every use.
- Password hashing: Argon2id, parameters defined in `apps/api/internal/auth/argon2.go` and documented in `ARCHITECTURE.md`.
- Photos: presigned upload to MinIO, validated by magic bytes, size-limited, metadata stripped (EXIF) before serving.
- Migrations: golang-migrate, files named `NNNN_description.up.sql` / `NNNN_description.down.sql`.

## Phase 1 technical decisions (2026-11-26, made by the agent, to be reviewed)
Details are in `docs/ARCHITECTURE.md`; the section is given in brackets.

| # | Topic | Decision | Reason |
|---|-------|----------|--------|
| T-01 | IDs | UUIDv7 generated in Go (`github.com/google/uuid`). [3.7] | Time-ordered keys keep indexes compact and make `(created_at, id)` cursors cheap. |
| T-02 | Access tokens | JWT HS256 with `JWT_SECRET` (optional `JWT_SECRET_PREVIOUS` for rotation), 10 minutes; the session row is checked on every authenticated request. [4.2] | Only the API signs and verifies, so one symmetric secret suffices; the session check makes logout and bans immediate. |
| T-03 | Refresh tokens | 30-day idle lifetime, 90-day absolute session lifetime, no reuse grace period; clients serialize refreshes. [4.3] | CLAUDE.md requires that reuse revokes the family; a grace period would weaken that. |
| T-04 | Web cookies and CSRF | `__Host-kk_access`, `__Secure-kk_refresh` (path `/api/v1/auth/web/refresh`), `__Host-kk_csrf`; signed double-submit token bound to the session with its own `CSRF_SECRET`; `Origin` check on all unsafe cookie requests including login. [4.4] | OWASP-recommended variant that also resists cookie injection from sibling subdomains. |
| T-05 | Auth endpoints | Separate `/auth/web/*` and `/auth/native/*` login and refresh; registration does not log in. [4] | Different token delivery per client type gives clean response schemas; one login path per client. |
| T-06 | Credentials | Exactly one credential type per request, otherwise `400 ambiguous_credentials`. [4.6] | Prevents confused-deputy mixes of cookie, bearer and API key. |
| T-07 | Argon2id | m = 19 MiB, t = 2, p = 1, at most 4 hashes in parallel. [4.1] | OWASP minimum, bounded memory for small self-hosted servers. |
| T-08 | Password reset | No e-mail in this version. A global administrator issues a one-time reset token (32 random bytes, 1 hour) and hands it over out of band; never for themselves or other global administrators; the owner is notified; tokens are consumed on password change and account deletion. E-mail verification is not implemented. [4.8] | SPEC forbids mandatory external services; without any reset path a forgotten password locks the user out permanently. Restrictions follow the phase 1 security review. |
| T-09 | Global administrators | Not implicit members of dormitories; they act only through audited `/admin/*` operations. They can promote an existing, unbanned member (never themselves, never creating a membership) to dormitory administrator, with a reason that is sent to the dormitory's staff. The first global administrator is promoted with `cmd/admin`. [5.5] | Keeps the "membership required" rule for all dormitory data paths and prevents the assignment from becoming a back door into private dormitories (security review). |
| T-10 | Dormitory creation | Default `DORMITORY_CREATION=global_admins`; `all_users` is optional. [3.4] | Prevents spam dormitories on public instances; single-dormitory operators are global administrators anyway. |
| T-11 | Roles | `member` < `moderator` < `admin` (dormitory administrator). Moderators manage menus, announcements and moderation; administrators manage roles, invitations, settings, API keys, audit log and deletion. [5.2] | Lets administrators delegate menu entry without giving away full control. |
| T-12 | Extra tables | `refresh_tokens`, `password_reset_tokens`, `meal_categories`, `user_avatars`, `announcements`, `user_sanctions`, in addition to the 15 tables of SPEC §6, grouped into 15 migrations by module. [6.2] | Needed for rotation history, T-08, dormitory-defined categories, profile pictures, "important announcements" and suspensions that survive leave/rejoin. |
| T-13 | Menus | One menu per dormitory, date and meal period; the weekly menu is the ISO week of daily menus. Ratings, recommendations and photos only for published menus dated today or earlier (dormitory time zone) and within `rating_window_days`. [7] | Matches how dormitory canteens publish menus; prevents rating meals before they are served. |
| T-14 | Rating privacy | Individual ratings are visible only to their author; others see aggregates. Trusted reviewer is a per-dormitory flag set by moderators; reputation is computed on read. [7], [10] | Data minimization and less harassment; no extra tables. |
| T-15 | Photos | Presigned **POST** policy (enforces size and type) for uploads; server re-encodes to JPEG without metadata; images are served by the API after the membership check, never by public or presigned GET URLs; HEIC is converted by the apps. [8] | Presigned PUT cannot enforce a size limit; capability URLs would leak photos of private dormitories. |
| T-16 | API keys | `kk_<prefix>_<secret>`, SHA-256 stored, mandatory expiry ≤ 730 days. Dormitory keys or instance service keys; instance keys may only use `dormitories:read` (DB check). No key reaches personal data, `/me` or `/admin`. Keys and invitation codes are revoked automatically when their creator loses the administrator role. [4.7] | SPEC "dormitory or service account" without letting any key bypass tenant isolation; credentials must not outlive the authority that created them (security review). |
| T-17 | Lint exception | `/healthz` and `/readyz` have no 4XX responses; `operation-4xx-response` is ignored for them in `.redocly.lint-ignore.yaml`. | Health probes have no client errors; an invented 4XX response would be inaccurate. |
| T-18 | Invitations | 16-character Crockford base32 codes (80 bits), hash and 4-character hint stored, shown once, always grant `member`, max 180 days, strict rate limit. [4], [11] | Codes are shared widely; hashing limits damage from a database leak; role changes stay explicit. |
| T-19 | Account deletion | Tombstone `users` row without personal data; ratings, recommendations, notifications deleted; comment texts cleared; photos and avatar deleted. [4.8] | Erases personal data while keeping foreign keys of moderation and audit rows valid. |
| T-20 | Schema conventions | `text` + named `CHECK` instead of PostgreSQL enums; no extensions; PostgreSQL ≥ 15 (tested 17.5); `*_by` actor columns unindexed. [6.1] | Easier evolution and error mapping; `ON DELETE SET NULL (column)` needs 15+; actor columns are never filtered and their targets are never deleted. |
| T-21 | Audit log | Triggers reject UPDATE, TRUNCATE and DELETE of rows younger than 30 days; foreign keys `RESTRICT`; `AUDIT_LOG_RETENTION_DAYS` ≥ 30. [6.1] | Tamper resistance against application bugs while allowing the retention purge; SET NULL would conflict with the triggers. Not a defence against a database superuser. |
| T-22 | Validation errors | `400` for malformed requests (including unknown fields), `422` for rule violations with field details. [3.5] | Clients can map `422` details to form fields. |
| T-23 | Rate limiting | In-memory token buckets in the single API process; buckets and limits in [11]. | No extra service (Redis) for a single-instance deployment. |
| T-24 | Clients | Android: Retrofit + kotlinx.serialization, minSdk 26, application ID `io.github.madarlevi.kolikaja` (maintainer to confirm). iOS: minimum iOS 17, `@Observable`. Web: Radix UI primitives, `openapi-typescript` + `openapi-fetch`. No certificate pinning. [14] | Application ID avoids inventing a domain; iOS 17 enables Observation; pinning cannot work across self-hosted instances. |
| T-25 | Deployment | Same-origin deployment behind Caddy (`/`, `/api`, `/storage`); one PostgreSQL role in this version. [13] | Same origin removes CORS and simplifies cookies and CSP; role split is a documented hardening option. |
| T-26 | Line endings | `.gitattributes` forces LF (`* text=auto eol=lf`). | The maintainer's Git uses `core.autocrlf=true`; CRLF breaks shell scripts and SQL inside Linux containers. |
| T-27 | Accepted risk: account enumeration | `register` and `changeEmail` return `409 already_exists` for a registered address, which reveals that it is registered. Mitigated by the `register` and `sensitive` rate limits. | A uniform response would need e-mail delivery ("if the address is new, check your inbox"), which this version does not have. Revisit when an optional SMTP channel is added. |
| T-28 | Storage route | Uploads go to `/storage/<bucket>` on the application origin by default; the proxy forwards only `POST` to that exact path, strips `Cookie` and `Authorization`, and adds `CSP: sandbox`, `nosniff` and `Content-Disposition: attachment`. A separate storage origin is the recommended alternative. [8] | Same-origin storage exposed cookies and a script-capable surface next to the readable CSRF cookie (security review); a second host name is optional so small self-hosters are not forced to manage one. |
| T-29 | Report routing | Reports store their subject (`subject_user_id`). Only members whose role is higher than the subject's see and handle the report; reports about dormitory administrators are escalated to global administrators automatically. [10] | Prevents moderators and administrators from handling or dismissing reports about themselves (security review). |

## Rating labels (exact text, UTF-8, do not change)
1. Le se menj
2. Ha nem vagy éhenhalás szélén, hagyd
3. Ehető, ha nincs más
4. Amúgy eskü nem olyan szar
5. Már határozottan ehetőnek nevezhető
