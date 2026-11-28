// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package menus implements daily and weekly menus, meals, meal statistics, ratings and
// recommendations (docs/ARCHITECTURE.md §7). Every query is scoped by dormitory_id after
// the membership check.
package menus

import (
	"time"

	"github.com/google/uuid"
)

// Category is the contract schema MealCategory.
type Category struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Position int       `json:"position"`
}

// Stats is the contract schema MealStats.
type Stats struct {
	RatingCount          int      `json:"rating_count"`
	RatingAverage        *float64 `json:"rating_average"`
	RatingDistribution   [5]int   `json:"rating_distribution"`
	TrustedRatingCount   int      `json:"trusted_rating_count"`
	TrustedRatingAverage *float64 `json:"trusted_rating_average"`
	RecommendedCount     int      `json:"recommended_count"`
	NotRecommendedCount  int      `json:"not_recommended_count"`
}

// MealSummary is the contract schema MealSummary.
type MealSummary struct {
	ID               uuid.UUID `json:"id"`
	MenuID           uuid.UUID `json:"menu_id"`
	Category         *Category `json:"category"`
	Name             string    `json:"name"`
	Description      *string   `json:"description"`
	Allergens        *[]string `json:"allergens"`
	AllergenNote     *string   `json:"allergen_note"`
	Position         int       `json:"position"`
	Stats            Stats     `json:"stats"`
	MyRating         *int      `json:"my_rating"`
	MyRecommendation *string   `json:"my_recommendation"`
	CommentCount     int       `json:"comment_count"`
	PhotoCount       int       `json:"photo_count"`
	CoverPhoto       *struct{} `json:"cover_photo"`
}

// MenuRef is the menu part of MealDetail.
type MenuRef struct {
	ID         uuid.UUID `json:"id"`
	MenuDate   string    `json:"menu_date"`
	MealPeriod string    `json:"meal_period"`
	Status     string    `json:"status"`
}

// MealDetail is the contract schema MealDetail.
type MealDetail struct {
	ID               uuid.UUID  `json:"id"`
	Menu             MenuRef    `json:"menu"`
	Category         *Category  `json:"category"`
	Name             string     `json:"name"`
	Description      *string    `json:"description"`
	Allergens        *[]string  `json:"allergens"`
	AllergenNote     *string    `json:"allergen_note"`
	Position         int        `json:"position"`
	Stats            Stats      `json:"stats"`
	MyRating         *int       `json:"my_rating"`
	MyRecommendation *string    `json:"my_recommendation"`
	CommentCount     int        `json:"comment_count"`
	PhotoCount       int        `json:"photo_count"`
	CoverPhoto       *struct{}  `json:"cover_photo"`
	RatingOpen       bool       `json:"rating_open"`
	RatingClosesAt   *time.Time `json:"rating_closes_at"`
}

// Menu is the contract schema Menu.
type Menu struct {
	ID          uuid.UUID     `json:"id"`
	DormitoryID uuid.UUID     `json:"dormitory_id"`
	MenuDate    string        `json:"menu_date"`
	MealPeriod  string        `json:"meal_period"`
	Title       *string       `json:"title"`
	Note        *string       `json:"note"`
	Status      string        `json:"status"`
	PublishedAt *time.Time    `json:"published_at"`
	Meals       []MealSummary `json:"meals"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// MenuList is the contract schema MenuList.
type MenuList struct {
	Data []Menu `json:"data"`
}

// DayMenus is the contract schema DayMenus.
type DayMenus struct {
	Date     string `json:"date"`
	Timezone string `json:"timezone"`
	Menus    []Menu `json:"menus"`
}

// Day is one entry of WeekMenus.days.
type Day struct {
	Date  string `json:"date"`
	Menus []Menu `json:"menus"`
}

// WeekMenus is the contract schema WeekMenus.
type WeekMenus struct {
	WeekStart string `json:"week_start"`
	WeekEnd   string `json:"week_end"`
	Timezone  string `json:"timezone"`
	Days      []Day  `json:"days"`
}

// CreateMealRequest is the contract schema CreateMealRequest.
type CreateMealRequest struct {
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	CategoryID   *string   `json:"category_id"`
	Allergens    *[]string `json:"allergens"`
	AllergenNote *string   `json:"allergen_note"`
	Position     *int      `json:"position"`
}

// CreateMenuRequest is the contract schema CreateMenuRequest.
type CreateMenuRequest struct {
	MenuDate   string              `json:"menu_date"`
	MealPeriod *string             `json:"meal_period"`
	Title      *string             `json:"title"`
	Note       *string             `json:"note"`
	Status     *string             `json:"status"`
	Meals      []CreateMealRequest `json:"meals"`
}

// RatingRequest is the contract schema RatingRequest.
type RatingRequest struct {
	Value int `json:"value"`
}

// RatingValue is the rating part of RatingResult.
type RatingValue struct {
	Value     int       `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RatingResult is the contract schema RatingResult.
type RatingResult struct {
	Rating RatingValue `json:"rating"`
	Stats  Stats       `json:"stats"`
}

// RecommendationRequest is the contract schema RecommendationRequest.
type RecommendationRequest struct {
	Value string `json:"value"`
}

// RecommendationValue is the recommendation part of RecommendationResult.
type RecommendationValue struct {
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RecommendationResult is the contract schema RecommendationResult.
type RecommendationResult struct {
	Recommendation RecommendationValue `json:"recommendation"`
	Stats          Stats               `json:"stats"`
}

// Allergens are the 14 groups of Annex II of Regulation (EU) No 1169/2011 (migration 0007).
var Allergens = map[string]bool{
	"gluten": true, "crustaceans": true, "eggs": true, "fish": true, "peanuts": true, "soybeans": true, "milk": true,
	"tree_nuts": true, "celery": true, "mustard": true, "sesame": true, "sulphites": true, "lupin": true, "molluscs": true,
}

// MealPeriods in display order.
var MealPeriods = []string{"breakfast", "lunch", "dinner", "all_day"}
