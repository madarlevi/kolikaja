---
name: authz-tenant-isolation
description: Use when adding or changing any endpoint, query or service that touches dormitory-scoped data (menus, meals, ratings, comments, photos, members, invitations, reports, notifications). Enforces server-side membership and role checks and cross-dormitory tests.
---

# Tenant isolation and authorization

KoliKaja is multi-tenant by dormitory. A user in dormitory A must never read or change dormitory B data, and client-supplied roles or IDs are never authorization input.

## Checklist for every dormitory-scoped endpoint
1. **Route** is under `/api/v1/dormitories/{dormitoryId}/...` or derives the dormitory from the owning resource (for example a rating ID -> meal -> menu -> dormitory).
2. **Authentication** middleware runs first and produces the user ID from the verified token or session. Never from a request body.
3. **Membership check** happens on the server, in the service layer, using a query such as `SELECT role FROM dormitory_members WHERE dormitory_id = $1 AND user_id = $2 AND left_at IS NULL`.
4. **Role check** uses the role from step 3 against the minimum required role. Roles in JSON bodies, headers or query strings are ignored.
5. **Resource scoping:** every read and write query includes the dormitory condition, for example `WHERE meal_id = $1 AND dormitory_id = $2`. Do not rely on the handler to filter afterwards.
6. **Not-found vs forbidden:** if the user is not a member, return 404 for resources of another dormitory, so existence is not leaked. Return 403 only for members lacking the role.
7. **Database backstop:** composite foreign keys or `dormitory_id` columns on child tables where practical, so a bug cannot join across tenants silently.
8. **Audit:** moderator and admin actions write a row to `audit_logs` in the same transaction.

## Authorization test template (`apps/api/tests/authz/`)
For each new endpoint, add table-driven tests with at least:
- anonymous user -> 401
- authenticated non-member of dormitory A -> 404 on A's resource
- member of dormitory B using A's resource ID -> 404
- member role below required role -> 403
- member with required role -> 2xx
- forged `role` or `dormitory_id` in body/query -> ignored, result unchanged

## Common mistakes to avoid
- Checking membership in the handler but not in the query that loads the resource.
- Using `WHERE id = $1` alone on child resources.
- Trusting a `dormitoryId` passed by the client without checking membership of that specific dormitory.
- Returning the full resource list and filtering in the frontend.
- Letting API keys bypass tenant checks. API keys carry a dormitory scope and go through the same checks.
