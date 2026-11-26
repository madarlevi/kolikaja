# KoliKaja – progress

Status legend: `[ ]` not started · `[~]` in progress · `[x]` done and verified · `[!]` done but NOT verified (reason given)

## Phase 1 – Foundation
- [ ] Inspect repository
- [ ] Write `docs/ARCHITECTURE.md`
- [ ] Create folder structure
- [ ] Define database schema (migrations 0001–0015, up and down)
- [ ] Define OpenAPI 3.1 contract (`docs/api/openapi.yaml`)

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
- (none yet)

## Verification log
| Date | Step | Result |
|------|------|--------|
