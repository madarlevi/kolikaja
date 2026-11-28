// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package menus

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/db"
)

const dateLayout = "2006-01-02"

var (
	errNotFound  = errors.New("menus: not found")
	errDuplicate = errors.New("menus: duplicate menu")
)

// minAggregate is the smallest group whose aggregates are shown (ratings, trusted ratings,
// recommendations). Smaller groups would reveal individual values (docs/DECISIONS.md T-52).
const minAggregate = 3

type dormitoryContext struct {
	Location         *time.Location
	Timezone         string
	RatingWindowDays *int
}

func loadDormitoryContext(ctx context.Context, q db.Querier, dormitoryID uuid.UUID) (dormitoryContext, error) {
	var c dormitoryContext
	err := q.QueryRow(ctx, `SELECT timezone, rating_window_days FROM dormitories WHERE id = $1 AND deleted_at IS NULL`,
		dormitoryID).Scan(&c.Timezone, &c.RatingWindowDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, errNotFound
	}
	if err != nil {
		return c, fmt.Errorf("menus: dormitory: %w", err)
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		loc = time.UTC
	}
	c.Location = loc
	return c, nil
}

func listMenus(ctx context.Context, q db.Querier, dormitoryID uuid.UUID, from, to time.Time, includeDrafts bool) ([]Menu, error) {
	rows, err := q.Query(ctx, `
		SELECT id, menu_date, meal_period, title, note, status, published_at, created_at, updated_at
		FROM menus
		WHERE dormitory_id = $1 AND deleted_at IS NULL AND menu_date BETWEEN $2 AND $3
		  AND (status = 'published' OR $4)
		ORDER BY menu_date,
		         CASE meal_period WHEN 'breakfast' THEN 1 WHEN 'lunch' THEN 2 WHEN 'dinner' THEN 3 ELSE 4 END`,
		dormitoryID, from, to, includeDrafts)
	if err != nil {
		return nil, fmt.Errorf("menus: list: %w", err)
	}
	defer rows.Close()
	out := []Menu{}
	for rows.Next() {
		m := Menu{DormitoryID: dormitoryID, Meals: []MealSummary{}}
		var date time.Time
		if err := rows.Scan(&m.ID, &date, &m.MealPeriod, &m.Title, &m.Note, &m.Status, &m.PublishedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("menus: list scan: %w", err)
		}
		m.MenuDate = date.Format(dateLayout)
		m.CreatedAt, m.UpdatedAt = m.CreatedAt.UTC(), m.UpdatedAt.UTC()
		if m.PublishedAt != nil {
			t := m.PublishedAt.UTC()
			m.PublishedAt = &t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func mealsOfMenus(ctx context.Context, q db.Querier, dormitoryID uuid.UUID, menuIDs []uuid.UUID) ([]MealSummary, error) {
	rows, err := q.Query(ctx, `
		SELECT m.id, m.menu_id, m.name, m.description, m.allergens, m.allergen_note, m.position, c.id, c.name, c.position
		FROM meals m
		LEFT JOIN meal_categories c ON c.id = m.category_id AND c.dormitory_id = m.dormitory_id AND c.deleted_at IS NULL
		WHERE m.dormitory_id = $1 AND m.menu_id = ANY($2) AND m.deleted_at IS NULL
		ORDER BY m.menu_id, m.position, m.name, m.id`, dormitoryID, menuIDs)
	if err != nil {
		return nil, fmt.Errorf("menus: meals: %w", err)
	}
	defer rows.Close()
	var out []MealSummary
	for rows.Next() {
		var s MealSummary
		var catID *uuid.UUID
		var catName *string
		var catPos *int
		if err := rows.Scan(&s.ID, &s.MenuID, &s.Name, &s.Description, &s.Allergens, &s.AllergenNote, &s.Position,
			&catID, &catName, &catPos); err != nil {
			return nil, fmt.Errorf("menus: meals scan: %w", err)
		}
		if catID != nil {
			s.Category = &Category{ID: *catID, Name: deref(catName), Position: derefInt(catPos)}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type mealRow struct {
	Detail   MealDetail
	MenuDate time.Time
}

func loadMeal(ctx context.Context, q db.Querier, dormitoryID, mealID uuid.UUID) (mealRow, error) {
	var r mealRow
	d := &r.Detail
	var catID *uuid.UUID
	var catName *string
	var catPos *int
	err := q.QueryRow(ctx, `
		SELECT m.id, m.name, m.description, m.allergens, m.allergen_note, m.position,
		       n.id, n.menu_date, n.meal_period, n.status, c.id, c.name, c.position
		FROM meals m
		JOIN menus n ON n.id = m.menu_id AND n.dormitory_id = m.dormitory_id AND n.deleted_at IS NULL
		LEFT JOIN meal_categories c ON c.id = m.category_id AND c.dormitory_id = m.dormitory_id AND c.deleted_at IS NULL
		WHERE m.id = $1 AND m.dormitory_id = $2 AND m.deleted_at IS NULL`, mealID, dormitoryID).Scan(
		&d.ID, &d.Name, &d.Description, &d.Allergens, &d.AllergenNote, &d.Position,
		&d.Menu.ID, &r.MenuDate, &d.Menu.MealPeriod, &d.Menu.Status, &catID, &catName, &catPos)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, errNotFound
	}
	if err != nil {
		return r, fmt.Errorf("menus: meal: %w", err)
	}
	d.Menu.MenuDate = r.MenuDate.Format(dateLayout)
	if catID != nil {
		d.Category = &Category{ID: *catID, Name: deref(catName), Position: derefInt(catPos)}
	}
	return r, nil
}

type mealExtras struct {
	Stats            map[uuid.UUID]*Stats
	MyRating         map[uuid.UUID]int
	MyRecommendation map[uuid.UUID]string
	Comments         map[uuid.UUID]int
	Photos           map[uuid.UUID]int
}

// loadExtras loads statistics and the caller's own rating and recommendation for meals of
// one dormitory. Individual ratings of other users never leave this function.
func loadExtras(ctx context.Context, q db.Querier, dormitoryID, userID uuid.UUID, mealIDs []uuid.UUID) (mealExtras, error) {
	x := mealExtras{
		Stats: map[uuid.UUID]*Stats{}, MyRating: map[uuid.UUID]int{}, MyRecommendation: map[uuid.UUID]string{},
		Comments: map[uuid.UUID]int{}, Photos: map[uuid.UUID]int{},
	}
	if len(mealIDs) == 0 {
		return x, nil
	}
	sums := map[uuid.UUID]*[2]float64{} // all, trusted rating sums
	stat := func(id uuid.UUID) *Stats {
		if x.Stats[id] == nil {
			x.Stats[id] = &Stats{}
			sums[id] = &[2]float64{}
		}
		return x.Stats[id]
	}
	rows, err := q.Query(ctx, `
		SELECT r.meal_id, r.value, count(*)::int, (count(*) FILTER (WHERE dm.is_trusted_reviewer))::int
		FROM ratings r
		LEFT JOIN dormitory_members dm ON dm.dormitory_id = r.dormitory_id AND dm.user_id = r.user_id AND dm.left_at IS NULL
		WHERE r.dormitory_id = $1 AND r.meal_id = ANY($2) AND r.deleted_at IS NULL
		GROUP BY r.meal_id, r.value`, dormitoryID, mealIDs)
	if err != nil {
		return x, fmt.Errorf("menus: rating stats: %w", err)
	}
	for rows.Next() {
		var id uuid.UUID
		var value, n, trusted int
		if err := rows.Scan(&id, &value, &n, &trusted); err != nil {
			rows.Close()
			return x, fmt.Errorf("menus: rating stats scan: %w", err)
		}
		if value < 1 || value > 5 {
			continue
		}
		s := stat(id)
		s.RatingDistribution[value-1] += n
		s.RatingCount += n
		s.TrustedRatingCount += trusted
		sums[id][0] += float64(value * n)
		sums[id][1] += float64(value * trusted)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return x, err
	}
	for id, s := range x.Stats {
		if s.RatingCount >= minAggregate {
			avg := round2(sums[id][0] / float64(s.RatingCount))
			s.RatingAverage = &avg
		} else {
			s.RatingDistribution = [5]int{}
		}
		if s.TrustedRatingCount >= minAggregate {
			avg := round2(sums[id][1] / float64(s.TrustedRatingCount))
			s.TrustedRatingAverage = &avg
		} else {
			s.TrustedRatingCount = 0
		}
	}

	if err := scanCounts(ctx, q, `
		SELECT meal_id, value, count(*)::int FROM recommendations
		WHERE dormitory_id = $1 AND meal_id = ANY($2) GROUP BY meal_id, value`, dormitoryID, mealIDs,
		func(id uuid.UUID, value string, n int) {
			if value == "recommended" {
				stat(id).RecommendedCount = n
			} else {
				stat(id).NotRecommendedCount = n
			}
		}); err != nil {
		return x, err
	}
	if err := scanCounts(ctx, q, `
		SELECT meal_id, '', count(*)::int FROM comments
		WHERE dormitory_id = $1 AND meal_id = ANY($2) AND deleted_at IS NULL GROUP BY meal_id`, dormitoryID, mealIDs,
		func(id uuid.UUID, _ string, n int) { x.Comments[id] = n }); err != nil {
		return x, err
	}
	if err := scanCounts(ctx, q, `
		SELECT meal_id, '', count(*)::int FROM meal_photos
		WHERE dormitory_id = $1 AND meal_id = ANY($2) AND status = 'ready' GROUP BY meal_id`, dormitoryID, mealIDs,
		func(id uuid.UUID, _ string, n int) { x.Photos[id] = n }); err != nil {
		return x, err
	}

	for _, s := range x.Stats {
		if s.RecommendedCount+s.NotRecommendedCount < minAggregate {
			s.RecommendedCount, s.NotRecommendedCount = 0, 0
		}
	}

	own, err := q.Query(ctx, `
		SELECT meal_id, value FROM ratings
		WHERE dormitory_id = $1 AND user_id = $2 AND meal_id = ANY($3) AND deleted_at IS NULL`, dormitoryID, userID, mealIDs)
	if err != nil {
		return x, fmt.Errorf("menus: own ratings: %w", err)
	}
	for own.Next() {
		var id uuid.UUID
		var v int
		if err := own.Scan(&id, &v); err != nil {
			own.Close()
			return x, fmt.Errorf("menus: own ratings scan: %w", err)
		}
		x.MyRating[id] = v
	}
	own.Close()
	if err := own.Err(); err != nil {
		return x, err
	}
	ownRec, err := q.Query(ctx, `
		SELECT meal_id, value FROM recommendations
		WHERE dormitory_id = $1 AND user_id = $2 AND meal_id = ANY($3)`, dormitoryID, userID, mealIDs)
	if err != nil {
		return x, fmt.Errorf("menus: own recommendations: %w", err)
	}
	defer ownRec.Close()
	for ownRec.Next() {
		var id uuid.UUID
		var v string
		if err := ownRec.Scan(&id, &v); err != nil {
			return x, fmt.Errorf("menus: own recommendations scan: %w", err)
		}
		x.MyRecommendation[id] = v
	}
	return x, ownRec.Err()
}

func scanCounts(ctx context.Context, q db.Querier, sql string, dormitoryID uuid.UUID, mealIDs []uuid.UUID, fn func(uuid.UUID, string, int)) error {
	rows, err := q.Query(ctx, sql, dormitoryID, mealIDs)
	if err != nil {
		return fmt.Errorf("menus: counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var key string
		var n int
		if err := rows.Scan(&id, &key, &n); err != nil {
			return fmt.Errorf("menus: counts scan: %w", err)
		}
		fn(id, key, n)
	}
	return rows.Err()
}

type newMeal struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	CategoryID   *uuid.UUID
	Allergens    *[]string
	AllergenNote *string
	Position     int
}

func insertMenu(ctx context.Context, q db.Querier, dormitoryID, menuID, userID uuid.UUID, date time.Time, period string,
	title, note *string, status string, now time.Time, meals []newMeal) error {
	var publishedAt *time.Time
	if status == "published" {
		publishedAt = &now
	}
	_, err := q.Exec(ctx, `
		INSERT INTO menus (id, dormitory_id, menu_date, meal_period, title, note, status, published_at, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $10)`,
		menuID, dormitoryID, date, period, title, note, status, publishedAt, userID, now)
	if db.Is(err, db.UniqueViolation, "menus_dormitory_date_period_key") {
		return errDuplicate
	}
	if err != nil {
		return fmt.Errorf("menus: insert menu: %w", err)
	}
	for _, m := range meals {
		_, err := q.Exec(ctx, `
			INSERT INTO meals (id, dormitory_id, menu_id, category_id, name, description, allergens, allergen_note, position, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			m.ID, dormitoryID, menuID, m.CategoryID, m.Name, m.Description, m.Allergens, m.AllergenNote, m.Position, now)
		if err != nil {
			return fmt.Errorf("menus: insert meal: %w", err)
		}
	}
	return nil
}

func categoryExists(ctx context.Context, q db.Querier, dormitoryID, categoryID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM meal_categories WHERE id = $1 AND dormitory_id = $2 AND deleted_at IS NULL)`,
		categoryID, dormitoryID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("menus: category: %w", err)
	}
	return ok, nil
}

func upsertRating(ctx context.Context, q db.Querier, dormitoryID, mealID, userID uuid.UUID, value int, now time.Time) (RatingValue, bool, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return RatingValue{}, false, err
	}
	var r RatingValue
	var inserted bool
	err = q.QueryRow(ctx, `
		INSERT INTO ratings (id, dormitory_id, meal_id, user_id, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (meal_id, user_id) WHERE deleted_at IS NULL
		DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
		RETURNING value, created_at, updated_at, (xmax = 0)`,
		id, dormitoryID, mealID, userID, value, now).Scan(&r.Value, &r.CreatedAt, &r.UpdatedAt, &inserted)
	if err != nil {
		return r, false, fmt.Errorf("menus: upsert rating: %w", err)
	}
	r.CreatedAt, r.UpdatedAt = r.CreatedAt.UTC(), r.UpdatedAt.UTC()
	return r, inserted, nil
}

func deleteRating(ctx context.Context, q db.Querier, dormitoryID, mealID, userID uuid.UUID, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE ratings SET deleted_at = $4, updated_at = $4
		WHERE dormitory_id = $1 AND meal_id = $2 AND user_id = $3 AND deleted_at IS NULL`, dormitoryID, mealID, userID, now)
	if err != nil {
		return false, fmt.Errorf("menus: delete rating: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func upsertRecommendation(ctx context.Context, q db.Querier, dormitoryID, mealID, userID uuid.UUID, value string, now time.Time) (RecommendationValue, bool, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return RecommendationValue{}, false, err
	}
	var r RecommendationValue
	var inserted bool
	err = q.QueryRow(ctx, `
		INSERT INTO recommendations (id, dormitory_id, meal_id, user_id, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT ON CONSTRAINT recommendations_meal_user_key
		DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
		RETURNING value, updated_at, (xmax = 0)`,
		id, dormitoryID, mealID, userID, value, now).Scan(&r.Value, &r.UpdatedAt, &inserted)
	if err != nil {
		return r, false, fmt.Errorf("menus: upsert recommendation: %w", err)
	}
	r.UpdatedAt = r.UpdatedAt.UTC()
	return r, inserted, nil
}

func deleteRecommendation(ctx context.Context, q db.Querier, dormitoryID, mealID, userID uuid.UUID) (bool, error) {
	tag, err := q.Exec(ctx, `DELETE FROM recommendations WHERE dormitory_id = $1 AND meal_id = $2 AND user_id = $3`,
		dormitoryID, mealID, userID)
	if err != nil {
		return false, fmt.Errorf("menus: delete recommendation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
