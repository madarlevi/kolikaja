-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0004 invitations (down).
-- Drops all invitation codes. Existing memberships are not affected because
-- dormitory_members is created in a later migration and is dropped first.

BEGIN;

DROP TABLE invitations;

COMMIT;
