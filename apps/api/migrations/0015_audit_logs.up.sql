-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 madarlevi
--
-- 0015 audit_logs (up): append-only audit trail.
--
-- Moderator, administrator, authentication and API key events are written in the same
-- transaction as the change they describe. metadata must never contain passwords,
-- tokens, API keys, cookies or e-mail addresses. ip_network is truncated
-- (IPv4 /24, IPv6 /48). Triggers reject UPDATE and TRUNCATE, and DELETE of rows younger
-- than 30 days; older rows can be deleted by the retention purge (AUDIT_LOG_RETENTION_DAYS,
-- which the API requires to be at least 30). This guards against application bugs; it is not
-- a defence against a database superuser.
-- Referenced users, dormitories and API keys are never hard-deleted, so the foreign
-- keys use RESTRICT. (SET NULL would issue an UPDATE, which the trigger rejects.)

BEGIN;

CREATE TABLE audit_logs (
    id               uuid        PRIMARY KEY,
    occurred_at      timestamptz NOT NULL DEFAULT now(),
    actor_type       text        NOT NULL,
    actor_user_id    uuid        REFERENCES users (id) ON DELETE RESTRICT,
    actor_api_key_id uuid        REFERENCES api_keys (id) ON DELETE RESTRICT,
    dormitory_id     uuid        REFERENCES dormitories (id) ON DELETE RESTRICT,
    action           text        NOT NULL,
    target_type      text,
    target_id        uuid,
    request_id       text,
    ip_network       cidr,
    metadata         jsonb       NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT audit_logs_actor_type_check
        CHECK (actor_type IN ('user', 'api_key', 'system', 'anonymous')),
    CONSTRAINT audit_logs_actor_check
        CHECK ((actor_type = 'user' AND actor_user_id IS NOT NULL AND actor_api_key_id IS NULL)
            OR (actor_type = 'api_key' AND actor_api_key_id IS NOT NULL AND actor_user_id IS NULL)
            OR (actor_type IN ('system', 'anonymous') AND actor_user_id IS NULL AND actor_api_key_id IS NULL)),
    -- Dotted lower-case action names, for example 'comment.removed' or 'auth.refresh_token_reused'.
    CONSTRAINT audit_logs_action_check
        CHECK (char_length(action) BETWEEN 3 AND 64 AND action ~ '^[a-z][a-z_]*(\.[a-z][a-z_]*)+$'),
    CONSTRAINT audit_logs_target_type_check
        CHECK (target_type IS NULL OR char_length(target_type) BETWEEN 1 AND 32),
    CONSTRAINT audit_logs_request_id_check
        CHECK (request_id IS NULL OR char_length(request_id) BETWEEN 1 AND 64),
    CONSTRAINT audit_logs_metadata_check
        CHECK (jsonb_typeof(metadata) = 'object' AND octet_length(metadata::text) <= 8192)
);

-- Dormitory audit log (dormitory administrators).
CREATE INDEX audit_logs_dormitory_idx ON audit_logs (dormitory_id, occurred_at DESC, id DESC)
    WHERE dormitory_id IS NOT NULL;
-- Instance audit log (global administrators) and the retention purge.
CREATE INDEX audit_logs_occurred_idx ON audit_logs (occurred_at DESC, id DESC);
CREATE INDEX audit_logs_actor_user_idx ON audit_logs (actor_user_id, occurred_at DESC)
    WHERE actor_user_id IS NOT NULL;
CREATE INDEX audit_logs_actor_api_key_idx ON audit_logs (actor_api_key_id, occurred_at DESC)
    WHERE actor_api_key_id IS NOT NULL;
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id)
    WHERE target_id IS NOT NULL;

CREATE FUNCTION audit_logs_reject_change() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only; % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_reject_change();

CREATE TRIGGER audit_logs_no_truncate
    BEFORE TRUNCATE ON audit_logs
    FOR EACH STATEMENT EXECUTE FUNCTION audit_logs_reject_change();

-- Minimum retention of 30 days, independent of application configuration.
CREATE TRIGGER audit_logs_retention_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW
    WHEN (OLD.occurred_at > now() - interval '30 days')
    EXECUTE FUNCTION audit_logs_reject_change();

COMMIT;
