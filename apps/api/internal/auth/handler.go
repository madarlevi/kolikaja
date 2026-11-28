// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/madarlevi/kolikaja/apps/api/internal/http/middleware"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/httpx"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/requestid"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// Handler exposes the authentication operations of the contract.
type Handler struct {
	svc     *Service
	limiter *middleware.Limiter
	logger  *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, limiter *middleware.Limiter, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, limiter: limiter, logger: logger}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	apierror.Write(w, r, h.logger, err)
}

func clientInfo(r *http.Request) ClientInfo {
	return ClientInfo{
		UserAgent: r.UserAgent(),
		Network:   middleware.TruncatedNetwork(r.Context()),
		RequestID: requestid.From(r.Context()),
	}
}

// Register handles POST /auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	user, err := h.svc.Register(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, user)
}

// maxFailuresPerAccount caps wrong-password attempts per account per hour across all
// client addresses (docs/ARCHITECTURE.md §11).
const maxFailuresPerAccount = 100

// LoginNative handles POST /auth/native/login. Besides the per-IP route limit, attempts
// are limited per client and account, and failures per account across all clients.
func (h *Handler) LoginNative(w http.ResponseWriter, r *http.Request) {
	failed := false
	var req NativeLoginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email != "" && len(email) <= 254 {
		// Keys hold a hash, never the raw input, so their size is bounded.
		sum := sha256.Sum256([]byte(email))
		account := hex.EncodeToString(sum[:16])
		failures := "login-failures|" + account
		perClient := "login-account|" + middleware.ClientKey(r.Context()) + "|" + account
		if !h.limiter.Peek(failures, maxFailuresPerAccount) || !h.limiter.Allow(perClient, 5, time.Minute).Allowed {
			w.Header().Set("Retry-After", "60")
			h.fail(w, r, apierror.New(http.StatusTooManyRequests, apierror.RateLimited, "Too many login attempts. Try again later."))
			return
		}
		defer func() {
			if failed {
				h.limiter.Allow(failures, maxFailuresPerAccount, time.Hour)
			}
		}()
	}
	tokens, err := h.svc.LoginNative(r.Context(), req, clientInfo(r))
	if err != nil {
		failed = errors.Is(err, errInvalidCredentials)
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokens)
}

// RefreshNative handles POST /auth/native/refresh.
func (h *Handler) RefreshNative(w http.ResponseWriter, r *http.Request) {
	var req NativeRefreshRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	tokens, err := h.svc.RefreshNative(r.Context(), req, clientInfo(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokens)
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token, err := middleware.BearerToken(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), token); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	user, err := h.svc.Me(r.Context(), p.UserID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}
