-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0007 meals (down).
-- Reverting an initial create migration drops all meals.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE meals;

COMMIT;
