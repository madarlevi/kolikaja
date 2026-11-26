-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0006 menus (up): dormitory-defined meal categories and daily menus.
--
-- A menu is one service (breakfast, lunch, dinner or all day) on one date in one
-- dormitory. "Weekly menus" are the daily menus of one ISO week; there is no separate
-- weekly entity. Only published menus are visible to ordinary members.

BEGIN;

CREATE TABLE meal_categories (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    name         text        NOT NULL,
    position     integer     NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,

    CONSTRAINT meal_categories_id_dormitory_key UNIQUE (id, dormitory_id),
    CONSTRAINT meal_categories_name_check
        CHECK (char_length(name) BETWEEN 1 AND 50),
    CONSTRAINT meal_categories_position_check
        CHECK (position BETWEEN 0 AND 1000)
);

CREATE UNIQUE INDEX meal_categories_name_key ON meal_categories (dormitory_id, lower(name))
    WHERE deleted_at IS NULL;
CREATE INDEX meal_categories_dormitory_idx ON meal_categories (dormitory_id, position)
    WHERE deleted_at IS NULL;

CREATE TABLE menus (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    menu_date    date        NOT NULL,
    meal_period  text        NOT NULL DEFAULT 'lunch',
    title        text,
    note         text,
    status       text        NOT NULL DEFAULT 'draft',
    published_at timestamptz,
    -- NULL when the change was made with an API key; the audit log records the key.
    created_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
    updated_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,

    CONSTRAINT menus_id_dormitory_key UNIQUE (id, dormitory_id),
    CONSTRAINT menus_meal_period_check
        CHECK (meal_period IN ('breakfast', 'lunch', 'dinner', 'all_day')),
    CONSTRAINT menus_title_check
        CHECK (title IS NULL OR char_length(title) BETWEEN 1 AND 100),
    CONSTRAINT menus_note_check
        CHECK (note IS NULL OR char_length(note) <= 1000),
    CONSTRAINT menus_status_check
        CHECK (status IN ('draft', 'published')),
    CONSTRAINT menus_published_check
        CHECK ((status = 'published') = (published_at IS NOT NULL)),
    CONSTRAINT menus_date_check
        CHECK (menu_date BETWEEN DATE '2000-01-01' AND DATE '2100-12-31')
);

-- One menu per dormitory, date and meal period. Also serves date-range reads
-- (today, this week) for a dormitory.
CREATE UNIQUE INDEX menus_dormitory_date_period_key ON menus (dormitory_id, menu_date, meal_period)
    WHERE deleted_at IS NULL;

COMMIT;
