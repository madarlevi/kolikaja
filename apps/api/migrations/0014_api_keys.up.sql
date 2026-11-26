-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0014 api_keys (up): server-to-server API keys (header X-API-Key).
--
-- Key format: kk_<key_prefix>_<secret>. key_prefix (12 base32 characters) is
-- public and used for lookup; key_hash is the SHA-256 of the full key. The full key is
-- shown once at creation. A key is either bound to one dormitory, or it is an
-- instance-level service key (dormitory_id NULL) that can only read the public
-- dormitory directory. API keys go through the same tenant checks as users.
-- last_used_at and use_count are flushed in batches by the API, not on every request.
-- The maximum lifetime (730 days) is enforced by the API.

BEGIN;

CREATE TABLE api_keys (
    id                    uuid        PRIMARY KEY,
    dormitory_id          uuid        REFERENCES dormitories (id) ON DELETE RESTRICT,
    name                  text        NOT NULL,
    key_prefix            text        NOT NULL,
    key_hash              bytea       NOT NULL,
    scopes                text[]      NOT NULL,
    rate_limit_per_minute integer     NOT NULL DEFAULT 60,
    created_by            uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    last_used_at          timestamptz,
    use_count             bigint      NOT NULL DEFAULT 0,
    revoked_at            timestamptz,
    revoked_by            uuid        REFERENCES users (id) ON DELETE RESTRICT,

    CONSTRAINT api_keys_key_prefix_key UNIQUE (key_prefix),
    CONSTRAINT api_keys_key_hash_key UNIQUE (key_hash),
    CONSTRAINT api_keys_key_prefix_check
        CHECK (key_prefix ~ '^[a-z2-7]{12}$'),
    CONSTRAINT api_keys_key_hash_check
        CHECK (octet_length(key_hash) = 32),
    CONSTRAINT api_keys_name_check
        CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT api_keys_scopes_check
        CHECK (cardinality(scopes) BETWEEN 1 AND 10 AND scopes <@ ARRAY[
            'dormitories:read', 'menus:read', 'menus:write', 'stats:read', 'announcements:write'
        ]::text[]),
    -- Instance-level service keys may only read the public dormitory directory.
    CONSTRAINT api_keys_instance_scopes_check
        CHECK (dormitory_id IS NOT NULL OR scopes <@ ARRAY['dormitories:read']::text[]),
    CONSTRAINT api_keys_rate_limit_check
        CHECK (rate_limit_per_minute BETWEEN 1 AND 1000),
    CONSTRAINT api_keys_expiry_check
        CHECK (expires_at > created_at),
    CONSTRAINT api_keys_use_count_check
        CHECK (use_count >= 0),
    CONSTRAINT api_keys_revocation_check
        CHECK (revoked_by IS NULL OR revoked_at IS NOT NULL)
);

CREATE INDEX api_keys_dormitory_idx ON api_keys (dormitory_id, created_at DESC) WHERE dormitory_id IS NOT NULL;
CREATE INDEX api_keys_instance_idx ON api_keys (created_at DESC) WHERE dormitory_id IS NULL;

COMMIT;
