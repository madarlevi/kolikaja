-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0014 api_keys (down).
-- Drops all API keys; integrations stop working until new keys are issued.

BEGIN;

DROP TABLE api_keys;

COMMIT;
