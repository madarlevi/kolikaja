# KoliKaja – agent instructions

## Project
KoliKaja is a self-hostable, multi-dormitory food menu and community rating platform.
Stack: Go API (chi, pgx, PostgreSQL, golang-migrate), React + TypeScript + Vite web app,
native Kotlin/Jetpack Compose Android app, native SwiftUI iOS app, MinIO for photos,
Docker Compose for deployment.

## Source of truth (read in this order at the start of each session)
1. `docs/DECISIONS.md` – decisions made by the maintainer. These override `docs/SPEC.md`.
2. `docs/SPEC.md` – the full original brief.
3. `docs/ARCHITECTURE.md` – the design written in phase 1. Keep it current when the design changes.
4. `docs/PROGRESS.md` – current state, open items, verification status. Update after every work unit.
5. `docs/ENVIRONMENT.md` – what can and cannot be run in the current environment.

## Repository layout
- `apps/api/` – Go backend. Has its own `CLAUDE.md`.
- `apps/web/` – React web app. Has its own `CLAUDE.md`.
- `apps/android/` – Kotlin/Compose app. Has its own `CLAUDE.md`.
- `apps/ios/` – SwiftUI app. Has its own `CLAUDE.md`.
- `deploy/` – Dockerfiles, compose files, reverse proxy config, backup/restore scripts.
- `docs/` – specification, decisions, architecture, progress, environment, API docs, legal docs.
- `.claude/` – skills, subagents, hooks and settings for this project.

Read the `CLAUDE.md` of an app before changing anything inside it.

## Working rules
- Work phase by phase as listed in `docs/PROGRESS.md`. Do not start a later phase unless asked.
- Use the task list for any multi-step work. Mark an item complete only after it is verified.
- Never claim that a build, test, lint or deployment passed unless you ran it and saw it pass.
  If a tool is unavailable (see `docs/ENVIRONMENT.md`), record the step as "not verified" in `docs/PROGRESS.md`.
- Commit in small steps using Conventional Commits, for example `feat(api): add dormitory membership endpoints`.
- Do not push, do not change git remotes, do not rewrite history.
- If a decision is missing from `docs/DECISIONS.md` and `docs/SPEC.md`, choose the sensible option,
  implement it, and record it in `docs/DECISIONS.md` with a short reason.

## Non-negotiable rules
- **Tenant isolation:** every dormitory-scoped query is restricted to dormitories where the authenticated
  user has membership, checked on the server. Role or dormitory identifiers sent by a client are never trusted
  as authorization input. Use the `authz-tenant-isolation` skill for every dormitory-scoped endpoint.
- **SQL:** only parameterized queries (pgx placeholders). No string concatenation or formatting into SQL.
- **Secrets:** never in source code, logs, client apps, committed fixtures or documentation.
  `.env` is git-ignored. `.env.example` contains placeholders only.
- **No default production credentials.** Compose must fail fast when required secrets are missing.
- **Migrations:** every migration has an `up` and a `down` file. Never edit a migration that was already released; add a new one.
- **Contract first:** the OpenAPI 3.1 document is the API contract. Change the spec, then the server,
  then regenerate the clients. Breaking changes need a new API version.
- **Errors:** responses never contain stack traces, SQL text or internal error details.
- **Passwords and tokens:** Argon2id for passwords. Refresh tokens are random, stored only as hashes,
  rotated on every use, and reuse of a rotated token revokes the whole session family.
- **Rating labels:** the five Hungarian labels in `docs/DECISIONS.md` must match exactly. All files are UTF-8.
- **Out of scope for this version:** PDF parsing, OCR, menu scraping, n8n workflows, payments, subscriptions,
  AI recommendations, external automation services, push notifications (leave an extension point only).
- **No telemetry** of any kind without explicit user consent.

## Licensing and legal
- Project license: GNU AGPL-3.0 (exact variant as stated in `docs/DECISIONS.md`).
- `LICENSE` must be the unmodified official text from the Free Software Foundation. Never retype or edit it.
- Add an `SPDX-License-Identifier:` header to source files that support comments.
- Copyright holder: madarlevi. Copyright notices must be consistent across the repository.
- Do not add trademark restrictions to the AGPL text. Trademark rules live in `TRADEMARKS.md` only.
- Do not invent legal entities, addresses, contact e-mails, registrations or compliance certifications.
  Use `TODO(operator)` placeholders and list them in the final report.
- Never state that the project is legally compliant. Use the `legal-audit` skill and the `legal-auditor` subagent.

## Environment limits
Read `docs/ENVIRONMENT.md`. At the time of writing Docker is not available. Compose and Dockerfile
files are still written and checked statically (YAML/Dockerfile syntax, `hadolint` if available), and every
runtime step is recorded as "not verified".

## Definition of done (per work unit)
- Code compiles and tests run, or the reason they could not run is recorded.
- Formatting and lint pass for every touched app.
- OpenAPI validates; clients regenerated if the contract changed.
- Every new dormitory-scoped endpoint has cross-tenant authorization tests.
- `docs/PROGRESS.md` and, if relevant, `docs/DECISIONS.md` are updated.
- `legal-audit` was run if licensing, headers or documentation changed.
- The security-reviewer subagent was run for auth, sessions, uploads or authorization changes.
