-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0009 comments (up): comments on meals.
--
-- Deletion is soft (deleted_at). When the author deletes a comment or the account is
-- deleted, the API also clears body; a moderator removal keeps body for appeals until
-- the retention purge. Reports keep their own content_snapshot.

BEGIN;

CREATE TABLE comments (
    id              uuid        PRIMARY KEY,
    dormitory_id    uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    meal_id         uuid        NOT NULL,
    user_id         uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    body            text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    edited_at       timestamptz,
    deleted_at      timestamptz,
    deleted_by      uuid        REFERENCES users (id) ON DELETE RESTRICT,
    deletion_reason text,

    CONSTRAINT comments_id_dormitory_key UNIQUE (id, dormitory_id),
    -- The meal must belong to the same dormitory as the comment (tenant backstop).
    CONSTRAINT comments_meal_fkey FOREIGN KEY (meal_id, dormitory_id)
        REFERENCES meals (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT comments_body_check
        CHECK (body IS NULL OR char_length(body) BETWEEN 1 AND 2000),
    -- Only deleted comments may have their body cleared.
    CONSTRAINT comments_body_present_check
        CHECK (deleted_at IS NOT NULL OR body IS NOT NULL),
    CONSTRAINT comments_deletion_check
        CHECK ((deleted_at IS NULL) = (deletion_reason IS NULL)),
    CONSTRAINT comments_deletion_reason_check
        CHECK (deletion_reason IS NULL OR deletion_reason IN ('author', 'moderator', 'account_deleted')),
    CONSTRAINT comments_moderator_deletion_check
        CHECK (deletion_reason IS DISTINCT FROM 'moderator' OR deleted_by IS NOT NULL)
);

-- Comment pages of a meal (newest first, cursor pagination).
CREATE INDEX comments_meal_page_idx ON comments (meal_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX comments_user_idx ON comments (user_id, created_at DESC, id DESC);
CREATE INDEX comments_dormitory_created_idx ON comments (dormitory_id, created_at)
    WHERE deleted_at IS NULL;

COMMIT;
