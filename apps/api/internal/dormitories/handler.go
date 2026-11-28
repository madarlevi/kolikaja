// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package dormitories

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/httpx"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// Handler exposes the dormitory operations.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandler returns the handler.
func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	apierror.Write(w, r, h.logger, err)
}

// Search handles GET /dormitories.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	limit, err := httpx.QueryLimit(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	page, err := h.svc.Search(r.Context(), p, q.Get("q"), q.Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

// Create handles POST /dormitories.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	var req CreateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.Create(r.Context(), p, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

// Get handles GET /dormitories/{dormitoryId}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, err := httpx.PathUUID(chi.URLParam(r, "dormitoryId"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// Join handles POST /dormitories/{dormitoryId}/join.
func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	id, err := httpx.PathUUID(chi.URLParam(r, "dormitoryId"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.Join(r.Context(), p, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, m)
}

// Mine handles GET /me/dormitories.
func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	list, err := h.svc.Mine(r.Context(), p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}
