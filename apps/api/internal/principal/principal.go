// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package principal carries the authenticated caller in the request context. The
// principal comes only from a verified token and an active session, never from input.
package principal

import (
	"context"

	"github.com/google/uuid"
)

// Principal is an authenticated user.
type Principal struct {
	UserID        uuid.UUID
	SessionID     uuid.UUID
	IsGlobalAdmin bool
}

type ctxKey struct{}

// With returns a context carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal of ctx.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
