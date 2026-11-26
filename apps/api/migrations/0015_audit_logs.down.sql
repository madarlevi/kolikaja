-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0015 audit_logs (down).
-- Reverting an initial create migration drops the audit trail.
-- This data loss cannot be avoided; export or back up the table first.

BEGIN;

DROP TRIGGER audit_logs_retention_delete ON audit_logs;
DROP TRIGGER audit_logs_no_truncate ON audit_logs;
DROP TRIGGER audit_logs_no_update ON audit_logs;
DROP TABLE audit_logs;
DROP FUNCTION audit_logs_reject_change();

COMMIT;
