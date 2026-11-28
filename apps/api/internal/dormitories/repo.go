// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package dormitories

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
	errNotFound  = errors.New("dormitories: not found")
	errSlugTaken = errors.New("dormitories: slug taken")
	errMember    = errors.New("dormitories: already a member")
)

type searchCursor struct {
	Name string    `json:"n"`
	ID   uuid.UUID `json:"i"`
}

func searchPublic(ctx context.Context, q db.Querier, userID uuid.UUID, pattern string, after *searchCursor, limit int) ([]Summary, []searchCursor, error) {
	var afterName *string
	var afterID *uuid.UUID
	if after != nil {
		afterName, afterID = &after.Name, &after.ID
	}
	rows, err := q.Query(ctx, `
		SELECT d.id, d.slug, d.name, d.city, d.description, d.visibility, d.join_policy,
		       (SELECT count(*) FROM dormitory_members m WHERE m.dormitory_id = d.id AND m.left_at IS NULL)::int,
		       EXISTS (SELECT 1 FROM dormitory_members m WHERE m.dormitory_id = d.id AND m.user_id = $1 AND m.left_at IS NULL),
		       lower(d.name)
		FROM dormitories d
		WHERE d.visibility = 'public' AND d.deleted_at IS NULL
		  AND ($2::text IS NULL OR d.name ILIKE $2 ESCAPE '\' OR d.city ILIKE $2 ESCAPE '\')
		  AND ($3::text IS NULL OR (lower(d.name), d.id) > ($3::text, $4::uuid))
		ORDER BY lower(d.name), d.id
		LIMIT $5`, userID, nilIfEmpty(pattern), afterName, afterID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("dormitories: search: %w", err)
	}
	defer rows.Close()
	var out []Summary
	var keys []searchCursor
	for rows.Next() {
		var s Summary
		var key searchCursor
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.City, &s.Description, &s.Visibility, &s.JoinPolicy,
			&s.MemberCount, &s.IsMember, &key.Name); err != nil {
			return nil, nil, fmt.Errorf("dormitories: search scan: %w", err)
		}
		key.ID = s.ID
		out = append(out, s)
		keys = append(keys, key)
	}
	return out, keys, rows.Err()
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type newDormitory struct {
	ID               uuid.UUID
	Slug, Name       string
	Description      *string
	City, Address    *string
	Visibility       string
	JoinPolicy       string
	Timezone         string
	CommentsEnabled  bool
	PhotosEnabled    bool
	RatingWindowDays *int
	CreatedBy        uuid.UUID
	Now              time.Time
}

func insertDormitory(ctx context.Context, q db.Querier, d newDormitory) error {
	_, err := q.Exec(ctx, `
		INSERT INTO dormitories (id, slug, name, description, city, address, visibility, join_policy, timezone,
		                         comments_enabled, photos_enabled, rating_window_days, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)`,
		d.ID, d.Slug, d.Name, d.Description, d.City, d.Address, d.Visibility, d.JoinPolicy, d.Timezone,
		d.CommentsEnabled, d.PhotosEnabled, d.RatingWindowDays, d.CreatedBy, d.Now)
	if db.Is(err, db.UniqueViolation, "dormitories_slug_key") {
		return errSlugTaken
	}
	if err != nil {
		return fmt.Errorf("dormitories: insert: %w", err)
	}
	return nil
}

func insertMembership(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID, role, joinedVia string, now time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO dormitory_members (id, dormitory_id, user_id, role, joined_via, joined_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)`, id, dormitoryID, userID, role, joinedVia, now)
	if db.Is(err, db.UniqueViolation, "dormitory_members_active_key") {
		return errMember
	}
	if err != nil {
		return fmt.Errorf("dormitories: insert membership: %w", err)
	}
	return nil
}

func getDormitory(ctx context.Context, q db.Querier, id uuid.UUID) (Dormitory, error) {
	var d Dormitory
	err := q.QueryRow(ctx, `
		SELECT id, slug, name, description, city, address, visibility, join_policy, timezone,
		       comments_enabled, photos_enabled, rating_window_days, created_at, updated_at,
		       (SELECT count(*) FROM dormitory_members m WHERE m.dormitory_id = dormitories.id AND m.left_at IS NULL)::int
		FROM dormitories WHERE id = $1 AND deleted_at IS NULL`, id).Scan(
		&d.ID, &d.Slug, &d.Name, &d.Description, &d.City, &d.Address, &d.Visibility, &d.JoinPolicy, &d.Timezone,
		&d.Settings.CommentsEnabled, &d.Settings.PhotosEnabled, &d.Settings.RatingWindowDays, &d.CreatedAt, &d.UpdatedAt,
		&d.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, errNotFound
	}
	if err != nil {
		return d, fmt.Errorf("dormitories: get: %w", err)
	}
	d.CreatedAt, d.UpdatedAt = d.CreatedAt.UTC(), d.UpdatedAt.UTC()
	return d, nil
}

func dormitorySuspension(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID) (*SanctionSummary, error) {
	var s SanctionSummary
	err := q.QueryRow(ctx, `
		SELECT id, kind, dormitory_id, starts_at, ends_at FROM user_sanctions
		WHERE user_id = $1 AND dormitory_id = $2 AND kind = 'suspension' AND revoked_at IS NULL
		  AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
		ORDER BY starts_at DESC LIMIT 1`, userID, dormitoryID).Scan(&s.ID, &s.Kind, &s.DormitoryID, &s.StartsAt, &s.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dormitories: suspension: %w", err)
	}
	s.StartsAt = s.StartsAt.UTC()
	if s.EndsAt != nil {
		t := s.EndsAt.UTC()
		s.EndsAt = &t
	}
	return &s, nil
}

func myMemberships(ctx context.Context, q db.Querier, userID uuid.UUID) ([]Membership, error) {
	rows, err := q.Query(ctx, `
		SELECT d.id, d.slug, d.name, d.city, d.description, d.visibility, d.join_policy,
		       (SELECT count(*) FROM dormitory_members x WHERE x.dormitory_id = d.id AND x.left_at IS NULL)::int,
		       m.role, m.is_trusted_reviewer, m.joined_at
		FROM dormitory_members m JOIN dormitories d ON d.id = m.dormitory_id
		WHERE m.user_id = $1 AND m.left_at IS NULL AND d.deleted_at IS NULL
		ORDER BY m.joined_at, d.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("dormitories: mine: %w", err)
	}
	defer rows.Close()
	out := []Membership{}
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.Dormitory.ID, &m.Dormitory.Slug, &m.Dormitory.Name, &m.Dormitory.City, &m.Dormitory.Description,
			&m.Dormitory.Visibility, &m.Dormitory.JoinPolicy, &m.Dormitory.MemberCount, &m.Role, &m.Trusted, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("dormitories: mine scan: %w", err)
		}
		m.Dormitory.IsMember = true
		m.JoinedAt = m.JoinedAt.UTC()
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		s, err := dormitorySuspension(ctx, q, out[i].Dormitory.ID, userID)
		if err != nil {
			return nil, err
		}
		out[i].Restriction = s
	}
	return out, nil
}
