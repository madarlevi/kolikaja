-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0001 users (down).
-- Reverting an initial create migration drops the table and therefore all of its rows.
-- This data loss cannot be avoided; take a backup (deploy/scripts/backup.sh) first.

BEGIN;

DROP TABLE users;

COMMIT;
