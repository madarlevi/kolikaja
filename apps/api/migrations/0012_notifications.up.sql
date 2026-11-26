-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0012 notifications (up): dormitory announcements and in-app notifications.
--
-- Notifications are per user. dormitory_id is set for dormitory content
-- (comments, announcements); the API lists those only while the user is an active
-- member and deletes them when the user leaves. Moderation notifications about the
-- user's own account carry dormitory_id = NULL.
-- payload holds identifiers and short display texts only, never secrets.
-- Push delivery is out of scope; see "Notifications" in docs/ARCHITECTURE.md.

BEGIN;

CREATE TABLE announcements (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    -- NULL when published with an API key; the audit log records the key.
    author_id    uuid        REFERENCES users (id) ON DELETE RESTRICT,
    title        text        NOT NULL,
    body         text        NOT NULL,
    priority     text        NOT NULL DEFAULT 'normal',
    published_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    deleted_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,

    CONSTRAINT announcements_id_dormitory_key UNIQUE (id, dormitory_id),
    CONSTRAINT announcements_title_check
        CHECK (char_length(title) BETWEEN 1 AND 150),
    CONSTRAINT announcements_body_check
        CHECK (char_length(body) BETWEEN 1 AND 5000),
    CONSTRAINT announcements_priority_check
        CHECK (priority IN ('normal', 'important')),
    CONSTRAINT announcements_expiry_check
        CHECK (expires_at IS NULL OR expires_at > published_at),
    CONSTRAINT announcements_deletion_check
        CHECK (deleted_by IS NULL OR deleted_at IS NOT NULL)
);

CREATE INDEX announcements_dormitory_idx ON announcements (dormitory_id, published_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE notifications (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Derived data: removed together with the dormitory if a dormitory is ever purged.
    dormitory_id uuid        REFERENCES dormitories (id) ON DELETE CASCADE,
    kind         text        NOT NULL,
    payload      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    -- Repeated events with the same group_key update one unread notification.
    group_key    text,
    occurrences  integer     NOT NULL DEFAULT 1,
    created_at   timestamptz NOT NULL DEFAULT now(),
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    read_at      timestamptz,

    CONSTRAINT notifications_kind_check
        CHECK (kind IN (
            'comment.created', 'announcement.published',
            'moderation.content_removed', 'moderation.sanction_applied', 'moderation.sanction_lifted',
            'moderation.report_resolved', 'membership.role_changed', 'membership.admin_assigned',
            'account.password_reset_issued')),
    CONSTRAINT notifications_payload_check
        CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 4096),
    CONSTRAINT notifications_group_key_check
        CHECK (group_key IS NULL OR char_length(group_key) BETWEEN 1 AND 200),
    CONSTRAINT notifications_occurrences_check
        CHECK (occurrences >= 1)
);

-- Notification list (newest first, cursor pagination).
CREATE INDEX notifications_user_page_idx ON notifications (user_id, occurred_at DESC, id DESC);
-- Unread badge count.
CREATE INDEX notifications_user_unread_idx ON notifications (user_id) WHERE read_at IS NULL;
-- Grouping key for unread notifications.
CREATE UNIQUE INDEX notifications_group_unread_key ON notifications (user_id, group_key)
    WHERE read_at IS NULL AND group_key IS NOT NULL;
-- Cleanup when a user leaves a dormitory.
CREATE INDEX notifications_dormitory_idx ON notifications (dormitory_id, user_id)
    WHERE dormitory_id IS NOT NULL;

COMMIT;
