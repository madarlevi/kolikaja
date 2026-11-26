-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0010 photos (down).
-- Drops photo and avatar metadata. Objects in MinIO are not touched by this migration;
-- without the metadata they become unreachable and must be removed manually if needed.
-- This data loss cannot be avoided; take a backup first.

BEGIN;

DROP TABLE user_avatars;
DROP TABLE meal_photos;

COMMIT;
