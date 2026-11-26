-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0002 user_sessions (up): login sessions, rotating refresh tokens and
-- administrator-issued password reset tokens.
--
-- A session is one refresh-token family. Every refresh marks the presented token as
-- used and inserts its successor. Presenting a used token again is reuse: the API
-- revokes the whole session (revoke_reason = 'refresh_token_reuse').
-- Tokens are stored only as SHA-256 hashes (32 bytes).

BEGIN;

CREATE TABLE user_sessions (
    id            uuid        PRIMARY KEY,
    user_id       uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_type   text        NOT NULL,
    device_label  text,
    user_agent    text,
    -- Truncated client network (IPv4 /24, IPv6 /48), never the full address.
    ip_network    cidr,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_used_at  timestamptz NOT NULL DEFAULT now(),
    -- Absolute session lifetime; refresh tokens never outlive it.
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    revoke_reason text,

    CONSTRAINT user_sessions_client_type_check
        CHECK (client_type IN ('web', 'native')),
    CONSTRAINT user_sessions_device_label_check
        CHECK (device_label IS NULL OR char_length(device_label) BETWEEN 1 AND 100),
    CONSTRAINT user_sessions_user_agent_check
        CHECK (user_agent IS NULL OR char_length(user_agent) <= 512),
    CONSTRAINT user_sessions_expiry_check
        CHECK (expires_at > created_at),
    CONSTRAINT user_sessions_revocation_check
        CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL)),
    CONSTRAINT user_sessions_revoke_reason_check
        CHECK (revoke_reason IS NULL OR revoke_reason IN (
            'logout', 'user_revoked', 'password_changed', 'password_reset', 'refresh_token_reuse',
            'account_deleted', 'account_banned', 'admin_revoked'))
);

CREATE INDEX user_sessions_user_active_idx ON user_sessions (user_id, created_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX user_sessions_expires_at_idx ON user_sessions (expires_at);

CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY,
    session_id uuid        NOT NULL REFERENCES user_sessions (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,

    CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT refresh_tokens_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT refresh_tokens_expiry_check CHECK (expires_at > created_at)
);

-- Rotation backstop: a session has at most one unused refresh token at any time,
-- so two concurrent refreshes with the same token cannot both succeed.
CREATE UNIQUE INDEX refresh_tokens_session_active_key ON refresh_tokens (session_id) WHERE used_at IS NULL;
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens (session_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

-- One-time tokens that a global administrator hands to a user out of band
-- (there is no e-mail delivery in this version). The maximum lifetime is enforced by the API.
CREATE TABLE password_reset_tokens (
    id          uuid        PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL,
    created_by  uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    -- Set when the token is used or superseded by a newer token.
    consumed_at timestamptz,

    CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash),
    CONSTRAINT password_reset_tokens_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT password_reset_tokens_expiry_check CHECK (expires_at > created_at)
);

-- At most one outstanding reset token per user.
CREATE UNIQUE INDEX password_reset_tokens_user_active_key ON password_reset_tokens (user_id) WHERE consumed_at IS NULL;

COMMIT;
