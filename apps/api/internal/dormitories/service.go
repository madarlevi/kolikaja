// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package dormitories

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madarlevi/kolikaja/apps/api/internal/audit"
	"github.com/madarlevi/kolikaja/apps/api/internal/authz"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/validate"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// Service implements the dormitory use cases.
type Service struct {
	pool              *pgxpool.Pool
	dormitoryCreation string
	now               func() time.Time
}

// NewService returns the service. creation is "global_admins" or "all_users".
func NewService(pool *pgxpool.Pool, creation string, now func() time.Time) *Service {
	return &Service{pool: pool, dormitoryCreation: creation, now: now}
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Search lists public dormitories by name (cursor pagination).
func (s *Service) Search(ctx context.Context, p principal.Principal, query, cursor string, limit int) (SummaryPage, error) {
	query = validate.Text(query)
	var v validate.Collector
	if query != "" {
		v.Length("q", query, 1, 100, false)
	}
	if err := v.Err(); err != nil {
		return SummaryPage{}, err
	}
	var after *searchCursor
	if cursor != "" {
		c, err := decodeCursor(cursor)
		if err != nil {
			return SummaryPage{}, err
		}
		after = &c
	}
	pattern := ""
	if query != "" {
		pattern = "%" + escapeLike(query) + "%"
	}
	items, keys, err := searchPublic(ctx, s.pool, p.UserID, pattern, after, limit+1)
	if err != nil {
		return SummaryPage{}, err
	}
	page := SummaryPage{Data: items}
	if page.Data == nil {
		page.Data = []Summary{}
	}
	if len(items) > limit {
		page.Data = items[:limit]
		next := encodeCursor(keys[limit-1])
		page.NextCursor = &next
	}
	return page, nil
}

// Create creates a dormitory; the caller becomes its administrator.
func (s *Service) Create(ctx context.Context, p principal.Principal, req CreateRequest) (Dormitory, error) {
	if s.dormitoryCreation != "all_users" && !p.IsGlobalAdmin {
		return Dormitory{}, apierror.Forbiddenf("Only global administrators can create dormitories on this server.")
	}
	// An instance-wide suspension makes the account read-only everywhere (uuid.Nil matches
	// no dormitory, so only instance-wide sanctions apply).
	if err := authz.RequireWritable(ctx, s.pool, uuid.Nil, p.UserID); err != nil {
		return Dormitory{}, err
	}
	d, err := validateCreate(req)
	if err != nil {
		return Dormitory{}, err
	}
	if d.ID, err = uuid.NewV7(); err != nil {
		return Dormitory{}, err
	}
	d.CreatedBy, d.Now = p.UserID, s.now().UTC()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertDormitory(ctx, tx, d); err != nil {
			return err
		}
		if err := insertMembership(ctx, tx, d.ID, p.UserID, authz.RoleAdmin, "creator", d.Now); err != nil {
			return err
		}
		uid, did := p.UserID, d.ID
		return audit.Write(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, ActorUserID: &uid, DormitoryID: &did,
			Action: "dormitory.created", TargetType: "dormitory", TargetID: &did,
		})
	})
	if errors.Is(err, errSlugTaken) {
		return Dormitory{}, apierror.ConflictCode(apierror.AlreadyExists, "This slug is already used.")
	}
	if err != nil {
		return Dormitory{}, err
	}
	return s.Get(ctx, p, d.ID)
}

func validateCreate(req CreateRequest) (newDormitory, error) {
	var v validate.Collector
	d := newDormitory{
		Slug:            req.Slug,
		Name:            validate.Text(req.Name),
		Visibility:      req.Visibility,
		JoinPolicy:      req.JoinPolicy,
		Timezone:        "Europe/Budapest",
		CommentsEnabled: true,
		PhotosEnabled:   true,
	}
	if len(d.Slug) < 3 || len(d.Slug) > 64 || !slugPattern.MatchString(d.Slug) {
		v.Add("slug", "invalid_format")
	}
	v.Length("name", d.Name, 2, 100, false)
	d.Description = optionalText(&v, "description", req.Description, 0, 2000, true)
	d.City = optionalText(&v, "city", req.City, 1, 100, false)
	d.Address = optionalText(&v, "address", req.Address, 1, 200, false)
	v.OneOf("visibility", d.Visibility, "public", "private")
	v.OneOf("join_policy", d.JoinPolicy, "open", "invite_only")
	if d.Visibility == "private" && d.JoinPolicy == "open" {
		v.Add("join_policy", "not_allowed")
	}
	if req.Timezone != nil {
		d.Timezone = *req.Timezone
		if _, err := time.LoadLocation(d.Timezone); err != nil || d.Timezone == "" || d.Timezone == "Local" || len(d.Timezone) > 64 {
			v.Add("timezone", "invalid_value")
		}
	}
	if st := req.Settings; st != nil {
		if st.CommentsEnabled != nil {
			d.CommentsEnabled = *st.CommentsEnabled
		}
		if st.PhotosEnabled != nil {
			d.PhotosEnabled = *st.PhotosEnabled
		}
		if st.RatingWindowDays != nil {
			if *st.RatingWindowDays < 1 || *st.RatingWindowDays > 365 {
				v.Add("settings.rating_window_days", "out_of_range")
			}
			d.RatingWindowDays = st.RatingWindowDays
		}
	}
	return d, v.Err()
}

func optionalText(v *validate.Collector, field string, raw *string, minLen, maxLen int, multiline bool) *string {
	if raw == nil {
		return nil
	}
	s := validate.Text(*raw)
	v.Length(field, s, minLen, maxLen, multiline)
	if s == "" {
		return nil
	}
	return &s
}

// Get returns a dormitory. Private dormitories are not found for non-members.
func (s *Service) Get(ctx context.Context, p principal.Principal, id uuid.UUID) (Dormitory, error) {
	d, err := getDormitory(ctx, s.pool, id)
	if errors.Is(err, errNotFound) {
		return d, apierror.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	m, err := authz.Lookup(ctx, s.pool, id, p.UserID)
	if err != nil {
		return d, err
	}
	if m == nil {
		if d.Visibility != "public" {
			return Dormitory{}, apierror.ErrNotFound
		}
		return d, nil
	}
	restriction, err := dormitorySuspension(ctx, s.pool, id, p.UserID)
	if err != nil {
		return d, err
	}
	d.MyMembership = &MembershipInfo{Role: m.Role, Trusted: m.Trusted, JoinedAt: m.JoinedAt.UTC(), Restriction: restriction}
	return d, nil
}

// Join joins a public dormitory with open join policy.
func (s *Service) Join(ctx context.Context, p principal.Principal, id uuid.UUID) (Membership, error) {
	d, err := getDormitory(ctx, s.pool, id)
	if errors.Is(err, errNotFound) || (err == nil && d.Visibility != "public") {
		return Membership{}, apierror.ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	if d.JoinPolicy != "open" {
		return Membership{}, apierror.ForbiddenCode(apierror.InvitationRequired, "This dormitory can only be joined with an invitation code.")
	}
	banned, err := authz.ActiveBan(ctx, s.pool, id, p.UserID)
	if err != nil {
		return Membership{}, err
	}
	if banned {
		return Membership{}, apierror.ForbiddenCode(apierror.Banned, "You are banned from this dormitory.")
	}
	// Suspended users (in this dormitory or instance-wide) cannot leave and rejoin either.
	if err := authz.RequireWritable(ctx, s.pool, id, p.UserID); err != nil {
		return Membership{}, err
	}
	now := s.now().UTC()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertMembership(ctx, tx, id, p.UserID, authz.RoleMember, "open_join", now); err != nil {
			return err
		}
		uid, did := p.UserID, id
		return audit.Write(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, ActorUserID: &uid, DormitoryID: &did,
			Action: "member.joined", TargetType: "dormitory", TargetID: &did,
		})
	})
	if errors.Is(err, errMember) {
		return Membership{}, apierror.ConflictCode(apierror.AlreadyMember, "You are already a member of this dormitory.")
	}
	if err != nil {
		return Membership{}, err
	}
	restriction, err := dormitorySuspension(ctx, s.pool, id, p.UserID)
	if err != nil {
		return Membership{}, err
	}
	return Membership{
		Dormitory: Summary{ID: d.ID, Slug: d.Slug, Name: d.Name, City: d.City, Description: d.Description,
			Visibility: d.Visibility, JoinPolicy: d.JoinPolicy, MemberCount: d.MemberCount + 1, IsMember: true},
		Role: authz.RoleMember, JoinedAt: now, Restriction: restriction,
	}, nil
}

// Mine lists the caller's active memberships.
func (s *Service) Mine(ctx context.Context, p principal.Principal) (MembershipList, error) {
	list, err := myMemberships(ctx, s.pool, p.UserID)
	if err != nil {
		return MembershipList{}, err
	}
	return MembershipList{Data: list}, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func encodeCursor(c searchCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (searchCursor, error) {
	var c searchCursor
	if len(s) > 200 {
		return c, apierror.BadRequestf("Invalid cursor.")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || json.Unmarshal(b, &c) != nil || c.ID == uuid.Nil {
		return c, apierror.BadRequestf("Invalid cursor.")
	}
	return c, nil
}
