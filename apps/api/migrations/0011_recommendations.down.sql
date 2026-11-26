-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0011 recommendations (down).
-- Reverting an initial create migration drops all recommendations.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE recommendations;

COMMIT;
