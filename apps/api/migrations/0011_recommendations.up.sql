-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0011 recommendations (up): "recommended" / "not recommended" flags on meals.
-- Kept separate from the numeric rating (SPEC section 3, Recommendations).
-- Withdrawing a recommendation deletes the row; no history is needed.

BEGIN;

CREATE TABLE recommendations (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    meal_id      uuid        NOT NULL,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    value        text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    -- The meal must belong to the same dormitory as the recommendation (tenant backstop).
    CONSTRAINT recommendations_meal_fkey FOREIGN KEY (meal_id, dormitory_id)
        REFERENCES meals (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT recommendations_value_check
        CHECK (value IN ('recommended', 'not_recommended')),
    -- One recommendation per user per meal. INCLUDE (value) serves the counts.
    CONSTRAINT recommendations_meal_user_key UNIQUE (meal_id, user_id) INCLUDE (value)
);

CREATE INDEX recommendations_user_idx ON recommendations (user_id, created_at DESC);
CREATE INDEX recommendations_dormitory_created_idx ON recommendations (dormitory_id, created_at);

COMMIT;
