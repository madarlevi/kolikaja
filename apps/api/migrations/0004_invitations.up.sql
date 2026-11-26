-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0004 invitations (up): invitation codes for joining a dormitory.
-- The full code is shown once at creation. Only its SHA-256 hash and the last four
-- characters (code_hint, for recognising a code in the admin list) are stored.

BEGIN;

CREATE TABLE invitations (
    id           uuid        PRIMARY KEY,
    dormitory_id uuid        NOT NULL REFERENCES dormitories (id) ON DELETE RESTRICT,
    code_hash    bytea       NOT NULL,
    code_hint    text        NOT NULL,
    label        text,
    -- NULL = unlimited uses until expiry or revocation.
    max_uses     integer,
    use_count    integer     NOT NULL DEFAULT 0,
    expires_at   timestamptz NOT NULL,
    created_by   uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    revoked_at   timestamptz,
    revoked_by   uuid        REFERENCES users (id) ON DELETE RESTRICT,

    -- Target of composite foreign keys from child tables (tenant backstop).
    CONSTRAINT invitations_id_dormitory_key UNIQUE (id, dormitory_id),
    CONSTRAINT invitations_code_hash_key UNIQUE (code_hash),
    CONSTRAINT invitations_code_hash_check
        CHECK (octet_length(code_hash) = 32),
    CONSTRAINT invitations_code_hint_check
        CHECK (char_length(code_hint) = 4),
    CONSTRAINT invitations_label_check
        CHECK (label IS NULL OR char_length(label) BETWEEN 1 AND 100),
    CONSTRAINT invitations_max_uses_check
        CHECK (max_uses IS NULL OR max_uses BETWEEN 1 AND 10000),
    CONSTRAINT invitations_use_count_check
        CHECK (use_count >= 0 AND (max_uses IS NULL OR use_count <= max_uses)),
    CONSTRAINT invitations_expiry_check
        CHECK (expires_at > created_at),
    CONSTRAINT invitations_revocation_check
        CHECK (revoked_by IS NULL OR revoked_at IS NOT NULL)
);

CREATE INDEX invitations_dormitory_idx ON invitations (dormitory_id, created_at DESC, id DESC);
CREATE INDEX invitations_created_by_idx ON invitations (created_by);

COMMIT;
