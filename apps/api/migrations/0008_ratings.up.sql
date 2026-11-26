-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0008 ratings (up): numeric 1-5 meal ratings.
--
-- The API stores and returns only the number. Clients map 1-5 to the exact Hungarian
-- labels in docs/DECISIONS.md through their i18n resources.
-- Individual ratings are private to their author; other users see aggregates only.

BEGIN;

CREATE TABLE ratings (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    meal_id      uuid        NOT NULL,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    value        smallint    NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,

    -- The meal must belong to the same dormitory as the rating (tenant backstop).
    CONSTRAINT ratings_meal_fkey FOREIGN KEY (meal_id, dormitory_id)
        REFERENCES meals (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT ratings_value_check
        CHECK (value BETWEEN 1 AND 5)
);

-- One active rating per user per meal (SPEC section 3, Ratings).
-- INCLUDE (value) lets average, count and distribution use an index-only scan.
CREATE UNIQUE INDEX ratings_meal_user_active_key ON ratings (meal_id, user_id)
    INCLUDE (value) WHERE deleted_at IS NULL;
-- Rating history of a user (newest first, cursor pagination).
CREATE INDEX ratings_user_history_idx ON ratings (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- Dormitory statistics over a time range.
CREATE INDEX ratings_dormitory_created_idx ON ratings (dormitory_id, created_at)
    WHERE deleted_at IS NULL;

COMMIT;
