# KoliKaja – security plan

This document describes the security tooling of madarlevi/kolikaja and the security gate that ends every
phase (D-10). It is an engineering plan, not a statement that the software is secure.

## 1. The gate
A phase is complete only when:
1. **Automated tools** in `.github/workflows/ci.yml` are green on the merged `main` commit (run URL recorded
   in `docs/PROGRESS.md`).
2. **Subagent review**: the local `security-reviewer` subagent (`.claude/agents/security-reviewer.md`)
   reviewed the phase's changes; its report is recorded in `docs/PROGRESS.md`.
3. **No open finding of severity high or critical** from either source. Medium and low findings are fixed,
   or recorded in `docs/DECISIONS.md` with a reason (accepted risk, false positive, deferred with phase).
4. **Phase-specific switches are turned on** when the phase that needs them closes: `AUTHZ_REQUIRE_TESTS=1`
   in the `openapi` job at the end of phase 2 (every dormitory-scoped operation needs an implemented
   `TestAuthz_` test), the payload harness against the Compose stack at the end of phase 3.

How CI enforces it: every security job fails on its threshold (table below); the `code-scanning-gate` job
fails while any open code-scanning alert on the pull request's ref has security severity high or critical
(CodeQL's own `analyze` step only uploads alerts and never fails). Jobs for components that do not exist yet
are skipped; the Go, migration and CodeQL Go jobs are unconditional because `apps/api` exists.

Severity: the tool's own severity for automated findings (CodeQL/Semgrep `error` = high, gosec high
severity = high, npm audit / govulncheck / dependency review by advisory severity); the reviewer's severity
for subagent findings. When sources disagree, the higher one counts.

## 2. Tools

| Area | Tool (version pinned in ci.yml) | Runs | Fails the build on | Active from |
|------|----------------------------------|------|--------------------|-------------|
| Secrets | gitleaks (`gitleaks/gitleaks-action` v3.0.0) | every push and PR, weekly; full history | any finding | now |
| Go security lint | golangci-lint v2.15.0 with `gosec` (`apps/api/.golangci.yml`) | push, PR | any issue | now (tooling and tests exist) |
| Go security | gosec v2.30.0 standalone | push, PR | any issue | now |
| Go dependencies | govulncheck v1.10.0 | push, PR, weekly | a vulnerability reachable from our code | now |
| SQL injection | Semgrep 1.187.0: custom rules `.semgrep/*.yml` (string building passed to database calls in Go and TypeScript, with tests) + `p/golang` + `p/typescript` | push, PR, weekly | any finding; the rule tests must pass | now |
| Static analysis | CodeQL `security-extended`, languages `go` and `javascript-typescript`, plus the `code-scanning-gate` job | push, PR, weekly | open alert with security severity high or critical on the ref | Go now; JS/TS with `apps/web` (phase 4) |
| Dependency changes | `actions/dependency-review-action` v5.0.1 | pull requests | new dependency with a high or critical advisory | now |
| Web dependencies | `npm audit --audit-level=high` | push, PR | high or critical advisory | phase 4 (`apps/web/package.json`) |
| Web lint | ESLint with `eslint-plugin-security` (CI fails if the plugin is not a devDependency) | push, PR | lint errors | phase 4 |
| Workflows | actionlint v1.7.12 with shellcheck | push, PR | any issue | now |
| Database rules | migrations up/down/up and 42 schema-constraint tests on PostgreSQL 17 (`apps/api/tests/integration`) | push, PR | any failure | now |
| Authorization | `scripts/check-authz-matrix.sh` (one matrix row per operation, unknown path prefixes rejected, every dormitory-scoped operation has a planned or real cross-dormitory test; with `AUTHZ_REQUIRE_TESTS=1` every one must be implemented); `TestAuthz_*` tests | push, PR | missing/unknown/stale rows, unclassified paths, failing tests; planned tests once the switch is on | matrix now; tests and switch phase 2 |
| Payload tests | `apps/api/tests/security`: 13 SQL injection and 9 XSS payloads against every operation of the contract | when `KOLIKAJA_API_URL` is set | 5xx, leaked SQL/stack details, HTML responses, missing `nosniff`, missing error envelope, time-based delay | inventory tests now; requests from phase 3 |
| Runtime scan | OWASP ZAP baseline (`zaproxy/action-baseline` v0.15.0, levels in `.zap/rules.tsv`) against the Compose stack | push, PR | rules marked FAIL | phase 3 (`docker-compose.yml`) |
| Images and Compose | image builds, `docker compose config`, `docker compose up --wait`, health checks | push, PR | any failure | phase 3 |
| Updates | Dependabot: Go modules, npm, GitHub Actions, Docker | weekly | – | now (npm and docker report "no manifest" until phases 3–4) |

Jobs for components that do not exist yet are skipped by the `detect` job, so the workflow stays green before
the code exists; each job switches on automatically when its files appear.

## 3. Rules for findings
- **Never** silence a finding to make CI green without a reason. Allowed suppressions:
  - Go: `//nolint:<linter> // #nosec <rule> -- <reason>` on the line (golangci-lint reads `nolint`,
    standalone gosec reads `#nosec`). Every suppression is reviewed in the pull request.
  - Semgrep: `// nosemgrep: <rule-id> -- <reason>`.
  - ZAP: change the level in `.zap/rules.tsv` with a comment in the pull request and an entry in
    `docs/DECISIONS.md`.
  - Redocly: `.redocly.lint-ignore.yaml` with a reason in `docs/DECISIONS.md`.
- Current suppressions (all in `apps/api/tests/security/security_test.go`): G101 on the *name* of the
  environment variable `KOLIKAJA_API_TOKEN` (not a credential); G704 on the two lines that send requests to
  the configured test target (the purpose of the harness). Where a false positive can be avoided by
  restructuring code (as for the operation table in `internal/authzmatrix`), restructure instead of suppressing.
- A vulnerable dependency that is not reachable (govulncheck "not called") is recorded in
  `docs/PROGRESS.md` and upgraded in the next Dependabot cycle.

## 4. Workflow hardening
Top-level `permissions: contents: read`; job-level elevation only where needed (`security-events: write`
for CodeQL, `contents: write` for releases, `packages: write` for GHCR); every third-party action pinned to
a full commit SHA with a version comment; `persist-credentials: false` on checkout; no secrets except
`GITHUB_TOKEN`; the CI PostgreSQL service uses trust authentication inside the job so no credential is
written into the workflow; Semgrep runs with metrics off (no telemetry).

## 5. Payload harness (apps/api/tests/security)
- The endpoint list is generated from `docs/api/openapi.yaml` at test time (`internal/contract`), so a new
  operation is covered automatically. `TestEndpointInventory` proves this on every run.
- Injection points: every path parameter, every query parameter and every string field of the JSON request
  body (nested one level), with plausible values in the other fields so validation reaches the strings.
- Configuration: `KOLIKAJA_API_URL` (base URL without `/api/v1`), optional `KOLIKAJA_API_TOKEN` (bearer token
  of a test user), `KOLIKAJA_SECURITY_ALLOW_WRITES=1` to include POST/PUT/PATCH/DELETE (only against a
  disposable test stack).
- Phase 3 wires it into the Compose job with a seeded test user.

## 6. Subagent reviews
The `security-reviewer` subagent is read-only and reports findings in a fixed format. Its reports are kept in
`docs/PROGRESS.md` ("Security review" sections). Phase 1: initial review (7 medium, 5 low, all addressed) and
the re-run of 2026-11-28 after the repository split and the authorization matrix (2 high, 5 medium, 5 low; the
high findings were fixed in the same session, see `docs/PROGRESS.md`).

## 7. Repository settings (maintainer)
The agent may not change repository settings beyond description and topics (D-07), so these server-side
controls are the maintainer's to enable. They are recommended for all three repositories:
- **Dependency graph** and Dependabot alerts (enabled for madarlevi/kolikaja on 2026-11-28; the dependency
  review job needs it).
- **Rulesets** for `main` (require a pull request and the CI status checks, block force pushes and deletion)
  and for tags `v*` (block deletion and updates; restrict creation to maintainers). Branch protection is the
  only control that stops a force push or tag push that slips past a client-side permission rule.
- **Code scanning merge protection** (block merging on high or critical alerts) as a second line behind
  `code-scanning-gate`.
- A **`release` environment** with a required reviewer for `release.yml` (releases are permanent).
- **Private vulnerability reporting**, so `SECURITY.md` can point to it until a contact address exists.

## 8. Mobile repositories
madarlevi/kolikaja-android and madarlevi/kolikaja-ios follow the same gate with their own tools (Android
lint, detekt, ktlint; SwiftLint; contract and client checks; gitleaks) and their own `security-reviewer`
subagent. Their plans live in their `docs/PROGRESS.md` and `CLAUDE.md`.
