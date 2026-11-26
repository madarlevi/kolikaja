-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0006 menus (down).
-- Reverting an initial create migration drops all menus and categories.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE menus;
DROP TABLE meal_categories;

COMMIT;
