// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package menus

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/httpx"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// Handler exposes the menu, meal, rating and recommendation operations.
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

// ids parses the dormitory ID and, when name is not empty, a second path UUID.
func ids(r *http.Request, name string) (principal.Principal, uuid.UUID, uuid.UUID, error) {
	p, _ := principal.From(r.Context())
	dormitoryID, err := httpx.PathUUID(chi.URLParam(r, "dormitoryId"))
	if err != nil || name == "" {
		return p, dormitoryID, uuid.Nil, err
	}
	other, err := httpx.PathUUID(chi.URLParam(r, name))
	return p, dormitoryID, other, err
}

// List handles GET /dormitories/{dormitoryId}/menus.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, _, err := ids(r, "")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	includeDrafts := false
	switch q.Get("include_drafts") {
	case "", "false":
	case "true":
		includeDrafts = true
	default:
		h.fail(w, r, apierror.BadRequestf("include_drafts must be true or false."))
		return
	}
	list, err := h.svc.List(r.Context(), p, dormitoryID, q.Get("from"), q.Get("to"), includeDrafts)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Create handles POST /dormitories/{dormitoryId}/menus.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, _, err := ids(r, "")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req CreateMenuRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	menu, err := h.svc.Create(r.Context(), p, dormitoryID, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, menu)
}

// Today handles GET /dormitories/{dormitoryId}/menus/today.
func (h *Handler) Today(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, _, err := ids(r, "")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	day, err := h.svc.Today(r.Context(), p, dormitoryID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, day)
}

// Week handles GET /dormitories/{dormitoryId}/menus/week.
func (h *Handler) Week(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, _, err := ids(r, "")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	week, err := h.svc.Week(r.Context(), p, dormitoryID, r.URL.Query().Get("date"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, week)
}

// Meal handles GET /dormitories/{dormitoryId}/meals/{mealId}.
func (h *Handler) Meal(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	meal, err := h.svc.Meal(r.Context(), p, dormitoryID, mealID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meal)
}

// MealStats handles GET /dormitories/{dormitoryId}/meals/{mealId}/stats.
func (h *Handler) MealStats(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	st, err := h.svc.MealStats(r.Context(), p, dormitoryID, mealID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

// PutRating handles PUT /dormitories/{dormitoryId}/meals/{mealId}/my-rating.
func (h *Handler) PutRating(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req RatingRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, created, err := h.svc.PutRating(r.Context(), p, dormitoryID, mealID, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, res)
}

// DeleteRating handles DELETE /dormitories/{dormitoryId}/meals/{mealId}/my-rating.
func (h *Handler) DeleteRating(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteRating(r.Context(), p, dormitoryID, mealID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PutRecommendation handles PUT /dormitories/{dormitoryId}/meals/{mealId}/my-recommendation.
func (h *Handler) PutRecommendation(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req RecommendationRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, created, err := h.svc.PutRecommendation(r.Context(), p, dormitoryID, mealID, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, res)
}

// DeleteRecommendation handles DELETE /dormitories/{dormitoryId}/meals/{mealId}/my-recommendation.
func (h *Handler) DeleteRecommendation(w http.ResponseWriter, r *http.Request) {
	p, dormitoryID, mealID, err := ids(r, "mealId")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteRecommendation(r.Context(), p, dormitoryID, mealID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
