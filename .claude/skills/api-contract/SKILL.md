---
name: api-contract
description: Use when adding or changing any REST endpoint, request or response schema, error format, pagination or auth scheme. Covers spec-first OpenAPI 3.1 editing, validation, the authorization matrix, TypeScript client regeneration, contract releases for the mobile repositories, and compatibility checks.
---

# API contract (OpenAPI 3.1)

Source of truth: `docs/api/openapi.yaml`. Base path: `/api/v1`.

## Workflow
1. **Spec first.** Add or change the path, schemas, parameters, examples and error responses in the OpenAPI file.
2. **Validate:** `npx @redocly/cli lint docs/api/openapi.yaml` (or the repo's configured lint command). Fix all errors; warnings need a reason in `docs/DECISIONS.md`.
3. **Authorization matrix:** `scripts/check-authz-matrix.sh --write`, review the changed rows in `docs/api/authz-matrix.md`, commit them. CI fails if the matrix is missing a row or is stale.
4. **Implement** the handler and service in `apps/api`, with `TestAuthz_<OperationId>` tests for dormitory-scoped operations.
5. **Regenerate the web client** (`scripts/gen-clients.sh`, phase 4): TypeScript -> `apps/web/src/api/generated/` (openapi-typescript + openapi-fetch). Generated files are committed and never hand-edited.
6. **Release for the mobile apps:** the Android and iOS apps live in madarlevi/kolikaja-android and madarlevi/kolikaja-ios (D-06). They pin a contract release in `api-version.txt` and regenerate their clients from the release asset `openapi-vX.Y.Z.yaml` (D-09). To release: set `info.version` to `X.Y.Z`, merge, tag `vX.Y.Z` on `main`; `.github/workflows/release.yml` attaches the asset and its SHA-256. Never edit a released contract file; release a new version.
7. **Compatibility check:** compare with the previous release (`oasdiff breaking`, or the equivalent). Breaking changes to `/api/v1` are not allowed; create `/api/v2` instead.

## Conventions
- Every operation has `operationId`, `summary`, tags, and documented responses for 400, 401, 403, 404, 409, 422 and 429 where applicable.
- Error envelope, single format everywhere:
  ```json
  { "error": { "code": "validation_failed", "message": "Human-readable message", "details": [ { "field": "rating", "reason": "out_of_range" } ], "request_id": "..." } }
  ```
  No stack traces, SQL or internal messages.
- Pagination: cursor-based for lists that grow (comments, meals, notifications). Response: `{ "data": [...], "next_cursor": "..." }`.
- Rate-limit headers documented: `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` on 429.
- Auth: `cookieAuth` (HttpOnly session cookie + `X-CSRF-Token` header) for web and `bearerAuth` for native clients. API keys: `apiKeyAuth` in header `X-API-Key`.
- Dates: RFC 3339 UTC strings. Ratings: integer 1–5. The API returns only the numeric value; clients map 1–5 to the exact Hungarian labels in `docs/DECISIONS.md` via their i18n resources.
- Request bodies: `additionalProperties: false`, explicit `maxLength`, `maxItems`, `minimum` and `maximum`.
- Security schemes must match the auth-security rules; no endpoint is public unless marked as such.
