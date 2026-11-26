---
name: api-contract
description: Use when adding or changing any REST endpoint, request or response schema, error format, pagination or auth scheme. Covers spec-first OpenAPI 3.1 editing, validation, client regeneration for TypeScript, Kotlin and Swift, and compatibility checks.
---

# API contract (OpenAPI 3.1)

Source of truth: `docs/api/openapi.yaml`. Base path: `/api/v1`.

## Workflow
1. **Spec first.** Add or change the path, schemas, parameters, examples and error responses in the OpenAPI file.
2. **Validate:** `npx @redocly/cli lint docs/api/openapi.yaml` (or the repo's configured lint command). Fix all errors; warnings need a reason in `docs/DECISIONS.md`.
3. **Implement** the handler and service in `apps/api`.
4. **Regenerate clients** using the repo script (`scripts/gen-clients.sh`):
   - TypeScript -> `apps/web/src/api/` (openapi-typescript + a small typed fetch wrapper)
   - Kotlin -> `apps/android/.../core/network/generated/` (openapi-generator, kotlin with kotlinx-serialization)
   - Swift -> `apps/ios/.../Core/Network/Generated/` (openapi-generator, swift5)
   Generated files are committed and never hand-edited.
5. **Compatibility check:** compare with the previous version (`oasdiff breaking`, or the equivalent). Breaking changes to `/api/v1` are not allowed; create `/api/v2` instead.

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
