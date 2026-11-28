// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/validate"
)

var (
	keyA = []byte("0123456789abcdef0123456789abcdef-a")
	keyB = []byte("0123456789abcdef0123456789abcdef-b")
)

func TestTokens(t *testing.T) {
	now := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	uid, sid := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	iss := NewTokenIssuer(keyA, nil, "https://kolikaja.example", 10*time.Minute)
	raw, exp, err := iss.Issue(uid, sid, now)
	if err != nil || !exp.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("issue: %v %v", exp, err)
	}
	gotU, gotS, err := iss.Parse(raw, now.Add(time.Minute), false)
	if err != nil || gotU != uid || gotS != sid {
		t.Fatalf("parse: %v %v %v", gotU, gotS, err)
	}
	if _, _, err := iss.Parse(raw, now.Add(11*time.Minute), false); err == nil {
		t.Error("expired token accepted")
	}
	if _, _, err := iss.Parse(raw, now.Add(11*time.Minute), true); err != nil {
		t.Errorf("expired token rejected for logout: %v", err)
	}
	if _, _, err := iss.Parse(raw, now.Add(-time.Hour), true); err == nil {
		t.Error("token issued in the future accepted")
	}

	// Key rotation: the previous key still verifies, an unknown key does not.
	rotated := NewTokenIssuer(keyB, keyA, "https://kolikaja.example", 10*time.Minute)
	if _, _, err := rotated.Parse(raw, now, false); err != nil {
		t.Errorf("token signed with the previous key rejected: %v", err)
	}
	other := NewTokenIssuer(keyB, nil, "https://kolikaja.example", 10*time.Minute)
	if _, _, err := other.Parse(raw, now, false); err == nil {
		t.Error("token with a foreign key accepted")
	}
	otherAud := NewTokenIssuer(keyA, nil, "https://other.example", 10*time.Minute)
	if _, _, err := otherAud.Parse(raw, now, false); err == nil {
		t.Error("token for another instance accepted")
	}
	if _, _, err := otherAud.Parse(raw, now, true); err == nil {
		t.Error("token for another instance accepted for logout")
	}

	// Wrong algorithm and wrong type.
	claims := Claims{SessionID: sid.String(), Type: "access", RegisteredClaims: jwt.RegisteredClaims{
		Subject: uid.String(), Issuer: "https://kolikaja.example", Audience: jwt.ClaimStrings{"https://kolikaja.example"},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	}}
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(keyA)
	if _, _, err := iss.Parse(hs512, now, false); err == nil {
		t.Error("HS512 token accepted")
	}
	claims.Type = "refresh"
	wrongType, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(keyA)
	if _, _, err := iss.Parse(wrongType, now, false); err == nil {
		t.Error("token of another type accepted")
	}
}

func TestRefreshToken(t *testing.T) {
	a, ha, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewRefreshToken()
	if a == b || len(a) != 43 || len(ha) != 32 || string(HashToken(a)) != string(ha) {
		t.Fatalf("refresh token: %q %d", a, len(ha))
	}
}

func TestArgon2(t *testing.T) {
	h, err := NewHasher(2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	enc, err := h.Hash(ctx, "correct horse battery staple")
	if err != nil || !strings.HasPrefix(enc, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q %v", enc, err)
	}
	enc2, _ := h.Hash(ctx, "correct horse battery staple")
	if enc == enc2 {
		t.Error("salt is not random")
	}
	if ok, err := h.Verify(ctx, "correct horse battery staple", enc); !ok || err != nil {
		t.Errorf("verify: %v %v", ok, err)
	}
	if ok, _ := h.Verify(ctx, "wrong", enc); ok {
		t.Error("wrong password verified")
	}
	for _, bad := range []string{
		"", "$argon2i$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=19$m=99999999,t=2,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5", // memory bomb
		"$argon2id$v=19$m=19456,t=1000,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=19$m=19456,t=2,p=1$!!$a2V5a2V5a2V5a2V5a2V5",
	} {
		if _, err := h.Verify(ctx, "x", bad); err == nil {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	h2, _ := NewHasher(1)
	h2.slots <- struct{}{} // occupy the only slot
	if _, err := h2.Hash(canceled, "x"); err == nil {
		t.Error("hash did not respect cancellation while waiting for a slot")
	}
}

func TestPasswordRules(t *testing.T) {
	cases := map[string]string{
		"short":                        "too_short",
		strings.Repeat("a", 129):       "too_long",
		"Password1234":                 "too_common",
		"user@example.test":            "too_common", // equal to the e-mail address
		"correct horse battery staple": "",
	}
	for pw, want := range cases {
		var v validate.Collector
		validatePassword(&v, "password", pw, "user@example.test")
		if got := reason(v.Err()); got != want {
			t.Errorf("%q: reason %q, want %q", pw, got, want)
		}
	}
	var v validate.Collector
	if got := normalizeEmail(&v, "email", "  Anna@Example.TEST "); got != "Anna@Example.TEST" || v.Err() != nil {
		t.Errorf("normalizeEmail = %q, %v", got, v.Err())
	}
	for _, bad := range []string{"", "no-at-sign", "Anna <anna@example.test>", strings.Repeat("a", 250) + "@example.test"} {
		var v validate.Collector
		normalizeEmail(&v, "email", bad)
		if reason(v.Err()) != "invalid_format" {
			t.Errorf("e-mail %q accepted", bad)
		}
	}
}

func reason(err error) string {
	var e *apierror.Error
	if !errors.As(err, &e) || len(e.Details) == 0 {
		return ""
	}
	return e.Details[0].Reason
}
