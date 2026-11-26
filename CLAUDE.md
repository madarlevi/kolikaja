# KoliKaja – agent instructions

## Project
KoliKaja is a self-hostable, multi-dormitory food menu and community rating platform.
This repository (madarlevi/kolikaja) contains the Go API (chi, pgx, PostgreSQL, golang-migrate), the
React + TypeScript + Vite web app, the deployment (Docker Compose, MinIO for photos), the documentation and
the OpenAPI contract. The native apps live in separate repositories (D-06):
- madarlevi/kolikaja-android – Kotlin / Jetpack Compose app.
- madarlevi/kolikaja-ios – SwiftUI app.
They consume the contract from the releases of this repository (D-09); nothing in this repository depends on them.

## Source of truth (read in this order at the start of each session)
1. `docs/DECISIONS.md` – decisions made by the maintainer. These override `docs/SPEC.md`.
2. `docs/SPEC.md` – the full original brief (it still describes the apps; the repository split is D-06).
3. `docs/ARCHITECTURE.md` – the design. Keep it current when the design changes.
4. `docs/PROGRESS.md` – current state, open items, verification status. Update after every work unit.
5. `docs/ENVIRONMENT.md` – what can and cannot be run locally; GitHub Actions is the verification
   environment for everything that cannot (D-08).
6. `docs/SECURITY-PLAN.md` – security tooling and the per-phase security gate (D-10).
7. `docs/DESIGN.md` – UI direction for the web app (and, by reference, the mobile apps).

## Repository layout
- `apps/api/` – Go backend. Has its own `CLAUDE.md`.
- `apps/web/` – React web app. Has its own `CLAUDE.md`.
- `deploy/` – Dockerfiles, compose files, reverse proxy config, backup/restore scripts.
- `docs/` – specification, decisions, architecture, progress, environment, security plan, design, API
  contract (`docs/api/openapi.yaml`) and authorization matrix (`docs/api/authz-matrix.md`), legal docs.
- `scripts/` – repository tooling (`check-authz-matrix.sh`, client generation for the web app).
- `.github/` – CI and release workflows, Dependabot, issue and pull request templates.
- `.semgrep/`, `.zap/` – custom Semgrep rules with tests; OWASP ZAP baseline rules.
- `.claude/` – skills, subagents, hooks and settings for this project.

Read the `CLAUDE.md` of an app before changing anything inside it.

## Working rules
- Work phase by phase as listed in `docs/PROGRESS.md`. Do not start a later phase unless asked.
- Use the task list for any multi-step work. Mark an item complete only after it is verified.
- Never claim that a build, test, lint or deployment passed unless you ran it and saw it pass, locally or in
  a CI run whose URL you record. If neither is possible, record the step as "not verified" in `docs/PROGRESS.md`.
- Conventional Commits, for example `feat(api): add dormitory membership endpoints`.
- Git workflow (D-07, D-11): push only to madarlevi/kolikaja, madarlevi/kolikaja-android and
  madarlevi/kolikaja-ios. The first commit on an empty default branch goes directly to `main`; every later
  change goes through a feature branch and a pull request that is squash-merged after CI is green.
  Never force-push, never rewrite published history, never delete branches, tags, releases or repositories,
  never change repository settings beyond description and topics.
- If a decision is missing from `docs/DECISIONS.md` and `docs/SPEC.md`, choose the sensible option,
  implement it, and record it in `docs/DECISIONS.md` with a short reason.

## Non-negotiable rules
- **Tenant isolation:** every dormitory-scoped query is restricted to dormitories where the authenticated
  user has membership, checked on the server. Role or dormitory identifiers sent by a client are never trusted
  as authorization input. Use the `authz-tenant-isolation` skill for every dormitory-scoped endpoint.
- **SQL:** only parameterized queries (pgx placeholders). No string concatenation or formatting into SQL
  (enforced by the custom Semgrep rule in `.semgrep/`).
- **Secrets:** never in source code, logs, client apps, committed fixtures or documentation.
  `.env` is git-ignored. `.env.example` contains placeholders only. CI uses no secret except `GITHUB_TOKEN`.
- **No default production credentials.** Compose must fail fast when required secrets are missing.
- **Migrations:** every migration has an `up` and a `down` file. Never edit a migration that was already
  released (a release is a `vX.Y.Z` tag); add a new one.
- **Contract first:** the OpenAPI 3.1 document is the API contract. Change the spec, regenerate the
  authorization matrix (`scripts/check-authz-matrix.sh --write`), then change the server, then regenerate the
  web client. Breaking changes need a new API version. Releases attach the contract as
  `openapi-vX.Y.Z.yaml`; `info.version` must equal the tag without `v`.
- **Errors:** responses never contain stack traces, SQL text or internal error details.
- **Passwords and tokens:** Argon2id for passwords. Refresh tokens are random, stored only as hashes,
  rotated on every use, and reuse of a rotated token revokes the whole session family.
- **Rating labels:** the five Hungarian labels in `docs/DECISIONS.md` must match exactly. All files are UTF-8.
- **Out of scope for this version:** PDF parsing, OCR, menu scraping, n8n workflows, payments, subscriptions,
  AI recommendations, external automation services, push notifications (leave an extension point only).
- **No telemetry** of any kind without explicit user consent (this includes tools: Semgrep runs with metrics off).

## Licensing and legal
- Project license: GNU AGPL-3.0 (exact variant as stated in `docs/DECISIONS.md`).
- `LICENSE` must be the unmodified official text from the Free Software Foundation. Never retype or edit it;
  its SHA-256 is recorded in `docs/PROGRESS.md`.
- Add an `SPDX-License-Identifier:` header to source files that support comments.
- Copyright holder: madarlevi. Copyright notices must be consistent across the repository.
- Do not add trademark restrictions to the AGPL text. Trademark rules live in `TRADEMARKS.md` only.
- Do not invent legal entities, addresses, contact e-mails, registrations or compliance certifications.
  Use `TODO(operator)` placeholders and list them in the final report.
- Never state that the project is legally compliant. Use the `legal-audit` skill and the `legal-auditor` subagent.

## Environment limits
Read `docs/ENVIRONMENT.md`. Docker is not available locally, so Docker images, the Compose stack and the ZAP
scan are verified by GitHub Actions (`.github/workflows/ci.yml`); record the run URL in `docs/PROGRESS.md`.

## Definition of done (per work unit)
- Code compiles and tests run (locally or in CI with the run URL), or the reason they could not run is recorded.
- Formatting and lint pass for every touched app.
- OpenAPI validates; the authorization matrix check passes; clients regenerated if the contract changed.
- Every new dormitory-scoped endpoint has cross-tenant authorization tests (`TestAuthz_<OperationId>`).
- `docs/PROGRESS.md` and, if relevant, `docs/DECISIONS.md` are updated.
- `legal-audit` was run if licensing, headers or documentation changed.
- Security gate (D-10, `docs/SECURITY-PLAN.md`): the CI security jobs are green and, for auth, sessions,
  uploads or authorization changes and at the end of every phase, the `security-reviewer` subagent was run.
  No high or critical finding may stay open; medium and low findings are fixed or recorded with a reason.
