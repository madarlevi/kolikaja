-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0013 moderation (down).
-- Reverting an initial create migration drops all reports and sanctions.
-- Active sanctions stop applying. Take a backup first.

BEGIN;

DROP TABLE user_sanctions;
DROP TABLE reports;

COMMIT;
