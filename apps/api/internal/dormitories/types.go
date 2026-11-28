// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package dormitories implements the dormitory directory, creation, details, joining
// public dormitories and the caller's memberships.
package dormitories

import (
	"time"

	"github.com/google/uuid"
)

// Settings is the contract schema DormitorySettings.
type Settings struct {
	CommentsEnabled  bool `json:"comments_enabled"`
	PhotosEnabled    bool `json:"photos_enabled"`
	RatingWindowDays *int `json:"rating_window_days"`
}

// SanctionSummary is the contract schema SanctionSummary.
type SanctionSummary struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	DormitoryID *uuid.UUID `json:"dormitory_id"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
}

// MembershipInfo is the contract schema MembershipInfo.
type MembershipInfo struct {
	Role        string           `json:"role"`
	Trusted     bool             `json:"is_trusted_reviewer"`
	JoinedAt    time.Time        `json:"joined_at"`
	Restriction *SanctionSummary `json:"restriction"`
}

// Dormitory is the contract schema Dormitory.
type Dormitory struct {
	ID           uuid.UUID       `json:"id"`
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	City         *string         `json:"city"`
	Address      *string         `json:"address"`
	Visibility   string          `json:"visibility"`
	JoinPolicy   string          `json:"join_policy"`
	Timezone     string          `json:"timezone"`
	Settings     Settings        `json:"settings"`
	MemberCount  int             `json:"member_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	MyMembership *MembershipInfo `json:"my_membership"`
}

// Summary is the contract schema DormitorySummary.
type Summary struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	City        *string   `json:"city"`
	Description *string   `json:"description"`
	Visibility  string    `json:"visibility"`
	JoinPolicy  string    `json:"join_policy"`
	MemberCount int       `json:"member_count"`
	IsMember    bool      `json:"is_member"`
}

// SummaryPage is the contract schema DormitorySummaryPage.
type SummaryPage struct {
	Data       []Summary `json:"data"`
	NextCursor *string   `json:"next_cursor"`
}

// Membership is the contract schema Membership.
type Membership struct {
	Dormitory   Summary          `json:"dormitory"`
	Role        string           `json:"role"`
	Trusted     bool             `json:"is_trusted_reviewer"`
	JoinedAt    time.Time        `json:"joined_at"`
	Restriction *SanctionSummary `json:"restriction"`
}

// MembershipList is the contract schema MembershipList.
type MembershipList struct {
	Data []Membership `json:"data"`
}

// SettingsInput is the contract schema DormitorySettingsInput.
type SettingsInput struct {
	CommentsEnabled  *bool `json:"comments_enabled"`
	PhotosEnabled    *bool `json:"photos_enabled"`
	RatingWindowDays *int  `json:"rating_window_days"`
}

// CreateRequest is the contract schema CreateDormitoryRequest.
type CreateRequest struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	City        *string        `json:"city"`
	Address     *string        `json:"address"`
	Visibility  string         `json:"visibility"`
	JoinPolicy  string         `json:"join_policy"`
	Timezone    *string        `json:"timezone"`
	Settings    *SettingsInput `json:"settings"`
}
