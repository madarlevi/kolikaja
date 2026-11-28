// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package authz implements the server-side membership and role checks that every
// dormitory-scoped operation runs before touching data (docs/ARCHITECTURE.md §5.3,
// .claude/skills/authz-tenant-isolation). Roles come only from dormitory_members.
package authz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/db"
)

// Dormitory roles in ascending order.
const (
	RoleMember    = "member"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)

var rank = map[string]int{RoleMember: 1, RoleModerator: 2, RoleAdmin: 3}

// AtLeast reports whether role is at least minRole.
func AtLeast(role, minRole string) bool { return rank[role] >= rank[minRole] && rank[role] > 0 }

// Membership is an active membership of a non-deleted dormitory.
type Membership struct {
	Role     string
	Trusted  bool
	JoinedAt time.Time
}

// Lookup returns the caller's active membership, or nil when there is none or the
// dormitory is deleted.
func Lookup(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID) (*Membership, error) {
	var m Membership
	err := q.QueryRow(ctx, `
		SELECT m.role, m.is_trusted_reviewer, m.joined_at
		FROM dormitory_members m JOIN dormitories d ON d.id = m.dormitory_id
		WHERE m.dormitory_id = $1 AND m.user_id = $2 AND m.left_at IS NULL AND d.deleted_at IS NULL`,
		dormitoryID, userID).Scan(&m.Role, &m.Trusted, &m.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("authz: membership: %w", err)
	}
	return &m, nil
}

// Require returns the membership when the caller has at least minRole. Non-members get
// 404 (existence is not revealed); members below the role get 403.
func Require(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID, minRole string) (*Membership, error) {
	m, err := Lookup(ctx, q, dormitoryID, userID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apierror.ErrNotFound
	}
	if !AtLeast(m.Role, minRole) {
		return nil, apierror.Forbiddenf("This action requires the " + minRole + " role.")
	}
	return m, nil
}

// RequireWritable returns 403 suspended while the user has an active suspension in the
// dormitory or instance-wide (read-only mode, docs/ARCHITECTURE.md §5.4).
func RequireWritable(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID) error {
	var suspended bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_sanctions
			WHERE user_id = $1 AND kind = 'suspension' AND revoked_at IS NULL
			  AND (dormitory_id = $2 OR dormitory_id IS NULL)
			  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now()))`,
		userID, dormitoryID).Scan(&suspended)
	if err != nil {
		return fmt.Errorf("authz: suspension: %w", err)
	}
	if suspended {
		return apierror.ForbiddenCode(apierror.Suspended, "Your account is read-only in this dormitory for now.")
	}
	return nil
}

// ActiveBan reports whether the user is banned from the dormitory.
func ActiveBan(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID) (bool, error) {
	var banned bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM user_sanctions
			WHERE user_id = $1 AND dormitory_id = $2 AND kind = 'ban' AND revoked_at IS NULL
			  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now()))`,
		userID, dormitoryID).Scan(&banned)
	if err != nil {
		return false, fmt.Errorf("authz: ban: %w", err)
	}
	return banned, nil
}
