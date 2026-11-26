-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0005 dormitory_members (down).
-- Reverting an initial create migration drops all memberships.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE dormitory_members;

COMMIT;
