// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/db"
)

var (
	errEmailTaken = errors.New("auth: e-mail already registered")
	errNoRows     = errors.New("auth: not found")
)

type userCredentials struct {
	ID           uuid.UUID
	PasswordHash string
}

func insertUser(ctx context.Context, q db.Querier, id uuid.UUID, email, hash, displayName, locale string, now time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, locale, password_changed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $6)`,
		id, email, hash, displayName, locale, now)
	if db.Is(err, db.UniqueViolation, "users_email_key") {
		return errEmailTaken
	}
	if err != nil {
		return fmt.Errorf("auth: insert user: %w", err)
	}
	return nil
}

func credentialsByEmail(ctx context.Context, q db.Querier, email string) (userCredentials, error) {
	var c userCredentials
	err := q.QueryRow(ctx, `
		SELECT id, password_hash FROM users
		WHERE lower(email) = lower($1) AND deleted_at IS NULL`, email).Scan(&c.ID, &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, errNoRows
	}
	if err != nil {
		return c, fmt.Errorf("auth: credentials: %w", err)
	}
	return c, nil
}

func currentUser(ctx context.Context, q db.Querier, id uuid.UUID) (CurrentUser, error) {
	var u CurrentUser
	err := q.QueryRow(ctx, `
		SELECT id, email, display_name, locale, is_global_admin, notify_comments, notify_announcements, created_at, password_changed_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.Locale, &u.IsGlobalAdmin,
		&u.NotificationSettings.Comments, &u.NotificationSettings.Announcements, &u.CreatedAt, &u.PasswordChangedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errNoRows
	}
	if err != nil {
		return u, fmt.Errorf("auth: current user: %w", err)
	}
	u.CreatedAt = u.CreatedAt.UTC()
	if u.PasswordChangedAt != nil {
		t := u.PasswordChangedAt.UTC()
		u.PasswordChangedAt = &t
	}
	s, err := activeInstanceSanction(ctx, q, id, "suspension")
	if err != nil {
		return u, err
	}
	u.Restriction = s
	return u, nil
}

// activeInstanceSanction returns the active instance-wide sanction of the given kind, if any.
func activeInstanceSanction(ctx context.Context, q db.Querier, userID uuid.UUID, kind string) (*SanctionSummary, error) {
	var s SanctionSummary
	err := q.QueryRow(ctx, `
		SELECT id, kind, starts_at, ends_at FROM user_sanctions
		WHERE user_id = $1 AND dormitory_id IS NULL AND kind = $2 AND revoked_at IS NULL
		  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
		ORDER BY starts_at DESC LIMIT 1`, userID, kind).Scan(&s.ID, &s.Kind, &s.StartsAt, &s.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: sanction: %w", err)
	}
	s.StartsAt = s.StartsAt.UTC()
	if s.EndsAt != nil {
		t := s.EndsAt.UTC()
		s.EndsAt = &t
	}
	return &s, nil
}

type newSession struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ClientType  string
	DeviceLabel *string
	UserAgent   *string
	Network     *string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

func insertSession(ctx context.Context, q db.Querier, s newSession) error {
	_, err := q.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, client_type, device_label, user_agent, ip_network, created_at, last_used_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6::cidr, $7, $7, $8)`,
		s.ID, s.UserID, s.ClientType, s.DeviceLabel, s.UserAgent, s.Network, s.CreatedAt, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("auth: insert session: %w", err)
	}
	return nil
}

func insertRefreshToken(ctx context.Context, q db.Querier, sessionID uuid.UUID, hash []byte, now, expires time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO refresh_tokens (id, session_id, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, id, sessionID, hash, now, expires)
	if err != nil {
		return fmt.Errorf("auth: insert refresh token: %w", err)
	}
	return nil
}

type refreshRow struct {
	TokenID          uuid.UUID
	TokenExpiresAt   time.Time
	Used             bool
	SessionID        uuid.UUID
	UserID           uuid.UUID
	ClientType       string
	SessionExpiresAt time.Time
	SessionRevoked   bool
	UserDeleted      bool
	UserBanned       bool
}

// lockRefreshToken loads a refresh token with its session and locks the session row, so
// concurrent refreshes of one session are serialized.
func lockRefreshToken(ctx context.Context, q db.Querier, hash []byte) (refreshRow, error) {
	var r refreshRow
	err := q.QueryRow(ctx, `
		SELECT t.id, t.expires_at, t.used_at IS NOT NULL, s.id, s.user_id, s.client_type, s.expires_at, s.revoked_at IS NOT NULL,
		       u.deleted_at IS NOT NULL,
		       EXISTS (SELECT 1 FROM user_sanctions b
		               WHERE b.user_id = u.id AND b.dormitory_id IS NULL AND b.kind = 'ban' AND b.revoked_at IS NULL
		                 AND b.starts_at <= now() AND (b.ends_at IS NULL OR b.ends_at > now()))
		FROM refresh_tokens t
		JOIN user_sessions s ON s.id = t.session_id
		JOIN users u ON u.id = s.user_id
		WHERE t.token_hash = $1
		FOR UPDATE OF s`, hash).Scan(
		&r.TokenID, &r.TokenExpiresAt, &r.Used, &r.SessionID, &r.UserID, &r.ClientType, &r.SessionExpiresAt, &r.SessionRevoked,
		&r.UserDeleted, &r.UserBanned)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, errNoRows
	}
	if err != nil {
		return r, fmt.Errorf("auth: load refresh token: %w", err)
	}
	return r, nil
}

// markRefreshTokenUsed marks the token used if it is still unused and unexpired; it
// reports false when another request used it first.
func markRefreshTokenUsed(ctx context.Context, q db.Querier, tokenID uuid.UUID, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE refresh_tokens SET used_at = $2
		WHERE id = $1 AND used_at IS NULL AND expires_at > $2`, tokenID, now)
	if err != nil {
		return false, fmt.Errorf("auth: use refresh token: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func touchSession(ctx context.Context, q db.Querier, sessionID uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE user_sessions SET last_used_at = $2 WHERE id = $1`, sessionID, now)
	if err != nil {
		return fmt.Errorf("auth: touch session: %w", err)
	}
	return nil
}

// revokeSession revokes an active session; it reports whether a row changed.
func revokeSession(ctx context.Context, q db.Querier, sessionID uuid.UUID, reason string, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE user_sessions SET revoked_at = $2, revoke_reason = $3
		WHERE id = $1 AND revoked_at IS NULL`, sessionID, now, reason)
	if err != nil {
		return false, fmt.Errorf("auth: revoke session: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func touchLastLogin(ctx context.Context, q db.Querier, userID uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, userID, now)
	if err != nil {
		return fmt.Errorf("auth: last login: %w", err)
	}
	return nil
}

type sessionState struct {
	UserID        uuid.UUID
	IsGlobalAdmin bool
	Active        bool
	Banned        bool
}

// sessionPrincipal loads the state needed to authenticate a request.
func sessionPrincipal(ctx context.Context, q db.Querier, sessionID uuid.UUID) (sessionState, error) {
	var s sessionState
	err := q.QueryRow(ctx, `
		SELECT s.user_id, u.is_global_admin,
		       s.revoked_at IS NULL AND s.expires_at > now() AND u.deleted_at IS NULL,
		       EXISTS (SELECT 1 FROM user_sanctions x
		               WHERE x.user_id = u.id AND x.dormitory_id IS NULL AND x.kind = 'ban' AND x.revoked_at IS NULL
		                 AND x.starts_at <= now() AND (x.ends_at IS NULL OR x.ends_at > now()))
		FROM user_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1`, sessionID).Scan(&s.UserID, &s.IsGlobalAdmin, &s.Active, &s.Banned)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, errNoRows
	}
	if err != nil {
		return s, fmt.Errorf("auth: session principal: %w", err)
	}
	return s, nil
}
