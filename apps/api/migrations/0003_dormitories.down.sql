-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0003 dormitories (down).
-- Reverting an initial create migration drops the table and all of its rows.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE dormitories;

COMMIT;
