// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package auth implements registration, native login, refresh-token rotation with reuse
// detection, logout, the current user and request authentication
// (docs/ARCHITECTURE.md §4). Web cookie authentication follows with the web app.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// RegisterRequest is the contract schema RegisterRequest.
type RegisterRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DisplayName string  `json:"display_name"`
	Locale      *string `json:"locale"`
}

// NativeLoginRequest is the contract schema NativeLoginRequest.
type NativeLoginRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DeviceLabel *string `json:"device_label"`
}

// NativeRefreshRequest is the contract schema NativeRefreshRequest.
type NativeRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// NotificationSettings is the contract schema NotificationSettings.
type NotificationSettings struct {
	Comments      bool `json:"comments"`
	Announcements bool `json:"announcements"`
}

// SanctionSummary is the contract schema SanctionSummary.
type SanctionSummary struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	DormitoryID *uuid.UUID `json:"dormitory_id"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
}

// CurrentUser is the contract schema CurrentUser.
type CurrentUser struct {
	ID                   uuid.UUID            `json:"id"`
	Email                string               `json:"email"`
	DisplayName          string               `json:"display_name"`
	Locale               string               `json:"locale"`
	Avatar               *struct{}            `json:"avatar"`
	IsGlobalAdmin        bool                 `json:"is_global_admin"`
	NotificationSettings NotificationSettings `json:"notification_settings"`
	Restriction          *SanctionSummary     `json:"restriction"`
	CreatedAt            time.Time            `json:"created_at"`
	PasswordChangedAt    *time.Time           `json:"password_changed_at"`
}

// SessionInfo is the contract schema SessionInfo.
type SessionInfo struct {
	ID        uuid.UUID `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NativeTokens is the contract schema NativeTokens.
type NativeTokens struct {
	TokenType             string      `json:"token_type"`
	AccessToken           string      `json:"access_token"`
	AccessTokenExpiresAt  time.Time   `json:"access_token_expires_at"`
	RefreshToken          string      `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time   `json:"refresh_token_expires_at"`
	Session               SessionInfo `json:"session"`
	User                  CurrentUser `json:"user"`
}

// ClientInfo describes where a login comes from (stored truncated, never the full IP).
type ClientInfo struct {
	UserAgent string
	Network   *string // truncated CIDR, e.g. 203.0.113.0/24
	RequestID string
}
