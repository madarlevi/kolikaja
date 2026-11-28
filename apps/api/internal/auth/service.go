// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madarlevi/kolikaja/apps/api/internal/audit"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/validate"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// Service implements the authentication use cases.
type Service struct {
	pool            *pgxpool.Pool
	hasher          *Hasher
	tokens          *TokenIssuer
	refreshTokenTTL time.Duration
	sessionMaxAge   time.Duration
	now             func() time.Time
}

// NewService returns the authentication service.
func NewService(pool *pgxpool.Pool, hasher *Hasher, tokens *TokenIssuer, refreshTTL, sessionMaxAge time.Duration, now func() time.Time) *Service {
	return &Service{pool: pool, hasher: hasher, tokens: tokens, refreshTokenTTL: refreshTTL, sessionMaxAge: sessionMaxAge, now: now}
}

var (
	errInvalidCredentials = apierror.Unauthorized(apierror.InvalidCredentials, "E-mail address or password is wrong.")
	errInvalidToken       = apierror.Unauthorized(apierror.InvalidToken, "The token is invalid or expired.")
	errSessionRevoked     = apierror.Unauthorized(apierror.SessionRevoked, "The session has ended; log in again.")
	errAccountBanned      = apierror.ForbiddenCode(apierror.AccountBanned, "This account is banned.")
)

// Register creates an account. It does not log in.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (CurrentUser, error) {
	var v validate.Collector
	email := normalizeEmail(&v, "email", req.Email)
	validatePassword(&v, "password", req.Password, email)
	name := validate.Text(req.DisplayName)
	v.Length("display_name", name, 2, 40, false)
	locale := "hu"
	if req.Locale != nil {
		locale = *req.Locale
		v.OneOf("locale", locale, "hu", "en")
	}
	if err := v.Err(); err != nil {
		return CurrentUser{}, err
	}
	hash, err := s.hasher.Hash(ctx, req.Password)
	if err != nil {
		return CurrentUser{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return CurrentUser{}, err
	}
	now := s.now().UTC()
	if err := insertUser(ctx, s.pool, id, email, hash, name, locale, now); err != nil {
		if errors.Is(err, errEmailTaken) {
			// Accepted risk T-27: reveals that the address is registered (rate limited).
			return CurrentUser{}, apierror.ConflictCode(apierror.AlreadyExists, "An account with this e-mail address already exists.")
		}
		return CurrentUser{}, err
	}
	return currentUser(ctx, s.pool, id)
}

// LoginNative checks the credentials and opens a native session.
func (s *Service) LoginNative(ctx context.Context, req NativeLoginRequest, client ClientInfo) (NativeTokens, error) {
	var v validate.Collector
	if req.Email == "" || len(req.Email) > 254 {
		v.Add("email", "invalid_format")
	}
	if req.Password == "" || utf8.RuneCountInString(req.Password) > PasswordMaxLength {
		v.Add("password", "invalid_value")
	}
	var device *string
	if req.DeviceLabel != nil {
		d := validate.Text(*req.DeviceLabel)
		v.Length("device_label", d, 1, 100, false)
		device = &d
	}
	if err := v.Err(); err != nil {
		return NativeTokens{}, err
	}

	creds, err := credentialsByEmail(ctx, s.pool, strings.TrimSpace(req.Email))
	if errors.Is(err, errNoRows) {
		s.hasher.VerifyDummy(ctx, req.Password)
		return NativeTokens{}, errInvalidCredentials
	}
	if err != nil {
		return NativeTokens{}, err
	}
	ok, err := s.hasher.Verify(ctx, req.Password, creds.PasswordHash)
	if err != nil {
		return NativeTokens{}, fmt.Errorf("auth: verify: %w", err)
	}
	if !ok {
		return NativeTokens{}, errInvalidCredentials
	}
	ban, err := activeInstanceSanction(ctx, s.pool, creds.ID, "ban")
	if err != nil {
		return NativeTokens{}, err
	}
	if ban != nil {
		return NativeTokens{}, errAccountBanned
	}

	now := s.now().UTC()
	sessionID, err := uuid.NewV7()
	if err != nil {
		return NativeTokens{}, err
	}
	refresh, refreshHash, err := NewRefreshToken()
	if err != nil {
		return NativeTokens{}, err
	}
	sessionExpires := now.Add(s.sessionMaxAge)
	refreshExpires := minTime(now.Add(s.refreshTokenTTL), sessionExpires)
	ua := truncate(client.UserAgent, 512)
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertSession(ctx, tx, newSession{
			ID: sessionID, UserID: creds.ID, ClientType: "native", DeviceLabel: device,
			UserAgent: nonEmpty(ua), Network: client.Network, CreatedAt: now, ExpiresAt: sessionExpires,
		}); err != nil {
			return err
		}
		if err := insertRefreshToken(ctx, tx, sessionID, refreshHash, now, refreshExpires); err != nil {
			return err
		}
		return touchLastLogin(ctx, tx, creds.ID, now)
	})
	if err != nil {
		return NativeTokens{}, err
	}
	return s.tokensFor(ctx, creds.ID, sessionID, refresh, refreshExpires, sessionExpires, now)
}

// RefreshNative rotates a native refresh token (docs/ARCHITECTURE.md §4.3).
func (s *Service) RefreshNative(ctx context.Context, req NativeRefreshRequest, client ClientInfo) (NativeTokens, error) {
	if l := len(req.RefreshToken); l < 43 || l > 128 {
		return NativeTokens{}, errInvalidToken
	}
	now := s.now().UTC()
	var (
		userID, sessionID uuid.UUID
		sessionExpires    time.Time
		refreshExpires    time.Time
		newToken          string
		reused            bool
		denied            error
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		row, err := lockRefreshToken(ctx, tx, HashToken(req.RefreshToken))
		if errors.Is(err, errNoRows) {
			return errInvalidToken
		}
		if err != nil {
			return err
		}
		if row.ClientType != "native" {
			return errInvalidToken
		}
		if row.Used {
			reused = true
			return s.revokeForReuse(ctx, tx, row, client, now)
		}
		if !row.TokenExpiresAt.After(now) {
			return errInvalidToken
		}
		if row.SessionRevoked || !row.SessionExpiresAt.After(now) {
			return errSessionRevoked
		}
		// Deleted or banned accounts: end the session (committed) and refuse the refresh.
		if row.UserDeleted || row.UserBanned {
			reason, refusal := "account_deleted", errSessionRevoked
			if row.UserBanned {
				reason, refusal = "account_banned", errAccountBanned
			}
			if _, err := revokeSession(ctx, tx, row.SessionID, reason, now); err != nil {
				return err
			}
			denied = refusal
			return nil
		}
		used, err := markRefreshTokenUsed(ctx, tx, row.TokenID, now)
		if err != nil {
			return err
		}
		if !used {
			reused = true
			return s.revokeForReuse(ctx, tx, row, client, now)
		}
		token, hash, err := NewRefreshToken()
		if err != nil {
			return err
		}
		refreshExpires = minTime(now.Add(s.refreshTokenTTL), row.SessionExpiresAt)
		if err := insertRefreshToken(ctx, tx, row.SessionID, hash, now, refreshExpires); err != nil {
			return err
		}
		if err := touchSession(ctx, tx, row.SessionID, now); err != nil {
			return err
		}
		userID, sessionID, sessionExpires, newToken = row.UserID, row.SessionID, row.SessionExpiresAt, token
		return nil
	})
	if reused {
		// The transaction committed the revocation; report the revoked session.
		if err != nil {
			return NativeTokens{}, err
		}
		return NativeTokens{}, errSessionRevoked
	}
	if err == nil && denied != nil {
		return NativeTokens{}, denied
	}
	if err != nil {
		return NativeTokens{}, err
	}
	return s.tokensFor(ctx, userID, sessionID, newToken, refreshExpires, sessionExpires, now)
}

func (s *Service) revokeForReuse(ctx context.Context, tx pgx.Tx, row refreshRow, client ClientInfo, now time.Time) error {
	changed, err := revokeSession(ctx, tx, row.SessionID, "refresh_token_reuse", now)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	uid := row.UserID
	sid := row.SessionID
	return audit.Write(ctx, tx, audit.Entry{
		ActorType: audit.ActorAnonymous, Action: "auth.refresh_token_reused",
		TargetType: "session", TargetID: &sid, RequestID: client.RequestID,
		Metadata: map[string]any{"user_id": uid.String()},
	})
}

func (s *Service) tokensFor(ctx context.Context, userID, sessionID uuid.UUID, refresh string, refreshExpires, sessionExpires, now time.Time) (NativeTokens, error) {
	access, accessExpires, err := s.tokens.Issue(userID, sessionID, now)
	if err != nil {
		return NativeTokens{}, err
	}
	user, err := currentUser(ctx, s.pool, userID)
	if err != nil {
		return NativeTokens{}, err
	}
	return NativeTokens{
		TokenType:             "Bearer",
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExpires.UTC(),
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExpires.UTC(),
		Session:               SessionInfo{ID: sessionID, ExpiresAt: sessionExpires.UTC()},
		User:                  user,
	}, nil
}

// Authenticate verifies an access token and the state of its session on every request.
func (s *Service) Authenticate(ctx context.Context, raw string) (principal.Principal, error) {
	userID, sessionID, err := s.tokens.Parse(raw, s.now(), false)
	if err != nil {
		return principal.Principal{}, apierror.ErrUnauthenticated
	}
	st, err := sessionPrincipal(ctx, s.pool, sessionID)
	if errors.Is(err, errNoRows) {
		return principal.Principal{}, apierror.ErrUnauthenticated
	}
	if err != nil {
		return principal.Principal{}, err
	}
	if st.UserID != userID || !st.Active {
		return principal.Principal{}, errSessionRevoked
	}
	if st.Banned {
		return principal.Principal{}, errAccountBanned
	}
	return principal.Principal{UserID: userID, SessionID: sessionID, IsGlobalAdmin: st.IsGlobalAdmin}, nil
}

// Logout revokes the session of the token. An expired but validly signed token is
// accepted so that logging out works after the access token ran out.
func (s *Service) Logout(ctx context.Context, raw string) error {
	userID, sessionID, err := s.tokens.Parse(raw, s.now(), true)
	if err != nil {
		return apierror.ErrUnauthenticated
	}
	st, err := sessionPrincipal(ctx, s.pool, sessionID)
	if errors.Is(err, errNoRows) || (err == nil && st.UserID != userID) {
		return apierror.ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	_, err = revokeSession(ctx, s.pool, sessionID, "logout", s.now().UTC())
	return err
}

// Me returns the current user.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (CurrentUser, error) {
	u, err := currentUser(ctx, s.pool, userID)
	if errors.Is(err, errNoRows) {
		return u, apierror.ErrUnauthenticated
	}
	return u, err
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return strings.ToValidUTF8(s, "")
	}
	r := []rune(strings.ToValidUTF8(s, ""))
	return string(r[:maxRunes])
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
