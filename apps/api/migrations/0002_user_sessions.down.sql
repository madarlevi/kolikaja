-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0002 user_sessions (down).
-- Drops sessions and tokens. Every user is logged out; no other data is lost.

BEGIN;

DROP TABLE password_reset_tokens;
DROP TABLE refresh_tokens;
DROP TABLE user_sessions;

COMMIT;
