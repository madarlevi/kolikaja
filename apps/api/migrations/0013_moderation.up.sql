-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0013 moderation (up): reports and user sanctions.
--
-- Reports are always filed in a dormitory. subject_user_id is the user the report is
-- about (the reported user, or the author of the reported comment or photo). Only
-- members with a higher role than the subject may see or resolve a report, so nobody
-- handles a report about themselves; reports about dormitory administrators are
-- escalated to the global administrators when they are filed.
-- Sanctions:
--   dormitory_id set,  kind 'suspension' -> read-only in that dormitory until ends_at
--   dormitory_id set,  kind 'ban'        -> removed from the dormitory, cannot rejoin
--   dormitory_id NULL, kind 'suspension' -> read-only everywhere (global administrators only)
--   dormitory_id NULL, kind 'ban'        -> cannot log in; sessions are revoked
-- Sanctions are keyed by user and dormitory, not by membership row, so leaving and
-- rejoining does not lift them.

BEGIN;

CREATE TABLE reports (
    id                uuid        PRIMARY KEY,
    dormitory_id      uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    reporter_id       uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    target_type       text        NOT NULL,
    target_comment_id uuid,
    target_photo_id   uuid,
    target_user_id    uuid        REFERENCES users (id) ON DELETE RESTRICT,
    subject_user_id   uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    reason            text        NOT NULL,
    details           text,
    -- Copy of the reported text at report time, so evidence survives edits and deletion.
    content_snapshot  text,
    status            text        NOT NULL DEFAULT 'open',
    resolution        text,
    resolution_note   text,
    resolved_by       uuid        REFERENCES users (id) ON DELETE RESTRICT,
    resolved_at       timestamptz,
    escalated_at      timestamptz,
    escalated_by      uuid        REFERENCES users (id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT reports_id_dormitory_key UNIQUE (id, dormitory_id),
    -- Target of the sanction backstop: a sanction citing a report must sanction its subject.
    CONSTRAINT reports_id_subject_key UNIQUE (id, subject_user_id),
    -- Reported content must belong to the report's dormitory (tenant backstop).
    -- If content is purged later, only the target column is cleared (PostgreSQL 15+).
    CONSTRAINT reports_target_comment_fkey FOREIGN KEY (target_comment_id, dormitory_id)
        REFERENCES comments (id, dormitory_id) ON DELETE SET NULL (target_comment_id),
    CONSTRAINT reports_target_photo_fkey FOREIGN KEY (target_photo_id, dormitory_id)
        REFERENCES meal_photos (id, dormitory_id) ON DELETE SET NULL (target_photo_id),
    CONSTRAINT reports_target_type_check
        CHECK (target_type IN ('comment', 'photo', 'user')),
    -- Only the target column that matches target_type may be set.
    CONSTRAINT reports_target_columns_check
        CHECK ((target_type = 'comment' AND target_photo_id IS NULL AND target_user_id IS NULL)
            OR (target_type = 'photo' AND target_comment_id IS NULL AND target_user_id IS NULL)
            OR (target_type = 'user' AND target_comment_id IS NULL AND target_photo_id IS NULL
                AND target_user_id IS NOT NULL)),
    -- Open reports always point at their target; content under review cannot be purged.
    CONSTRAINT reports_open_target_check
        CHECK (status <> 'open' OR num_nonnulls(target_comment_id, target_photo_id, target_user_id) = 1),
    -- Nobody reports themselves or their own content.
    CONSTRAINT reports_self_check
        CHECK (subject_user_id <> reporter_id),
    -- For user reports the subject is the reported user.
    CONSTRAINT reports_subject_check
        CHECK (target_type <> 'user' OR target_user_id = subject_user_id),
    CONSTRAINT reports_reason_check
        CHECK (reason IN ('spam', 'harassment', 'hate_speech', 'inappropriate', 'misinformation', 'other')),
    CONSTRAINT reports_details_check
        CHECK (details IS NULL OR char_length(details) <= 1000),
    CONSTRAINT reports_snapshot_check
        CHECK (content_snapshot IS NULL OR char_length(content_snapshot) <= 2000),
    CONSTRAINT reports_status_check
        CHECK (status IN ('open', 'resolved')),
    CONSTRAINT reports_resolution_check
        CHECK (resolution IS NULL OR resolution IN ('content_removed', 'user_sanctioned', 'no_action', 'dismissed')),
    CONSTRAINT reports_resolution_state_check
        CHECK ((status = 'open' AND resolution IS NULL AND resolved_at IS NULL AND resolved_by IS NULL)
            OR (status = 'resolved' AND resolution IS NOT NULL AND resolved_at IS NOT NULL)),
    CONSTRAINT reports_resolution_note_check
        CHECK (resolution_note IS NULL OR char_length(resolution_note) <= 1000),
    CONSTRAINT reports_escalation_check
        CHECK (escalated_by IS NULL OR escalated_at IS NOT NULL)
);

-- Moderator review queue of a dormitory (oldest open report first).
CREATE INDEX reports_queue_idx ON reports (dormitory_id, created_at, id) WHERE status = 'open';
CREATE INDEX reports_dormitory_idx ON reports (dormitory_id, created_at DESC, id DESC);
-- Global administrator queue of escalated reports.
CREATE INDEX reports_escalated_idx ON reports (escalated_at, id)
    WHERE status = 'open' AND escalated_at IS NOT NULL;
CREATE INDEX reports_target_comment_idx ON reports (target_comment_id) WHERE target_comment_id IS NOT NULL;
CREATE INDEX reports_target_photo_idx ON reports (target_photo_id) WHERE target_photo_id IS NOT NULL;
CREATE INDEX reports_target_user_idx ON reports (target_user_id) WHERE target_user_id IS NOT NULL;
CREATE INDEX reports_reporter_idx ON reports (reporter_id, created_at DESC);
-- Queue filtering by subject (exclude reports about the caller, route by the subject's role).
CREATE INDEX reports_subject_idx ON reports (dormitory_id, subject_user_id) WHERE status = 'open';
-- One open report per reporter and target (prevents report flooding).
CREATE UNIQUE INDEX reports_open_comment_key ON reports (reporter_id, target_comment_id)
    WHERE status = 'open' AND target_comment_id IS NOT NULL;
CREATE UNIQUE INDEX reports_open_photo_key ON reports (reporter_id, target_photo_id)
    WHERE status = 'open' AND target_photo_id IS NOT NULL;
CREATE UNIQUE INDEX reports_open_user_key ON reports (reporter_id, dormitory_id, target_user_id)
    WHERE status = 'open' AND target_user_id IS NOT NULL;

CREATE TABLE user_sanctions (
    id            uuid        PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    -- NULL = instance-wide sanction issued by a global administrator.
    dormitory_id  uuid        REFERENCES dormitories (id) ON DELETE RESTRICT,
    kind          text        NOT NULL,
    reason        text        NOT NULL,
    report_id     uuid        REFERENCES reports (id) ON DELETE RESTRICT,
    starts_at     timestamptz NOT NULL DEFAULT now(),
    -- NULL = until revoked.
    ends_at       timestamptz,
    created_by    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    revoked_at    timestamptz,
    revoked_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
    revoke_reason text,

    -- A dormitory sanction can only cite a report of the same dormitory (tenant backstop).
    -- For instance-wide sanctions (dormitory_id NULL) the simple report_id key above applies.
    CONSTRAINT user_sanctions_report_dormitory_fkey FOREIGN KEY (report_id, dormitory_id)
        REFERENCES reports (id, dormitory_id) ON DELETE RESTRICT,
    -- A sanction citing a report must target the subject of that report.
    CONSTRAINT user_sanctions_report_subject_fkey FOREIGN KEY (report_id, user_id)
        REFERENCES reports (id, subject_user_id) ON DELETE RESTRICT,
    CONSTRAINT user_sanctions_kind_check
        CHECK (kind IN ('suspension', 'ban')),
    CONSTRAINT user_sanctions_reason_check
        CHECK (char_length(reason) BETWEEN 1 AND 500),
    CONSTRAINT user_sanctions_period_check
        CHECK (ends_at IS NULL OR ends_at > starts_at),
    CONSTRAINT user_sanctions_self_check
        CHECK (created_by <> user_id),
    CONSTRAINT user_sanctions_revocation_check
        CHECK ((revoked_at IS NULL) = (revoked_by IS NULL)),
    CONSTRAINT user_sanctions_revoke_reason_check
        CHECK (revoke_reason IS NULL OR (revoked_at IS NOT NULL AND char_length(revoke_reason) BETWEEN 1 AND 500))
);

-- Active sanction lookup on every write request.
CREATE INDEX user_sanctions_user_active_idx ON user_sanctions (user_id, dormitory_id) WHERE revoked_at IS NULL;
CREATE INDEX user_sanctions_dormitory_idx ON user_sanctions (dormitory_id, created_at DESC)
    WHERE dormitory_id IS NOT NULL;
CREATE INDEX user_sanctions_report_idx ON user_sanctions (report_id) WHERE report_id IS NOT NULL;

COMMIT;
