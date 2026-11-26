-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0007 meals (up): individual meals of a menu.
--
-- Allergen codes follow the 14 allergen groups listed in Annex II of
-- Regulation (EU) No 1169/2011. NULL means "not provided"; an empty array means
-- the provider declared that the meal contains none of them. Clients must show
-- that allergen data comes from the dormitory and has to be verified with the provider.

BEGIN;

CREATE TABLE meals (
    id            uuid        PRIMARY KEY,
    dormitory_id  uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    menu_id       uuid        NOT NULL,
    category_id   uuid,
    name          text        NOT NULL,
    description   text,
    allergens     text[],
    allergen_note text,
    position      integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,

    CONSTRAINT meals_id_dormitory_key UNIQUE (id, dormitory_id),
    -- Menu and category must belong to the same dormitory as the meal (tenant backstop).
    CONSTRAINT meals_menu_fkey FOREIGN KEY (menu_id, dormitory_id)
        REFERENCES menus (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT meals_category_fkey FOREIGN KEY (category_id, dormitory_id)
        REFERENCES meal_categories (id, dormitory_id) ON DELETE RESTRICT,
    CONSTRAINT meals_name_check
        CHECK (char_length(name) BETWEEN 1 AND 150),
    CONSTRAINT meals_description_check
        CHECK (description IS NULL OR char_length(description) <= 1000),
    CONSTRAINT meals_allergens_check
        CHECK (allergens IS NULL OR (
            cardinality(allergens) <= 14 AND allergens <@ ARRAY[
                'gluten', 'crustaceans', 'eggs', 'fish', 'peanuts', 'soybeans', 'milk',
                'tree_nuts', 'celery', 'mustard', 'sesame', 'sulphites', 'lupin', 'molluscs'
            ]::text[])),
    CONSTRAINT meals_allergen_note_check
        CHECK (allergen_note IS NULL OR char_length(allergen_note) <= 300),
    CONSTRAINT meals_position_check
        CHECK (position BETWEEN 0 AND 1000)
);

CREATE INDEX meals_menu_idx ON meals (menu_id, position) WHERE deleted_at IS NULL;
CREATE INDEX meals_dormitory_idx ON meals (dormitory_id);
CREATE INDEX meals_category_idx ON meals (category_id) WHERE category_id IS NOT NULL;

COMMIT;
