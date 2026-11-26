-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0010 photos (up): meal photos and profile pictures (avatars).
--
-- Upload flow (docs/ARCHITECTURE.md, "Photo pipeline"): the API creates a row in
-- 'pending_upload' and returns a presigned POST policy for a server-generated object
-- key. After the client uploads, the API validates magic bytes and dimensions,
-- re-encodes the image without metadata (EXIF), writes the variants and marks the row
-- 'ready'. Object keys are derived from IDs and never come from the client.

BEGIN;

CREATE TABLE meal_photos (
    id                    uuid        PRIMARY KEY,
    dormitory_id          uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    meal_id               uuid        NOT NULL,
    user_id               uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status                text        NOT NULL DEFAULT 'pending_upload',
    declared_content_type text        NOT NULL,
    declared_size_bytes   bigint      NOT NULL,
    upload_expires_at     timestamptz NOT NULL,
    width                 integer,
    height                integer,
    stored_size_bytes     bigint,
    rejection_reason      text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    processed_at          timestamptz,
    deleted_at            timestamptz,
    deleted_by            uuid        REFERENCES users (id) ON DELETE RESTRICT,
    deletion_reason       text,

    CONSTRAINT meal_photos_id_dormitory_key UNIQUE (id, dormitory_id),
    -- The meal must belong to the same dormitory as the photo (tenant backstop).
    CONSTRAINT meal_photos_meal_fkey FOREIGN KEY (meal_id, dormitory_id)
        REFERENCES meals (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT meal_photos_status_check
        CHECK (status IN ('pending_upload', 'ready', 'rejected', 'removed')),
    CONSTRAINT meal_photos_content_type_check
        CHECK (declared_content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    -- Hard upper bound (25 MiB). The effective, lower limit is configured in the API.
    CONSTRAINT meal_photos_declared_size_check
        CHECK (declared_size_bytes BETWEEN 1 AND 26214400),
    CONSTRAINT meal_photos_dimensions_check
        CHECK ((width IS NULL OR width BETWEEN 1 AND 20000) AND (height IS NULL OR height BETWEEN 1 AND 20000)),
    CONSTRAINT meal_photos_ready_check
        CHECK (status <> 'ready' OR (processed_at IS NOT NULL AND width IS NOT NULL
                                     AND height IS NOT NULL AND stored_size_bytes IS NOT NULL)),
    CONSTRAINT meal_photos_rejection_check
        CHECK ((status = 'rejected') = (rejection_reason IS NOT NULL)),
    CONSTRAINT meal_photos_rejection_reason_check
        CHECK (rejection_reason IS NULL OR rejection_reason IN (
            'missing_upload', 'too_large', 'unsupported_type', 'invalid_image', 'dimensions_exceeded')),
    CONSTRAINT meal_photos_removal_check
        CHECK ((status = 'removed') = (deleted_at IS NOT NULL)),
    CONSTRAINT meal_photos_deletion_check
        CHECK ((deleted_at IS NULL) = (deletion_reason IS NULL)),
    CONSTRAINT meal_photos_deletion_reason_check
        CHECK (deletion_reason IS NULL OR deletion_reason IN ('author', 'moderator', 'account_deleted')),
    CONSTRAINT meal_photos_moderator_deletion_check
        CHECK (deletion_reason IS DISTINCT FROM 'moderator' OR deleted_by IS NOT NULL)
);

-- Photos of one meal and the dormitory gallery (newest first, cursor pagination).
CREATE INDEX meal_photos_meal_ready_idx ON meal_photos (meal_id, created_at DESC, id DESC)
    WHERE status = 'ready';
CREATE INDEX meal_photos_dormitory_ready_idx ON meal_photos (dormitory_id, created_at DESC, id DESC)
    WHERE status = 'ready';
-- Cleanup of abandoned uploads.
CREATE INDEX meal_photos_pending_idx ON meal_photos (upload_expires_at)
    WHERE status = 'pending_upload';
CREATE INDEX meal_photos_user_idx ON meal_photos (user_id, created_at DESC);

CREATE TABLE user_avatars (
    id                    uuid        PRIMARY KEY,
    user_id               uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status                text        NOT NULL DEFAULT 'pending_upload',
    declared_content_type text        NOT NULL,
    declared_size_bytes   bigint      NOT NULL,
    upload_expires_at     timestamptz NOT NULL,
    stored_size_bytes     bigint,
    rejection_reason      text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    processed_at          timestamptz,
    removed_at            timestamptz,

    CONSTRAINT user_avatars_status_check
        CHECK (status IN ('pending_upload', 'ready', 'rejected', 'removed')),
    CONSTRAINT user_avatars_content_type_check
        CHECK (declared_content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT user_avatars_declared_size_check
        CHECK (declared_size_bytes BETWEEN 1 AND 26214400),
    CONSTRAINT user_avatars_ready_check
        CHECK (status <> 'ready' OR (processed_at IS NOT NULL AND stored_size_bytes IS NOT NULL)),
    CONSTRAINT user_avatars_rejection_check
        CHECK ((status = 'rejected') = (rejection_reason IS NOT NULL)),
    CONSTRAINT user_avatars_rejection_reason_check
        CHECK (rejection_reason IS NULL OR rejection_reason IN (
            'missing_upload', 'too_large', 'unsupported_type', 'invalid_image', 'dimensions_exceeded')),
    CONSTRAINT user_avatars_removal_check
        CHECK ((status = 'removed') = (removed_at IS NOT NULL))
);

-- At most one current avatar per user.
CREATE UNIQUE INDEX user_avatars_current_key ON user_avatars (user_id) WHERE status = 'ready';
CREATE INDEX user_avatars_pending_idx ON user_avatars (upload_expires_at) WHERE status = 'pending_upload';
CREATE INDEX user_avatars_user_idx ON user_avatars (user_id, created_at DESC);

COMMIT;
