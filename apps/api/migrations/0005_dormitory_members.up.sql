-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0005 dormitory_members (up): membership and dormitory roles.
--
-- This table is the single source of authorization for dormitory-scoped data.
-- The API checks membership with:
--   SELECT role FROM dormitory_members
--   WHERE dormitory_id = $1 AND user_id = $2 AND left_at IS NULL
-- Rows are kept as history: leaving sets left_at instead of deleting the row.
-- Global administrators are flagged on users.is_global_admin and are not members implicitly.

BEGIN;

CREATE TABLE dormitory_members (
    id                  uuid        PRIMARY KEY,
    dormitory_id        uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    user_id             uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    role                text        NOT NULL DEFAULT 'member',
    is_trusted_reviewer boolean     NOT NULL DEFAULT false,
    joined_via          text        NOT NULL,
    invitation_id       uuid,
    joined_at           timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    left_at             timestamptz,
    leave_reason        text,
    removed_by          uuid        REFERENCES users (id) ON DELETE RESTRICT,

    -- The invitation must belong to the same dormitory (tenant backstop).
    CONSTRAINT dormitory_members_invitation_fkey FOREIGN KEY (invitation_id, dormitory_id)
        REFERENCES invitations (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT dormitory_members_role_check
        CHECK (role IN ('member', 'moderator', 'admin')),
    -- There is deliberately no "assigned by a global administrator" path: global
    -- administrators can only promote users who are already active members.
    CONSTRAINT dormitory_members_joined_via_check
        CHECK (joined_via IN ('creator', 'open_join', 'invitation')),
    CONSTRAINT dormitory_members_invitation_check
        CHECK ((joined_via = 'invitation') = (invitation_id IS NOT NULL)),
    CONSTRAINT dormitory_members_leave_check
        CHECK ((left_at IS NULL) = (leave_reason IS NULL)),
    CONSTRAINT dormitory_members_leave_reason_check
        CHECK (leave_reason IS NULL OR leave_reason IN ('left', 'removed', 'banned', 'account_deleted', 'dormitory_deleted')),
    CONSTRAINT dormitory_members_removed_by_check
        CHECK (removed_by IS NULL OR (leave_reason IS NOT NULL AND leave_reason IN ('removed', 'banned')))
);

-- One active membership per user and dormitory. INCLUDE allows index-only membership checks.
CREATE UNIQUE INDEX dormitory_members_active_key ON dormitory_members (dormitory_id, user_id)
    INCLUDE (role, is_trusted_reviewer) WHERE left_at IS NULL;
-- "My dormitories" and shared-dormitory checks by user.
CREATE INDEX dormitory_members_user_active_idx ON dormitory_members (user_id, dormitory_id)
    INCLUDE (role) WHERE left_at IS NULL;
-- Member lists filtered by role.
CREATE INDEX dormitory_members_dormitory_role_idx ON dormitory_members (dormitory_id, role, joined_at)
    WHERE left_at IS NULL;
CREATE INDEX dormitory_members_invitation_idx ON dormitory_members (invitation_id)
    WHERE invitation_id IS NOT NULL;
-- Membership history (rejoin rules, data export).
CREATE INDEX dormitory_members_history_idx ON dormitory_members (user_id, joined_at DESC);

COMMIT;
