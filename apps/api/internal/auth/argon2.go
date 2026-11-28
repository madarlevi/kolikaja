// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (docs/ARCHITECTURE.md §4.1, OWASP minimum): 19 MiB, 2 iterations,
// 1 lane, 16-byte salt, 32-byte key. Hashes with other parameters are still verified.
const (
	argonMemoryKiB = 19456
	argonTime      = 2
	argonThreads   = 1
	argonSaltLen   = 16
	argonKeyLen    = 32
)

// Hasher hashes and verifies passwords and bounds the memory used by concurrent hashing.
type Hasher struct {
	slots chan struct{}
	dummy string
}

// NewHasher returns a hasher that runs at most maxConcurrent hash operations at once.
func NewHasher(maxConcurrent int) (*Hasher, error) {
	h := &Hasher{slots: make(chan struct{}, maxConcurrent)}
	dummy, err := h.hash(context.Background(), "kolikaja-dummy-password-for-timing")
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	return h, nil
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

// Hash returns the PHC string of password.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	return h.hash(ctx, password)
}

func (h *Hasher) hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches the PHC string, in constant time.
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	p, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()
	key := argon2.IDKey([]byte(password), p.salt, p.time, p.memory, p.threads, uint32(len(p.key))) // #nosec G115 -- key length is bounded by parsePHC.
	return subtle.ConstantTimeCompare(key, p.key) == 1, nil
}

// VerifyDummy spends the same time as a real verification; used when the account does
// not exist so that timing does not reveal registered e-mail addresses.
func (h *Hasher) VerifyDummy(ctx context.Context, password string) {
	_, _ = h.Verify(ctx, password, h.dummy)
}

type phc struct {
	memory, time uint32
	threads      uint8
	salt, key    []byte
}

var errBadHash = errors.New("argon2: malformed hash")

func parsePHC(s string) (phc, error) {
	parts := strings.Split(s, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return phc{}, errBadHash
	}
	var p phc
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return phc{}, errBadHash
	}
	if p.memory < 8*1024 || p.memory > 1<<20 || p.time < 1 || p.time > 10 || p.threads < 1 || p.threads > 16 {
		return phc{}, errBadHash
	}
	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(p.salt) < 8 {
		return phc{}, errBadHash
	}
	if p.key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil || len(p.key) < 16 || len(p.key) > 64 {
		return phc{}, errBadHash
	}
	return p, nil
}
