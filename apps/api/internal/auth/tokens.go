// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Access tokens: HS256 JWT with sub, sid, iat, exp, iss, aud and typ=access
// (docs/ARCHITECTURE.md §4.2). The algorithm is pinned; "none" and all others are rejected.

const (
	keyIDCurrent = "current"
	tokenType    = "access"
)

// Claims are the access-token claims.
type Claims struct {
	SessionID string `json:"sid"`
	Type      string `json:"typ"`
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies access tokens.
type TokenIssuer struct {
	current, previous []byte
	issuer, audience  string
	ttl               time.Duration
}

// NewTokenIssuer returns an issuer; previous may be nil.
func NewTokenIssuer(current, previous []byte, issuer string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{current: current, previous: previous, issuer: issuer, audience: issuer, ttl: ttl}
}

// Issue returns a signed access token for the session and its expiry.
func (t *TokenIssuer) Issue(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error) {
	exp := now.Add(t.ttl)
	claims := Claims{
		SessionID: sessionID.String(),
		Type:      tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{t.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = keyIDCurrent
	signed, err := tok.SignedString(t.current)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("tokens: sign: %w", err)
	}
	return signed, exp, nil
}

// ErrInvalidToken is returned for any token that does not verify.
var ErrInvalidToken = errors.New("tokens: invalid access token")

// Parse verifies the token and returns its user and session IDs. With allowExpired, an
// expired token whose signature, issuer, audience and type are valid is accepted (used
// only by logout, docs/ARCHITECTURE.md §4.2).
func (t *TokenIssuer) Parse(raw string, now time.Time, allowExpired bool) (userID, sessionID uuid.UUID, err error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(t.audience),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	}
	if allowExpired {
		opts = append(opts, jwt.WithoutClaimsValidation())
	}
	var claims Claims
	_, err = jwt.ParseWithClaims(raw, &claims, t.key, opts...)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	if allowExpired {
		// Claims validation was skipped: check everything except the expiry by hand.
		if claims.Issuer != t.issuer || !audienceContains(claims.Audience, t.audience) ||
			claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.IssuedAt.After(now.Add(time.Minute)) {
			return uuid.Nil, uuid.Nil, ErrInvalidToken
		}
	}
	if claims.Type != tokenType {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	if userID, err = uuid.Parse(claims.Subject); err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	if sessionID, err = uuid.Parse(claims.SessionID); err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return userID, sessionID, nil
}

// key returns the verification keys. The kid header is not trusted for key selection:
// during a rotation both the current and the previous key are tried.
func (t *TokenIssuer) key(*jwt.Token) (any, error) {
	if len(t.previous) == 0 {
		return t.current, nil
	}
	return jwt.VerificationKeySet{Keys: []jwt.VerificationKey{t.current, t.previous}}, nil
}

func audienceContains(aud jwt.ClaimStrings, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}

// NewRefreshToken returns a random opaque refresh token (32 bytes, base64url) and its
// SHA-256 hash, which is the only form stored.
func NewRefreshToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("tokens: random: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the SHA-256 of an opaque token.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
