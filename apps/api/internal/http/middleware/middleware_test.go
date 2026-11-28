// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct client, header ignored", "203.0.113.9:4000", "198.51.100.1", "203.0.113.9"},
		{"trusted proxy", "172.18.0.2:4000", "198.51.100.1", "198.51.100.1"},
		{"spoofed left-most hop ignored", "172.18.0.2:4000", "10.9.9.9, 198.51.100.1", "198.51.100.1"},
		{"chain of trusted proxies", "172.18.0.2:4000", "198.51.100.1, 172.18.0.3", "198.51.100.1"},
		{"garbage hop stops the walk", "172.18.0.2:4000", "198.51.100.1, nonsense", "172.18.0.2"},
		{"IPv4-mapped peer", "[::ffff:203.0.113.9]:4000", "", "203.0.113.9"},
	}
	for _, c := range cases {
		var got netip.Addr
		h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = IP(r.Context()) }))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if got.String() != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestTruncatedNetwork(t *testing.T) {
	for remote, want := range map[string]string{
		"203.0.113.9:1":            "203.0.113.0/24",
		"[2001:db8:1:2:3::4]:1":    "2001:db8:1::/48",
		"[::ffff:198.51.100.7]:80": "198.51.100.0/24",
	} {
		var got *string
		h := ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = TruncatedNetwork(r.Context()) }))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		h.ServeHTTP(httptest.NewRecorder(), req)
		if got == nil || *got != want {
			t.Errorf("%s: got %v, want %s", remote, got, want)
		}
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(func() time.Time { return now })
	for i := 1; i <= 3; i++ {
		if r := l.Allow("k", 3, time.Minute); !r.Allowed || r.Remaining != 3-i {
			t.Fatalf("request %d: %+v", i, r)
		}
	}
	if r := l.Allow("k", 3, time.Minute); r.Allowed {
		t.Fatal("fourth request allowed")
	}
	if r := l.Allow("other", 3, time.Minute); !r.Allowed {
		t.Fatal("keys are not independent")
	}
	now = now.Add(time.Minute)
	if r := l.Allow("k", 3, time.Minute); !r.Allowed {
		t.Fatal("window did not reset")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	l := NewLimiter(time.Now)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := ClientIP(nil)(RateLimit(l, logger, Rule{Bucket: "b", Limit: 1, Window: time.Minute, KeyFunc: ByIP})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })))
	send := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.1:1"
		h.ServeHTTP(rec, req)
		return rec
	}
	if r := send(); r.Code != 204 || r.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("first: %d %v", r.Code, r.Header())
	}
	if r := send(); r.Code != 429 || r.Header().Get("Retry-After") == "" {
		t.Fatalf("second: %d %v", r.Code, r.Header())
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		authz, key string
		ok         bool
	}{
		{"Bearer abc", "", true},
		{"bearer abc", "", true},
		{"Basic abc", "", false},
		{"Bearer ", "", false},
		{"", "", false},
		{"Bearer abc", "kk_x", false},
		{"", "kk_x", false},
	}
	for _, c := range cases {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if c.authz != "" {
			req.Header.Set("Authorization", c.authz)
		}
		if c.key != "" {
			req.Header.Set("X-API-Key", c.key)
		}
		tok, err := BearerToken(req)
		if (err == nil) != c.ok || (c.ok && tok != "abc") {
			t.Errorf("%q/%q: token %q err %v", c.authz, c.key, tok, err)
		}
	}
}

func TestRecoverHidesPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := RequestID(Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret detail") })))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Code != 500 {
		t.Fatalf("status %d", rec.Code)
	}
	if body := rec.Body.String(); body == "" || strings.Contains(body, "secret") {
		t.Fatalf("body leaks the panic: %s", body)
	}
}

func TestClientIPMultipleHeaderLines(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	var got netip.Addr
	h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = IP(r.Context()) }))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "172.18.0.2:4000"
	// The client sends a spoofed first line; the proxy appends the real address as a second line.
	req.Header.Add("X-Forwarded-For", "10.9.9.9")
	req.Header.Add("X-Forwarded-For", "198.51.100.1")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.String() != "198.51.100.1" {
		t.Fatalf("got %s, want the address appended by the proxy", got)
	}
}

func TestClientKeyGroupsIPv6(t *testing.T) {
	keys := map[string]string{}
	for _, remote := range []string{"[2001:db8:1:2::1]:1", "[2001:db8:1:2:ffff::9]:1", "203.0.113.9:1"} {
		h := ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { keys[remote] = ClientKey(r.Context()) }))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if keys["[2001:db8:1:2::1]:1"] != "2001:db8:1:2::/64" || keys["[2001:db8:1:2:ffff::9]:1"] != "2001:db8:1:2::/64" {
		t.Errorf("IPv6 keys: %v", keys)
	}
	if keys["203.0.113.9:1"] != "203.0.113.9" {
		t.Errorf("IPv4 key: %v", keys)
	}
}

func TestLimiterMemoryCap(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewLimiter(func() time.Time { return now })
	l.maxKeys = 3
	for _, k := range []string{"a", "b", "c"} {
		if !l.Allow(k, 10, time.Minute).Allowed {
			t.Fatalf("%s refused", k)
		}
	}
	if l.Allow("d", 10, time.Minute).Allowed {
		t.Fatal("a new key was accepted with a full table")
	}
	if !l.Allow("a", 10, time.Minute).Allowed {
		t.Fatal("an existing key was refused")
	}
	now = now.Add(2 * time.Minute)
	if !l.Allow("d", 10, time.Minute).Allowed {
		t.Fatal("expired windows were not dropped")
	}
	if !l.Peek("e", 1) || len(l.windows) != 1 {
		t.Fatal("Peek must not create windows")
	}
}
