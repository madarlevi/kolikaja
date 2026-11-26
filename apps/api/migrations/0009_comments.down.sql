-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0009 comments (down).
-- Reverting an initial create migration drops all comments.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE comments;

COMMIT;
