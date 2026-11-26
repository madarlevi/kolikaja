---
name: auth-security
description: Use when implementing or changing registration, login, logout, password change, sessions, refresh-token rotation, JWT access tokens, CSRF protection, cookies, API keys, rate limiting on auth endpoints, or account deletion. Security-critical; read fully before editing.
---

# Authentication and session security

## Passwords
- Argon2id only. Start from OWASP parameters (memory 19 MiB, iterations 2, parallelism 1, or stronger) and record the chosen values in `ARCHITECTURE.md`.
- Store the encoded PHC string (`$argon2id$v=19$...`). Verify with constant-time comparison.
- Never store, log or return plaintext passwords. Password change requires the current password and revokes other sessions.
- Enforce minimum length (12) and maximum length (128) server-side. Reject the 1000 most common passwords if a list is available in the repo.

## Access tokens
- JWT, short-lived (default 10–15 minutes). Claims: `sub`, `sid` (session ID), `iat`, `exp`, `iss`, `aud`, `typ`.
- Verify algorithm explicitly; reject `none`. Keys from env, never defaults.
- Access tokens are not stored server-side; revocation is done by session status checked on each request for sensitive operations, or by a short expiry.

## Refresh tokens and sessions
- Refresh token: 32+ bytes from `crypto/rand`, base64url. Only the SHA-256 hash is stored in `user_sessions`.
- Each refresh issues a new refresh token and invalidates the previous one.
- Reuse detection: if a rotated (already used) refresh token is presented, revoke the whole session and all descendants, and write an audit log entry.
- Sessions list shows device label, created_at, last_used_at, IP (truncated), and allows revoking individual sessions.
- Logout revokes the current session. "Logout everywhere" revokes all.

## Cookies (web)
- `HttpOnly; Secure; SameSite=Strict` for the refresh cookie (path restricted to the refresh endpoint). `SameSite=Lax` is acceptable for the access cookie only if CSRF protection is in place.
- CSRF: double-submit token or synchronizer token. Required header on every state-changing cookie-authenticated request. Checked before handler logic.

## Native clients
- Bearer access token in `Authorization: Bearer`. Refresh token in the request body to a native-only endpoint, or in a header, never in a URL.

## API keys (server-to-server)
- Format: prefix + 32+ random bytes; show the full key once; store only a hash and a non-secret prefix for lookup.
- Scopes, expiry, dormitory association, revocation, usage counters and audit events.
- Per-key rate limits. API keys never bypass tenant or role checks.

## Rate limiting
- Login: per IP and per account, with exponential backoff. Do not reveal whether an email exists (generic error, equal timing where practical).
- Registration: per IP.

## Account deletion
- Requires password confirmation. Revokes all sessions. Removes or anonymizes personal data per `PRIVACY.md`. Keeps only what is legally necessary, documented in PRIVACY.md.

## Review
After any change in this area, run the `security-reviewer` subagent and record its findings in `docs/PROGRESS.md`.
