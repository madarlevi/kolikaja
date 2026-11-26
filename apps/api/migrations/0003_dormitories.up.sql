-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0003 dormitories (up): the tenants of the platform and their settings.
-- Dormitories are soft-deleted (deleted_at). Every dormitory-scoped read joins
-- dormitories and filters on deleted_at IS NULL.

BEGIN;

CREATE TABLE dormitories (
    id                 uuid        PRIMARY KEY,
    slug               text        NOT NULL,
    name               text        NOT NULL,
    description        text,
    city               text,
    address            text,
    visibility         text        NOT NULL DEFAULT 'private',
    join_policy        text        NOT NULL DEFAULT 'invite_only',
    -- IANA time zone that decides what "today" and "this week" mean for menus.
    timezone           text        NOT NULL DEFAULT 'Europe/Budapest',
    comments_enabled   boolean     NOT NULL DEFAULT true,
    photos_enabled     boolean     NOT NULL DEFAULT true,
    -- Days after the menu date during which meals can be rated; NULL = no limit.
    rating_window_days integer,
    created_by         uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    deleted_by         uuid        REFERENCES users (id) ON DELETE RESTRICT,

    CONSTRAINT dormitories_slug_key UNIQUE (slug),
    CONSTRAINT dormitories_slug_check
        CHECK (char_length(slug) BETWEEN 3 AND 64 AND slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT dormitories_name_check
        CHECK (char_length(name) BETWEEN 2 AND 100),
    CONSTRAINT dormitories_description_check
        CHECK (description IS NULL OR char_length(description) <= 2000),
    CONSTRAINT dormitories_city_check
        CHECK (city IS NULL OR char_length(city) BETWEEN 1 AND 100),
    CONSTRAINT dormitories_address_check
        CHECK (address IS NULL OR char_length(address) BETWEEN 1 AND 200),
    CONSTRAINT dormitories_visibility_check
        CHECK (visibility IN ('public', 'private')),
    CONSTRAINT dormitories_join_policy_check
        CHECK (join_policy IN ('open', 'invite_only')),
    -- Private dormitories can only be joined with an invitation code.
    CONSTRAINT dormitories_private_invite_only_check
        CHECK (visibility = 'public' OR join_policy = 'invite_only'),
    CONSTRAINT dormitories_timezone_check
        CHECK (char_length(timezone) BETWEEN 1 AND 64),
    CONSTRAINT dormitories_rating_window_check
        CHECK (rating_window_days IS NULL OR rating_window_days BETWEEN 1 AND 365),
    CONSTRAINT dormitories_deletion_check
        CHECK (deleted_by IS NULL OR deleted_at IS NOT NULL)
);

-- Public directory: listing and search sorted by name.
CREATE INDEX dormitories_public_name_idx ON dormitories (lower(name), id)
    WHERE visibility = 'public' AND deleted_at IS NULL;
CREATE INDEX dormitories_created_by_idx ON dormitories (created_by);

COMMIT;
