# KoliKaja – progress

Status legend: `[ ]` not started · `[~]` in progress · `[x]` done and verified · `[!]` done but NOT verified (reason given)

## Phase 1 – Foundation (completed 2026-11-26)
- [x] Inspect repository – no commits and no code existed; only instructions, skills and docs.
  Findings: no `.gitignore` (so `.env` was not ignored), no `.gitattributes` while `core.autocrlf=true`,
  `jq` missing so both hooks are inactive, Docker CLI present but daemon not running. Environment recorded
  in `docs/ENVIRONMENT.md`.
- [x] Write `docs/ARCHITECTURE.md` – 18 sections: layers, configuration, auth and sessions, authorization and
  tenant isolation, data model, photo pipeline, notifications, moderation, rate limits, contract, deployment,
  clients, licensing notes, extension points. 29 technical decisions recorded as T-01…T-29 in `docs/DECISIONS.md`.
- [x] Create folder structure – layout from ARCHITECTURE §2 with `.gitkeep` placeholders; added `.gitignore`,
  `.gitattributes` (LF), `.editorconfig`; hook scripts marked executable.
- [x] Define database schema (migrations 0001–0015, up and down) – 21 tables in 15 migrations
  (SPEC §6 tables + `refresh_tokens`, `password_reset_tokens`, `meal_categories`, `user_avatars`,
  `announcements`, `user_sanctions`; see T-12). Composite foreign keys as tenant backstop.
  Verified on real PostgreSQL 17.5 (embedded binaries) with golang-migrate v4.20.1, see the verification log.
  PostgreSQL inside Docker Compose: not verified (phase 3).
- [x] Define OpenAPI 3.1 contract (`docs/api/openapi.yaml`) – 84 paths, 112 operations, 136 schemas;
  `x-required-role`, `x-api-key-scopes`, `x-rate-limit` on operations. `redocly lint` passes.
  Client regeneration: not applicable yet (no client projects exist; `scripts/gen-clients.sh` is created in
  phases 4–6). Breaking-change check (`oasdiff`): not applicable (first version, tool not installed).
- [x] Legal audit (scoped to phase 1 files) – `docs/LEGAL_AUDIT.md`.
- [x] Security review of the design (`security-reviewer` subagent) – see "Security review" below.

### Security review (2026-11-26, phase 1 design)
The `security-reviewer` subagent reviewed `docs/ARCHITECTURE.md`, all migrations and the full contract.
Verdict before fixes: "blocking findings present" (no critical or high findings; 7 medium, 5 low, several info).
All findings were addressed in the design before committing; the reviewer was not run a second time.

| Severity | Finding | Resolution |
|----------|---------|------------|
| medium | Refresh did not check refresh-token expiry or session client type | Refresh algorithm rewritten (ARCHITECTURE §4.3): idle expiry enforced, client type must match endpoint, used token is always reuse; contract descriptions updated |
| medium | Empty `TRUSTED_PROXIES` default would merge all clients into one rate-limit key | Compose sets it to the fixed `frontend` subnet; right-most untrusted hop; Caddy overwrites `X-Forwarded-For`; warning when empty (§3.4, §11, §13) |
| medium | `/storage` on the app origin exposed cookies and a script-capable surface | Proxy forwards only `POST /storage/<bucket>`, strips `Cookie`/`Authorization`, adds `CSP: sandbox`, `nosniff`, attachment; policy returns 204; separate origin recommended (§8, T-28) |
| medium | Global admin could make themselves admin of any private dormitory | Only existing unbanned members, never self, no membership creation, required reason sent to dormitory staff; `joined_via = 'admin_assigned'` removed from the schema (§5.5, T-09) |
| medium | Admin reset tokens allowed silent takeover | No tokens for self or other global admins; owner notified (`account.password_reset_issued`); 32-byte tokens; consumed on password change and deletion (§4.8, T-08) |
| medium | API keys and invitations outlived their creator's admin rights | Revoked automatically in the same transaction when the creator loses the role (§4.7, T-16) |
| medium | Reported moderators/admins could see and dismiss reports about themselves | New `reports.subject_user_id`; routing by the subject's role; reports about admins auto-escalated (§10, T-29) |
| low | Sanction `user_id` not tied to the dormitory; report could concern another user | Must be current or former member (`404`); composite FK `(report_id, user_id)` → `reports (id, subject_user_id)` |
| low | Logout needed an unexpired access token | Logout accepts a validly signed expired access token; always clears cookies (§4.2) |
| low | Trusted average with one trusted rater revealed an individual rating; self-flagging | Trusted average only when ≥ 3 trusted ratings; nobody sets their own flag |
| low | `audit_logs` allowed TRUNCATE and any DELETE | Triggers reject TRUNCATE and DELETE of rows younger than 30 days; retention ≥ 30 days (T-21). DB role split stays a phase 3 option |
| low | Registration and e-mail change reveal registered addresses | Accepted risk, documented (T-27) |
| info | Draft-menu children, Origin `null`, API key parsing, single GET on complete, evidence of deleted reported photos, targeted login lockout, session data on account deletion, `menus:read` aggregates | All documented in ARCHITECTURE §4.4, §4.7, §4.8, §5.3, §8, §11 and the contract conventions |

Phase 2 must implement and test these rules; the authorization test tables in `apps/api/tests/authz/`
must cover the report routing, the sanction target rules and the global-admin assignment limits.

## Phase 2 – Backend
- [ ] Config loading, fail-fast on missing secrets
- [ ] Auth: register, login, logout, refresh rotation, reuse detection, sessions, password change, account deletion
- [ ] Users: profile, avatar upload, statistics, reputation, trusted reviewer
- [ ] Dormitories: CRUD, search, join/leave, invitations, membership, dormitory settings
- [ ] Roles and RBAC (member, moderator, dormitory admin, global admin)
- [ ] Menus and meals (daily/weekly, categories, allergens, publishing)
- [ ] Ratings (1–5, one active per user per meal, DB unique constraint, distribution)
- [ ] Comments (pagination, edit/delete own, moderator delete, reports)
- [ ] Photos (MinIO, presigned upload, validation, moderator removal)
- [ ] Recommendations (separate from rating)
- [ ] Moderation (reports, review queue, suspension, audit log)
- [ ] In-app notifications
- [ ] API keys for server-to-server (hashed, scoped, expiring, rate limited)
- [ ] Rate limiting, security headers, CORS, CSRF
- [ ] Authorization tests for cross-dormitory access

## Phase 3 – Infrastructure
- [ ] API Dockerfile (non-root, multi-stage)
- [ ] Web Dockerfile
- [ ] docker-compose.yml with PostgreSQL, MinIO, migrations job, health checks, restart policies, volumes
- [ ] `.env.example` with placeholders only
- [ ] Backup and restore scripts

## Phase 4 – Web application
- [ ] Project setup (Vite, React, TypeScript, Tailwind, React Router, TanStack Query)
- [ ] i18n with Hungarian default and English
- [ ] Pages 1–13 from SPEC §7
- [ ] PWA installation
- [ ] Tests

## Phase 5 – Android application
- [ ] Project setup (Kotlin, Compose, Material 3, Retrofit or Ktor, Keystore)
- [ ] Features from SPEC §8
- [ ] Build verification (blocked, see `docs/ENVIRONMENT.md`)

## Phase 6 – iOS application
- [ ] Project setup (SwiftUI, URLSession, Keychain)
- [ ] Features from SPEC §9
- [ ] Build verification (blocked, see `docs/ENVIRONMENT.md`)

## Phase 7 – Documentation and legal
- [ ] README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY (placeholder contact), DISCLAIMER, PRIVACY (template), THIRD_PARTY_LICENSES, COPYRIGHT, TRADEMARKS.md
- [ ] LICENSE (unmodified official AGPL text, checksum recorded)
- [ ] GitHub issue and PR templates
- [ ] `legal-audit` run and findings resolved
- [ ] Final release checklist (SPEC §16)

## Open items for the maintainer
1. **Install `jq`** (or tell the agent to rewrite the hooks without it). Until then
   `.claude/hooks/block-secret-files.sh` does not block writes to `.env` or key files and
   `format-on-edit.sh` does not format.
2. **Android application ID** `io.github.madarlevi.kolikaja` (T-24): confirm or choose another before the
   first store release; it cannot change afterwards.
3. **MinIO server distribution**: check in phase 3 which maintained MinIO images are available and under
   which terms (ARCHITECTURE §18). The API only uses the S3 API, so another S3-compatible server could be used.
4. **Review the agent decisions T-01…T-29** in `docs/DECISIONS.md`, especially T-08 (admin-issued password
   reset, no e-mail), T-10 (only global administrators create dormitories by default), T-14 (individual
   ratings are private), T-27 (accepted account-enumeration risk) and T-28 (storage on the app origin by
   default, separate origin optional).
5. `docs/SPEC.md` §15.1 says AGPL-3.0-or-later; D-04 (AGPL-3.0-only) is applied everywhere. No action unless
   the maintainer wants to change D-04.
6. `TODO(operator)` values for deployments: privacy policy URL, terms URL, trademarks URL, operator contact and,
   for modified versions, `SOURCE_CODE_URL` (ARCHITECTURE §3.4, §16).

## Verification log
| Date | Step | Result |
|------|------|--------|
| 2026-11-26 | Toolchain check | Go 1.26.5, Node 24.13.1, npm 11.21.0, Java 21, Python 3.13.5 available; Docker daemon, PostgreSQL server, `jq`, `golangci-lint`, `migrate`, `hadolint` not available. |
| 2026-11-26 | Migrations on PostgreSQL 17.5 (embedded-postgres v1.34.0, golang-migrate v4.20.1, scratch harness outside the repo) | PASS: fresh `up` to version 15; `down 1` → 14; `up`; schema fingerprint identical; every step `down` to empty database (no tables or functions left); step-wise `up`; fingerprint identical again (10/10 checks). |
| 2026-11-26 | Schema constraint tests (same harness, each case in a rolled-back transaction) – first run | PASS 34/34 on the initial schema. |
| 2026-11-26 | Migrations and constraint tests after the security-review fixes (final schema) | PASS: 10/10 migration checks and 42/42 constraint tests. Duplicate active rating rejected; re-rating after soft delete allowed; `ON CONFLICT … WHERE deleted_at IS NULL` upsert works; rating 6 rejected; cross-tenant rating, comment, meal, report and sanction rejected by composite FKs; sanction citing a report about another user rejected and about the same user accepted; duplicate open report, self report, report on own content, user report with mismatching subject, membership via global-admin assignment, second active membership, unknown role, private+open dormitory, case-insensitive duplicate e-mail, plaintext password, unscrubbed deleted account, unknown allergen, duplicate menu slot, published menu without timestamp, second unused refresh token, short token hash, instance API key with write scope, malformed audit action and non-object notification payload rejected; audit log UPDATE, TRUNCATE and DELETE of recent rows rejected, DELETE of rows older than 30 days allowed; UTF-8 names stored. |
| 2026-11-26 | `npx @redocly/cli@2.84.0 lint docs/api/openapi.yaml` (before and after the security-review fixes) | PASS: valid, 0 errors, 0 warnings; 2 problems explicitly ignored (`/healthz`, `/readyz` have no 4XX, T-17). |
| 2026-11-26 | Project-rule contract checks (scratch Python script, before and after the fixes) | PASS on 112 operations and 36 request schemas: every unsafe cookie operation requires `csrfToken`; API key operations declare scopes and never reach `/admin`; authenticated operations document 401, role-restricted ones 403, dormitory-scoped ones 404; request bodies reject unknown fields and bound every string, integer and array; rating labels present verbatim. |
| 2026-11-26 | Second OpenAPI 3.1 schema validator (`@seriousme/openapi-schema-validator`) | NOT RUN: blocked by the Claude Code permission policy; not worked around. |
| 2026-11-26 | Migrations through Docker Compose `migrate` job | NOT VERIFIED: Docker daemon not available (D-01); job is written in phase 3. |
| 2026-11-26 | Legal audit (scoped) | PASS for headers, identifier, copyright, ® and claims; other checks NOT CHECKED until phase 7 (`docs/LEGAL_AUDIT.md`). |
| 2026-11-26 | `security-reviewer` subagent on the phase 1 design | 7 medium, 5 low findings; all addressed (see "Security review"). Re-review of the fixes: not run. |
