---
name: db-migration
description: Use when creating or changing PostgreSQL schema, indexes, constraints or seed data in apps/api/migrations. Covers golang-migrate file naming, up/down pairs, tenant keys, constraints for ratings and soft deletion, and safe rollout rules.
---

# Database migrations

Tool: golang-migrate with SQL files. Location: `apps/api/migrations/`.

## Naming
- `NNNN_short_description.up.sql` and `NNNN_short_description.down.sql`, with a 4-digit sequence number.
- One logical change per migration. Never edit a migration that has been merged or released; add a new one.

## Required tables (from SPEC §6)
users, user_sessions, dormitories, dormitory_members, invitations, menus, meals, ratings, comments, meal_photos, recommendations, notifications, reports, api_keys, audit_logs.

## Rules
- Primary keys: `uuid` generated in the application (see ARCHITECTURE.md).
- Timestamps: `created_at timestamptz NOT NULL DEFAULT now()`, `updated_at` maintained by the application, `deleted_at timestamptz` only where soft delete is required.
- Foreign keys on every reference. Use `ON DELETE` explicitly; prefer `RESTRICT` for tenant data.
- Tenant ownership explicit: `dormitory_id uuid NOT NULL REFERENCES dormitories(id)` on every dormitory-scoped table, including child tables such as ratings and comments.
- Uniqueness enforced in the database, for example:
  - `ratings`: `UNIQUE (meal_id, user_id) WHERE deleted_at IS NULL` (partial unique index), so there is one active rating per user per meal.
  - `dormitory_members`: `UNIQUE (dormitory_id, user_id) WHERE left_at IS NULL`.
  - `users`: `UNIQUE (lower(email))`.
- Check constraints for enums and ranges: rating value `BETWEEN 1 AND 5`, role in an allowed set, status in an allowed set.
- Indexes: foreign keys, every `(dormitory_id, ...)` access path, and cursor pagination keys (`created_at, id`).
- Passwords and tokens: store only hashes. Column names must say so (`password_hash`, `token_hash`).
- Down migrations must restore the previous schema without data loss where possible; if not possible, add a comment explaining why.

## Procedure
1. Write the up file, then the down file.
2. Run `migrate -path apps/api/migrations -database "$DATABASE_URL" up` and `down 1`, then `up` again against a local database. If no database is available, record "not verified".
3. Confirm the migration is idempotent in the sense that a fresh database reaches the same schema.
4. Update the schema table in `docs/ARCHITECTURE.md` and the phase checklist in `docs/PROGRESS.md`.
