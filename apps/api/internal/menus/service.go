// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package menus

import (
	"context"
	"errors"
	"strconv"
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

// Service implements the menu, meal, rating and recommendation use cases.
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewService returns the service.
func NewService(pool *pgxpool.Pool, now func() time.Time) *Service {
	return &Service{pool: pool, now: now}
}

const (
	maxRangeDays    = 31
	maxMealsPerMenu = 30
)

func today(loc *time.Location, now time.Time) time.Time {
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// member checks membership and loads the dormitory context.
func (s *Service) member(ctx context.Context, p principal.Principal, dormitoryID uuid.UUID, minRole string) (*authz.Membership, dormitoryContext, error) {
	m, err := authz.Require(ctx, s.pool, dormitoryID, p.UserID, minRole)
	if err != nil {
		return nil, dormitoryContext{}, err
	}
	dc, err := loadDormitoryContext(ctx, s.pool, dormitoryID)
	if errors.Is(err, errNotFound) {
		return nil, dc, apierror.ErrNotFound
	}
	return m, dc, err
}

// Today returns the published menus of today in the dormitory's time zone.
func (s *Service) Today(ctx context.Context, p principal.Principal, dormitoryID uuid.UUID) (DayMenus, error) {
	_, dc, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return DayMenus{}, err
	}
	day := today(dc.Location, s.now())
	menus, err := s.loadMenus(ctx, dormitoryID, p.UserID, day, day, false)
	if err != nil {
		return DayMenus{}, err
	}
	return DayMenus{Date: day.Format(dateLayout), Timezone: dc.Timezone, Menus: menus}, nil
}

// Week returns the published menus of the ISO week containing date (default: this week).
func (s *Service) Week(ctx context.Context, p principal.Principal, dormitoryID uuid.UUID, date string) (WeekMenus, error) {
	_, dc, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return WeekMenus{}, err
	}
	ref := today(dc.Location, s.now())
	if date != "" {
		if ref, err = time.Parse(dateLayout, date); err != nil {
			return WeekMenus{}, apierror.BadRequestf("date must be YYYY-MM-DD.")
		}
	}
	offset := (int(ref.Weekday()) + 6) % 7 // Monday = 0
	start := ref.AddDate(0, 0, -offset)
	end := start.AddDate(0, 0, 6)
	menus, err := s.loadMenus(ctx, dormitoryID, p.UserID, start, end, false)
	if err != nil {
		return WeekMenus{}, err
	}
	week := WeekMenus{WeekStart: start.Format(dateLayout), WeekEnd: end.Format(dateLayout), Timezone: dc.Timezone}
	for i := 0; i < 7; i++ {
		d := start.AddDate(0, 0, i).Format(dateLayout)
		day := Day{Date: d, Menus: []Menu{}}
		for _, m := range menus {
			if m.MenuDate == d {
				day.Menus = append(day.Menus, m)
			}
		}
		week.Days = append(week.Days, day)
	}
	return week, nil
}

// List returns menus between from and to (inclusive, at most 31 days).
func (s *Service) List(ctx context.Context, p principal.Principal, dormitoryID uuid.UUID, fromRaw, toRaw string, includeDrafts bool) (MenuList, error) {
	minRole := authz.RoleMember
	if includeDrafts {
		minRole = authz.RoleModerator
	}
	if _, _, err := s.member(ctx, p, dormitoryID, minRole); err != nil {
		return MenuList{}, err
	}
	from, errFrom := time.Parse(dateLayout, fromRaw)
	to, errTo := time.Parse(dateLayout, toRaw)
	if errFrom != nil || errTo != nil {
		return MenuList{}, apierror.BadRequestf("from and to must be YYYY-MM-DD dates.")
	}
	var v validate.Collector
	if to.Before(from) {
		v.Add("to", "out_of_range")
	} else if to.Sub(from) > (maxRangeDays-1)*24*time.Hour {
		v.Add("to", "range_too_long")
	}
	if err := v.Err(); err != nil {
		return MenuList{}, err
	}
	menus, err := s.loadMenus(ctx, dormitoryID, p.UserID, from, to, includeDrafts)
	return MenuList{Data: menus}, err
}

func (s *Service) loadMenus(ctx context.Context, dormitoryID, userID uuid.UUID, from, to time.Time, includeDrafts bool) ([]Menu, error) {
	menus, err := listMenus(ctx, s.pool, dormitoryID, from, to, includeDrafts)
	if err != nil || len(menus) == 0 {
		return menus, err
	}
	ids := make([]uuid.UUID, len(menus))
	index := map[uuid.UUID]int{}
	for i, m := range menus {
		ids[i] = m.ID
		index[m.ID] = i
	}
	meals, err := mealsOfMenus(ctx, s.pool, dormitoryID, ids)
	if err != nil {
		return nil, err
	}
	mealIDs := make([]uuid.UUID, len(meals))
	for i, m := range meals {
		mealIDs[i] = m.ID
	}
	extras, err := loadExtras(ctx, s.pool, dormitoryID, userID, mealIDs)
	if err != nil {
		return nil, err
	}
	for _, meal := range meals {
		applyExtras(&meal, extras)
		i := index[meal.MenuID]
		menus[i].Meals = append(menus[i].Meals, meal)
	}
	return menus, nil
}

func applyExtras(m *MealSummary, x mealExtras) {
	if st := x.Stats[m.ID]; st != nil {
		m.Stats = *st
	}
	if v, ok := x.MyRating[m.ID]; ok {
		m.MyRating = &v
	}
	if v, ok := x.MyRecommendation[m.ID]; ok {
		m.MyRecommendation = &v
	}
	m.CommentCount, m.PhotoCount = x.Comments[m.ID], x.Photos[m.ID]
}

// Create creates a daily menu with optional meals (moderators and above).
func (s *Service) Create(ctx context.Context, p principal.Principal, dormitoryID uuid.UUID, req CreateMenuRequest) (Menu, error) {
	if _, _, err := s.member(ctx, p, dormitoryID, authz.RoleModerator); err != nil {
		return Menu{}, err
	}
	if err := authz.RequireWritable(ctx, s.pool, dormitoryID, p.UserID); err != nil {
		return Menu{}, err
	}
	var v validate.Collector
	date, err := time.Parse(dateLayout, req.MenuDate)
	if err != nil || date.Year() < 2000 || date.Year() > 2100 {
		v.Add("menu_date", "invalid_format")
	}
	period := "lunch"
	if req.MealPeriod != nil {
		period = *req.MealPeriod
		v.OneOf("meal_period", period, MealPeriods...)
	}
	status := "draft"
	if req.Status != nil {
		status = *req.Status
		v.OneOf("status", status, "draft", "published")
	}
	title := optionalText(&v, "title", req.Title, 1, 100, false)
	note := optionalText(&v, "note", req.Note, 0, 1000, true)
	if len(req.Meals) > maxMealsPerMenu {
		v.Add("meals", "too_long")
		return Menu{}, v.Err()
	}
	meals := make([]newMeal, 0, len(req.Meals))
	for i, m := range req.Meals {
		nm, err := s.validateMeal(ctx, &v, dormitoryID, "meals["+strconv.Itoa(i)+"]", m, i)
		if err != nil {
			return Menu{}, err
		}
		meals = append(meals, nm)
	}
	if err := v.Err(); err != nil {
		return Menu{}, err
	}
	menuID, err := uuid.NewV7()
	if err != nil {
		return Menu{}, err
	}
	now := s.now().UTC()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := insertMenu(ctx, tx, dormitoryID, menuID, p.UserID, date, period, title, note, status, now, meals); err != nil {
			return err
		}
		uid, did, mid := p.UserID, dormitoryID, menuID
		return audit.Write(ctx, tx, audit.Entry{
			ActorType: audit.ActorUser, ActorUserID: &uid, DormitoryID: &did,
			Action: "menu.created", TargetType: "menu", TargetID: &mid,
			Metadata: map[string]any{"menu_date": req.MenuDate, "meal_period": period, "status": status, "meals": len(meals)},
		})
	})
	if errors.Is(err, errDuplicate) {
		return Menu{}, apierror.ConflictCode(apierror.Conflict, "A menu for this date and meal period already exists.")
	}
	if err != nil {
		return Menu{}, err
	}
	menus, err := s.loadMenus(ctx, dormitoryID, p.UserID, date, date, true)
	if err != nil {
		return Menu{}, err
	}
	for _, m := range menus {
		if m.ID == menuID {
			return m, nil
		}
	}
	return Menu{}, errors.New("menus: created menu not found")
}

func (s *Service) validateMeal(ctx context.Context, v *validate.Collector, dormitoryID uuid.UUID, prefix string, m CreateMealRequest, index int) (newMeal, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return newMeal{}, err
	}
	nm := newMeal{ID: id, Name: validate.Text(m.Name), Position: index}
	v.Length(prefix+".name", nm.Name, 1, 150, false)
	nm.Description = optionalText(v, prefix+".description", m.Description, 0, 1000, true)
	nm.AllergenNote = optionalText(v, prefix+".allergen_note", m.AllergenNote, 0, 300, true)
	if m.Position != nil {
		if *m.Position < 0 || *m.Position > 1000 {
			v.Add(prefix+".position", "out_of_range")
		}
		nm.Position = *m.Position
	}
	if m.Allergens != nil {
		seen := map[string]bool{}
		list := []string{}
		for _, a := range *m.Allergens {
			if !Allergens[a] {
				v.Add(prefix+".allergens", "invalid_value")
				break
			}
			if seen[a] {
				v.Add(prefix+".allergens", "duplicate")
				break
			}
			seen[a] = true
			list = append(list, a)
		}
		nm.Allergens = &list
	}
	if m.CategoryID != nil {
		cid, err := uuid.Parse(*m.CategoryID)
		if err != nil {
			v.Add(prefix+".category_id", "invalid_format")
			return nm, nil
		}
		ok, err := categoryExists(ctx, s.pool, dormitoryID, cid)
		if err != nil {
			return nm, err
		}
		if !ok {
			v.Add(prefix+".category_id", "invalid_value")
		}
		nm.CategoryID = &cid
	}
	return nm, nil
}

// visibleMeal loads a meal of the dormitory. Meals of draft menus are not found for
// callers below moderator.
func (s *Service) visibleMeal(ctx context.Context, m *authz.Membership, dormitoryID, mealID uuid.UUID) (mealRow, error) {
	row, err := loadMeal(ctx, s.pool, dormitoryID, mealID)
	if errors.Is(err, errNotFound) {
		return row, apierror.ErrNotFound
	}
	if err != nil {
		return row, err
	}
	if row.Detail.Menu.Status != "published" && !authz.AtLeast(m.Role, authz.RoleModerator) {
		return row, apierror.ErrNotFound
	}
	return row, nil
}

// ratingWindow reports whether rating is open and when it closes.
func ratingWindow(row mealRow, dc dormitoryContext, now time.Time) (bool, *time.Time) {
	if row.Detail.Menu.Status != "published" {
		return false, nil
	}
	day := today(dc.Location, now)
	menuDay := time.Date(row.MenuDate.Year(), row.MenuDate.Month(), row.MenuDate.Day(), 0, 0, 0, 0, time.UTC)
	if day.Before(menuDay) {
		return false, nil
	}
	if dc.RatingWindowDays == nil {
		return true, nil
	}
	last := menuDay.AddDate(0, 0, *dc.RatingWindowDays)
	closes := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, dc.Location).AddDate(0, 0, 1).UTC()
	return !day.After(last), &closes
}

// Meal returns meal details with statistics and the caller's own rating.
func (s *Service) Meal(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID) (MealDetail, error) {
	m, dc, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return MealDetail{}, err
	}
	row, err := s.visibleMeal(ctx, m, dormitoryID, mealID)
	if err != nil {
		return MealDetail{}, err
	}
	x, err := loadExtras(ctx, s.pool, dormitoryID, p.UserID, []uuid.UUID{mealID})
	if err != nil {
		return MealDetail{}, err
	}
	d := row.Detail
	if st := x.Stats[mealID]; st != nil {
		d.Stats = *st
	}
	if v, ok := x.MyRating[mealID]; ok {
		d.MyRating = &v
	}
	if v, ok := x.MyRecommendation[mealID]; ok {
		d.MyRecommendation = &v
	}
	d.CommentCount, d.PhotoCount = x.Comments[mealID], x.Photos[mealID]
	d.RatingOpen, d.RatingClosesAt = ratingWindow(row, dc, s.now())
	return d, nil
}

// MealStats returns the statistics of a meal.
func (s *Service) MealStats(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID) (Stats, error) {
	d, err := s.Meal(ctx, p, dormitoryID, mealID)
	return d.Stats, err
}

// writableMeal runs the checks shared by rating and recommendation writes.
func (s *Service) writableMeal(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID) error {
	m, dc, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return err
	}
	if err := authz.RequireWritable(ctx, s.pool, dormitoryID, p.UserID); err != nil {
		return err
	}
	row, err := s.visibleMeal(ctx, m, dormitoryID, mealID)
	if err != nil {
		return err
	}
	if open, _ := ratingWindow(row, dc, s.now()); !open {
		return apierror.ConflictCode(apierror.RatingClosed, "This meal cannot be rated now.")
	}
	return nil
}

func (s *Service) stats(ctx context.Context, dormitoryID, userID, mealID uuid.UUID) (Stats, error) {
	x, err := loadExtras(ctx, s.pool, dormitoryID, userID, []uuid.UUID{mealID})
	if err != nil || x.Stats[mealID] == nil {
		return Stats{}, err
	}
	return *x.Stats[mealID], nil
}

// PutRating creates or replaces the caller's rating; created reports a new rating.
func (s *Service) PutRating(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID, req RatingRequest) (RatingResult, bool, error) {
	if req.Value < 1 || req.Value > 5 {
		var v validate.Collector
		v.Add("value", "out_of_range")
		return RatingResult{}, false, v.Err()
	}
	if err := s.writableMeal(ctx, p, dormitoryID, mealID); err != nil {
		return RatingResult{}, false, err
	}
	r, created, err := upsertRating(ctx, s.pool, dormitoryID, mealID, p.UserID, req.Value, s.now().UTC())
	if err != nil {
		return RatingResult{}, false, err
	}
	st, err := s.stats(ctx, dormitoryID, p.UserID, mealID)
	return RatingResult{Rating: r, Stats: st}, created, err
}

// DeleteRating deletes the caller's rating.
func (s *Service) DeleteRating(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID) error {
	m, _, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return err
	}
	if err := authz.RequireWritable(ctx, s.pool, dormitoryID, p.UserID); err != nil {
		return err
	}
	if _, err := s.visibleMeal(ctx, m, dormitoryID, mealID); err != nil {
		return err
	}
	ok, err := deleteRating(ctx, s.pool, dormitoryID, mealID, p.UserID, s.now().UTC())
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrNotFound
	}
	return nil
}

// PutRecommendation sets the caller's recommendation.
func (s *Service) PutRecommendation(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID, req RecommendationRequest) (RecommendationResult, bool, error) {
	if req.Value != "recommended" && req.Value != "not_recommended" {
		var v validate.Collector
		v.Add("value", "invalid_value")
		return RecommendationResult{}, false, v.Err()
	}
	if err := s.writableMeal(ctx, p, dormitoryID, mealID); err != nil {
		return RecommendationResult{}, false, err
	}
	r, created, err := upsertRecommendation(ctx, s.pool, dormitoryID, mealID, p.UserID, req.Value, s.now().UTC())
	if err != nil {
		return RecommendationResult{}, false, err
	}
	st, err := s.stats(ctx, dormitoryID, p.UserID, mealID)
	return RecommendationResult{Recommendation: r, Stats: st}, created, err
}

// DeleteRecommendation withdraws the caller's recommendation.
func (s *Service) DeleteRecommendation(ctx context.Context, p principal.Principal, dormitoryID, mealID uuid.UUID) error {
	m, _, err := s.member(ctx, p, dormitoryID, authz.RoleMember)
	if err != nil {
		return err
	}
	if err := authz.RequireWritable(ctx, s.pool, dormitoryID, p.UserID); err != nil {
		return err
	}
	if _, err := s.visibleMeal(ctx, m, dormitoryID, mealID); err != nil {
		return err
	}
	ok, err := deleteRecommendation(ctx, s.pool, dormitoryID, mealID, p.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrNotFound
	}
	return nil
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
