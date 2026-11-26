-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0008 ratings (down).
-- Reverting an initial create migration drops all ratings.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE ratings;

COMMIT;
