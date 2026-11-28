// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package apierror defines the error envelope of the API contract and maps domain errors
// to HTTP responses. Responses never contain stack traces, SQL or internal details.
package apierror

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/requestid"
)

// Code is an error code of the contract (ErrorCode schema).
type Code string

// Error codes used by the API.
const (
	BadRequest           Code = "bad_request"
	AmbiguousCredentials Code = "ambiguous_credentials"
	Unauthenticated      Code = "unauthenticated"
	InvalidCredentials   Code = "invalid_credentials" // #nosec G101 -- error code name, not a credential.
	InvalidToken         Code = "invalid_token"
	SessionRevoked       Code = "session_revoked"
	Forbidden            Code = "forbidden"
	Suspended            Code = "suspended"
	Banned               Code = "banned"
	AccountBanned        Code = "account_banned"
	InvitationRequired   Code = "invitation_required"
	NotFound             Code = "not_found"
	Conflict             Code = "conflict"
	AlreadyExists        Code = "already_exists"
	AlreadyMember        Code = "already_member"
	RatingClosed         Code = "rating_closed"
	ValidationFailed     Code = "validation_failed"
	PayloadTooLarge      Code = "payload_too_large"
	UnsupportedMedia     Code = "unsupported_media_type"
	RateLimited          Code = "rate_limited"
	Internal             Code = "internal_error"
	ServiceUnavailable   Code = "service_unavailable"
)

// Detail names one invalid field (ErrorDetail schema).
type Detail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Error is an error that maps to a specific HTTP status and contract code.
type Error struct {
	Status  int
	Code    Code
	Message string
	Details []Detail
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// New returns an Error.
func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Validation returns a 422 validation_failed error with field details.
func Validation(details ...Detail) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: ValidationFailed, Message: "Some fields are invalid.", Details: details}
}

// ErrNotFound is the generic 404, also used for resources of dormitories the caller may not see.
var ErrNotFound = New(http.StatusNotFound, NotFound, "Resource not found.")

// ErrUnauthenticated is the generic 401.
var ErrUnauthenticated = New(http.StatusUnauthorized, Unauthenticated, "Authentication required.")

// Forbiddenf returns a 403 forbidden error.
func Forbiddenf(message string) *Error { return New(http.StatusForbidden, Forbidden, message) }

// ForbiddenCode returns a 403 with a specific code (suspended, banned, ...).
func ForbiddenCode(code Code, message string) *Error { return New(http.StatusForbidden, code, message) }

// Unauthorized returns a 401 with a specific code.
func Unauthorized(code Code, message string) *Error {
	return New(http.StatusUnauthorized, code, message)
}

// ConflictCode returns a 409 with a specific code.
func ConflictCode(code Code, message string) *Error { return New(http.StatusConflict, code, message) }

// BadRequestf returns a 400 bad_request error.
func BadRequestf(message string) *Error { return New(http.StatusBadRequest, BadRequest, message) }

type envelope struct {
	Error body `json:"error"`
}

type body struct {
	Code      Code     `json:"code"`
	Message   string   `json:"message"`
	Details   []Detail `json:"details,omitempty"`
	RequestID string   `json:"request_id"`
}

// Write writes err as the error envelope. Errors that are not *Error become a generic 500;
// their text is logged with the request ID and never sent to the client.
func Write(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	var apiErr *Error
	switch {
	case errors.As(err, &apiErr):
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		// The per-request timeout expired (or the client went away): busy, not broken.
		logger.WarnContext(r.Context(), "request timed out", "request_id", requestid.From(r.Context()))
		apiErr = New(http.StatusServiceUnavailable, ServiceUnavailable, "The server is busy. Try again shortly.")
	default:
		logger.ErrorContext(r.Context(), "internal error", "request_id", requestid.From(r.Context()), "error", err)
		apiErr = New(http.StatusInternalServerError, Internal, "Internal server error.")
	}
	if apiErr.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(apiErr.Status)
	_ = json.NewEncoder(w).Encode(envelope{Error: body{
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		Details:   apiErr.Details,
		RequestID: requestid.From(r.Context()),
	}})
}
