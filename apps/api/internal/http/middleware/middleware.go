// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package middleware contains the HTTP middleware chain (docs/ARCHITECTURE.md §3.2):
// request ID, client IP, logging, panic recovery, security headers and authentication.
package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/platform/requestid"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// RequestID assigns a server-generated request ID (client-supplied IDs are ignored).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.New()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), id)))
	})
}

type ipKey struct{}

// ClientIP resolves the client address. X-Forwarded-For is used only when the direct
// peer is a trusted proxy; the right-most address that is not a trusted proxy is used.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			addr := peerAddr(r.RemoteAddr)
			if addr.IsValid() && isTrusted(addr) {
				// All header lines: some proxies append their value as a separate line.
				hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
				for i := len(hops) - 1; i >= 0; i-- {
					hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
					if err != nil {
						break
					}
					addr = hop.Unmap()
					if !isTrusted(addr) {
						break
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ipKey{}, addr)))
		})
	}
}

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// IP returns the client address resolved by ClientIP.
func IP(ctx context.Context) netip.Addr {
	a, _ := ctx.Value(ipKey{}).(netip.Addr)
	return a
}

// ClientKey is the rate-limit key of the client: the IPv4 address, or the /64 network of
// an IPv6 address (one subscriber usually controls a whole /64).
func ClientKey(ctx context.Context) string {
	a := IP(ctx)
	if a.Is6() {
		if p, err := a.Prefix(64); err == nil {
			return p.String()
		}
	}
	return a.String()
}

// TruncatedNetwork returns the client network stored for sessions and audit rows:
// IPv4 /24, IPv6 /48, never the full address.
func TruncatedNetwork(ctx context.Context) *string {
	a := IP(ctx)
	if !a.IsValid() {
		return nil
	}
	bits := 24
	if a.Is6() {
		bits = 48
	}
	p, err := a.Prefix(bits)
	if err != nil {
		return nil
	}
	s := p.String()
	return &s
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger logs one line per request: method, route pattern, status, duration, request ID
// and user ID. Headers, bodies, tokens and query strings are never logged.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			attrs := []any{
				"method", r.Method, "route", route, "status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(), "request_id", requestid.From(r.Context()),
			}
			if p, ok := principal.From(r.Context()); ok {
				attrs = append(attrs, "user_id", p.UserID.String())
			}
			logger.InfoContext(r.Context(), "request", attrs...)
		})
	}
}

// Recover turns panics into a generic 500 without exposing details.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as panic value
						panic(v)
					}
					logger.ErrorContext(r.Context(), "panic", "request_id", requestid.From(r.Context()), "panic", v)
					apierror.Write(w, r, logger, apierror.New(http.StatusInternalServerError, apierror.Internal, "Internal server error."))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout bounds every request with a context deadline (docs/ARCHITECTURE.md §11), so work
// waiting for the database or an Argon2 slot stops when the deadline passes.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityHeaders sets the API response headers (docs/ARCHITECTURE.md §11).
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Embedder-Policy", "require-corp")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			h.Set("Cache-Control", "no-store")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Authenticator verifies a bearer token and returns the principal.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (principal.Principal, error)
}

// Authenticate requires exactly one credential: a bearer token. API keys and web cookies
// are not enabled in this version; sending more than one credential type is rejected
// (docs/ARCHITECTURE.md §4.6).
func Authenticate(auth Authenticator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := BearerToken(r)
			if err != nil {
				apierror.Write(w, r, logger, err)
				return
			}
			p, err := auth.Authenticate(r.Context(), token)
			if err != nil {
				apierror.Write(w, r, logger, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(principal.With(r.Context(), p)))
		})
	}
}

// BearerToken extracts the bearer token and enforces the one-credential rule.
func BearerToken(r *http.Request) (string, error) {
	authz := r.Header.Get("Authorization")
	hasKey := r.Header.Get("X-API-Key") != ""
	if authz != "" && hasKey {
		return "", apierror.New(http.StatusBadRequest, apierror.AmbiguousCredentials, "Send only one kind of credential.")
	}
	if hasKey {
		return "", apierror.Unauthorized(apierror.Unauthenticated, "API keys are not enabled on this server yet.")
	}
	scheme, token, ok := strings.Cut(authz, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 4096 {
		return "", apierror.ErrUnauthenticated
	}
	return strings.TrimSpace(token), nil
}
