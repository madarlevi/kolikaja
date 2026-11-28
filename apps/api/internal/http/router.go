// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package httpapi wires the HTTP router: middleware chain, rate limits, health probes and
// the implemented operations of docs/api/openapi.yaml under /api/v1.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madarlevi/kolikaja/apps/api/internal/auth"
	"github.com/madarlevi/kolikaja/apps/api/internal/config"
	"github.com/madarlevi/kolikaja/apps/api/internal/dormitories"
	"github.com/madarlevi/kolikaja/apps/api/internal/http/middleware"
	"github.com/madarlevi/kolikaja/apps/api/internal/menus"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/httpx"
	"github.com/madarlevi/kolikaja/apps/api/migrations"
)

// Deps are the dependencies of the router.
type Deps struct {
	Config        config.Config
	Pool          *pgxpool.Pool
	Logger        *slog.Logger
	Now           func() time.Time
	ServerVersion string
}

// requestTimeout bounds every request; shorter than the server's WriteTimeout (30 s).
const requestTimeout = 15 * time.Second

// Rate-limit buckets (docs/ARCHITECTURE.md §11).
func readRule() middleware.Rule {
	return middleware.Rule{Bucket: "read", Limit: 300, Window: time.Minute, KeyFunc: middleware.ByUserOrIP}
}

func writeRule() middleware.Rule {
	return middleware.Rule{Bucket: "write", Limit: 60, Window: time.Minute, KeyFunc: middleware.ByUserOrIP}
}

// NewRouter builds the HTTP handler.
func NewRouter(d Deps) (http.Handler, error) {
	cfg := d.Config
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTSecretPrevious, cfg.PublicBaseURL, cfg.AccessTokenTTL)
	limiter := middleware.NewLimiter(d.Now)
	hasher, err := auth.NewHasher(cfg.Argon2MaxConcurrency)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(d.Pool, hasher, tokens, cfg.RefreshTokenTTL, cfg.SessionMaxAge, d.Now)
	authH := auth.NewHandler(authSvc, limiter, d.Logger)
	dormH := dormitories.NewHandler(dormitories.NewService(d.Pool, cfg.DormitoryCreation, d.Now), d.Logger)
	menuH := menus.NewHandler(menus.NewService(d.Pool, d.Now), d.Logger)
	latest, err := migrations.Latest()
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIP(cfg.TrustedProxies))
	r.Use(middleware.Recover(d.Logger))
	r.Use(middleware.Logger(d.Logger))
	r.Use(middleware.Timeout(requestTimeout))
	r.Use(middleware.SecurityHeaders(strings.HasPrefix(cfg.PublicBaseURL, "https://")))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		apierror.Write(w, r, d.Logger, apierror.ErrNotFound)
	})
	// Contract operations that are not implemented yet answer 404 as well (docs/DECISIONS.md T-45).
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		apierror.Write(w, r, d.Logger, apierror.ErrNotFound)
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", readiness(d.Pool, latest, d.Now))

	authenticate := middleware.Authenticate(authSvc, d.Logger)
	rl := func(rules ...middleware.Rule) func(http.Handler) http.Handler {
		return middleware.RateLimit(limiter, d.Logger, rules...)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.With(rl(readRule())).Get("/meta", meta(cfg, d.ServerVersion))

		r.With(rl(middleware.Rule{Bucket: "register", Limit: 5, Window: time.Hour, KeyFunc: middleware.ByIP})).
			Post("/auth/register", authH.Register)
		r.With(rl(middleware.Rule{Bucket: "login", Limit: 20, Window: time.Minute, KeyFunc: middleware.ByIP})).
			Post("/auth/native/login", authH.LoginNative)
		r.With(rl(middleware.Rule{Bucket: "refresh", Limit: 30, Window: time.Minute, KeyFunc: middleware.ByIP})).
			Post("/auth/native/refresh", authH.RefreshNative)
		r.With(rl(middleware.Rule{Bucket: "logout", Limit: 60, Window: time.Minute, KeyFunc: middleware.ByIP})).
			Post("/auth/logout", authH.Logout)

		r.Group(func(r chi.Router) {
			r.Use(authenticate)
			r.Group(func(r chi.Router) {
				r.Use(rl(readRule()))
				r.Get("/me", authH.Me)
				r.Get("/me/dormitories", dormH.Mine)
				r.Get("/dormitories", dormH.Search)
				r.Get("/dormitories/{dormitoryId}", dormH.Get)
				r.Get("/dormitories/{dormitoryId}/menus", menuH.List)
				r.Get("/dormitories/{dormitoryId}/menus/today", menuH.Today)
				r.Get("/dormitories/{dormitoryId}/menus/week", menuH.Week)
				r.Get("/dormitories/{dormitoryId}/meals/{mealId}", menuH.Meal)
				r.Get("/dormitories/{dormitoryId}/meals/{mealId}/stats", menuH.MealStats)
			})
			r.Group(func(r chi.Router) {
				r.Use(rl(writeRule()))
				r.Post("/dormitories", dormH.Create)
				r.Post("/dormitories/{dormitoryId}/join", dormH.Join)
				r.Post("/dormitories/{dormitoryId}/menus", menuH.Create)
				r.Put("/dormitories/{dormitoryId}/meals/{mealId}/my-rating", menuH.PutRating)
				r.Delete("/dormitories/{dormitoryId}/meals/{mealId}/my-rating", menuH.DeleteRating)
				r.Put("/dormitories/{dormitoryId}/meals/{mealId}/my-recommendation", menuH.PutRecommendation)
				r.Delete("/dormitories/{dormitoryId}/meals/{mealId}/my-recommendation", menuH.DeleteRecommendation)
			})
		})
	})
	return r, nil
}

// InstanceMeta is the contract schema InstanceMeta.
type InstanceMeta struct {
	Name              string `json:"name"`
	APIVersion        string `json:"api_version"`
	ServerVersion     string `json:"server_version"`
	License           string `json:"license"`
	SourceCodeURL     string `json:"source_code_url"`
	DormitoryCreation string `json:"dormitory_creation"`
	Limits            struct {
		PhotoMaxBytes     int64    `json:"photo_max_bytes"`
		PhotoContentTypes []string `json:"photo_content_types"`
		PasswordMinLength int      `json:"password_min_length"`
		PasswordMaxLength int      `json:"password_max_length"`
	} `json:"limits"`
	Legal struct {
		PrivacyPolicyURL *string `json:"privacy_policy_url"`
		TermsURL         *string `json:"terms_url"`
		TrademarksURL    *string `json:"trademarks_url"`
		OperatorContact  *string `json:"operator_contact"`
	} `json:"legal"`
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func meta(cfg config.Config, version string) http.HandlerFunc {
	var m InstanceMeta
	m.Name = cfg.InstanceName
	m.APIVersion = "v1"
	m.ServerVersion = version
	m.License = "AGPL-3.0-only"
	m.SourceCodeURL = cfg.SourceCodeURL
	m.DormitoryCreation = cfg.DormitoryCreation
	m.Limits.PhotoMaxBytes = cfg.PhotoMaxBytes
	m.Limits.PhotoContentTypes = []string{"image/jpeg", "image/png", "image/webp"}
	m.Limits.PasswordMinLength = auth.PasswordMinLength
	m.Limits.PasswordMaxLength = auth.PasswordMaxLength
	m.Legal.PrivacyPolicyURL = optional(cfg.PrivacyPolicyURL)
	m.Legal.TermsURL = optional(cfg.TermsURL)
	m.Legal.TrademarksURL = optional(cfg.TrademarksURL)
	m.Legal.OperatorContact = optional(cfg.OperatorContact)
	return func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, m)
	}
}

type health struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// readiness reports ready when the database answers and its schema is at the version this
// build expects (the migrate job has run). The result is cached for a second so that a
// flood of probes cannot exhaust the connection pool.
func readiness(pool *pgxpool.Pool, latest uint, now func() time.Time) http.HandlerFunc {
	var (
		mu      sync.Mutex
		checked time.Time
		ready   bool
	)
	check := func(ctx context.Context) bool {
		mu.Lock()
		defer mu.Unlock()
		if t := now(); t.Sub(checked) < time.Second {
			return ready
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var version int64
		var dirty bool
		err := pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
		ready = err == nil && !dirty && version == int64(latest) // #nosec G115 -- migration versions are small (4 digits).
		checked = now()
		return ready
	}
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus, status := "ok", http.StatusOK
		if !check(r.Context()) {
			dbStatus, status = "unavailable", http.StatusServiceUnavailable
		}
		httpx.WriteJSON(w, status, health{Status: dbStatus, Checks: map[string]string{"database": dbStatus}})
	}
}
