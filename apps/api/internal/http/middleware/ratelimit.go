// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/madarlevi/kolikaja/apps/api/internal/platform/apierror"
	"github.com/madarlevi/kolikaja/apps/api/internal/principal"
)

// MaxLimiterKeys bounds the memory of the limiter. When the table is full and expired
// windows cannot be dropped, new keys are refused (fail closed with 429).
const MaxLimiterKeys = 200_000

// Limiter is an in-memory fixed-window rate limiter for a single API instance
// (docs/ARCHITECTURE.md §11). Each bucket has its own limit and window.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	now     func() time.Time
	lastGC  time.Time
	maxKeys int
}

type window struct {
	start time.Time
	count int
	size  time.Duration
}

// NewLimiter returns a limiter.
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{windows: map[string]*window{}, now: now, maxKeys: MaxLimiterKeys}
}

// Result of one rate-limit decision.
type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	Reset     time.Duration
}

// Allow counts one request for key in a bucket with limit per window.
func (l *Limiter) Allow(key string, limit int, size time.Duration) Result {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastGC) > time.Minute {
		l.gc(now)
	}
	w, ok := l.windows[key]
	if !ok && len(l.windows) >= l.maxKeys {
		l.gc(now)
		if len(l.windows) >= l.maxKeys {
			return Result{Allowed: false, Limit: limit, Reset: size}
		}
	}
	if !ok || now.Sub(w.start) >= size {
		w = &window{start: now, size: size}
		l.windows[key] = w
	}
	w.count++
	reset := size - now.Sub(w.start)
	remaining := limit - w.count
	if remaining < 0 {
		remaining = 0
	}
	return Result{Allowed: w.count <= limit, Limit: limit, Remaining: remaining, Reset: reset}
}

// Peek reports whether key is still under limit without counting a request.
func (l *Limiter) Peek(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[key]
	return !ok || l.now().Sub(w.start) >= w.size || w.count < limit
}

func (l *Limiter) gc(now time.Time) {
	for k, w := range l.windows {
		if now.Sub(w.start) >= w.size {
			delete(l.windows, k)
		}
	}
	l.lastGC = now
}

// Rule is one limit; KeyFunc returns the key, or "" to skip the rule.
type Rule struct {
	Bucket  string
	Limit   int
	Window  time.Duration
	KeyFunc func(*http.Request) string
}

// ByIP keys a rule by client address (IPv6 by its /64, see ClientKey).
func ByIP(r *http.Request) string { return ClientKey(r.Context()) }

// ByUserOrIP keys a rule by user ID when authenticated, otherwise by client address.
func ByUserOrIP(r *http.Request) string {
	if p, ok := principal.From(r.Context()); ok {
		return "user:" + p.UserID.String()
	}
	return "ip:" + ClientKey(r.Context())
}

// RateLimit enforces the rules in order; the most restrictive result sets the headers.
func RateLimit(l *Limiter, logger *slog.Logger, rules ...Rule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tightest *Result
			for _, rule := range rules {
				key := rule.KeyFunc(r)
				if key == "" {
					continue
				}
				res := l.Allow(rule.Bucket+"|"+key, rule.Limit, rule.Window)
				if tightest == nil || res.Remaining < tightest.Remaining || !res.Allowed {
					decision := res
					tightest = &decision
				}
				if !res.Allowed {
					break
				}
			}
			if tightest != nil {
				h := w.Header()
				h.Set("RateLimit-Limit", strconv.Itoa(tightest.Limit))
				h.Set("RateLimit-Remaining", strconv.Itoa(tightest.Remaining))
				h.Set("RateLimit-Reset", strconv.Itoa(int(tightest.Reset.Seconds()+0.999)))
				if !tightest.Allowed {
					h.Set("Retry-After", strconv.Itoa(int(tightest.Reset.Seconds()+0.999)))
					apierror.Write(w, r, logger, apierror.New(http.StatusTooManyRequests, apierror.RateLimited, "Too many requests. Try again later."))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
