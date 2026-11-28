// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package httpx contains JSON request and response helpers shared by the handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
)

// MaxJSONBody is the largest accepted JSON request body (docs/ARCHITECTURE.md §11).
const MaxJSONBody = 256 << 10

// DecodeJSON decodes a single JSON object into dst. Unknown fields, trailing data,
// a wrong content type and oversized bodies are client errors (400, 415, 413).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return apierror.New(http.StatusUnsupportedMediaType, apierror.UnsupportedMedia, "Content-Type must be application/json.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apierror.New(http.StatusRequestEntityTooLarge, apierror.PayloadTooLarge, "Request body is too large.")
		}
		return apierror.New(http.StatusBadRequest, apierror.BadRequest, "Request body is not valid JSON for this operation.")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apierror.New(http.StatusBadRequest, apierror.BadRequest, "Request body must contain a single JSON object.")
	}
	return nil
}

// WriteJSON writes v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// PathUUID parses a UUID path parameter value; malformed IDs are reported as 404 so that
// probing with invalid IDs reveals nothing.
func PathUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || len(value) != 36 {
		return uuid.Nil, apierror.ErrNotFound
	}
	return id, nil
}

// QueryLimit parses the optional limit parameter (default 20, 1..100).
func QueryLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 100 {
		return 0, apierror.New(http.StatusBadRequest, apierror.BadRequest, "limit must be an integer between 1 and 100.")
	}
	return n, nil
}
