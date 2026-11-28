// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package config reads the configuration from environment variables once at start-up
// (docs/ARCHITECTURE.md §3.4). Every secret can also be read from a file named by
// <NAME>_FILE (used by Docker Compose). Missing or weak secrets stop the process; there
// are no default secrets.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the effective configuration.
type Config struct {
	Env                  string
	HTTPAddr             string
	PublicBaseURL        string
	DatabaseURL          string
	DBMaxConns           int32
	DBStatementTimeout   time.Duration
	JWTSecret            []byte
	JWTSecretPrevious    []byte
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	SessionMaxAge        time.Duration
	DormitoryCreation    string
	InstanceName         string
	SourceCodeURL        string
	PrivacyPolicyURL     string
	TermsURL             string
	TrademarksURL        string
	OperatorContact      string
	TrustedProxies       []netip.Prefix
	LogLevel             slog.Level
	Argon2MaxConcurrency int
	PhotoMaxBytes        int64
}

// Getenv reads an environment variable; ReadFile reads a secret file. Both are
// injectable for tests.
type (
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
)

// FromEnvironment loads the configuration from the process environment.
func FromEnvironment() (*Config, error) {
	return Load(os.Getenv, func(p string) ([]byte, error) { return os.ReadFile(filepath.Clean(p)) })
}

// LoadDatabaseURL loads only the database settings (used by cmd/migrate and cmd/admin).
func LoadDatabaseURL(getenv Getenv, readFile ReadFile) (string, error) {
	l := loader{getenv: getenv, readFile: readFile}
	dsn := l.databaseURL()
	return dsn, errors.Join(l.errs...)
}

// Load builds and validates the configuration.
func Load(getenv Getenv, readFile ReadFile) (*Config, error) {
	l := loader{getenv: getenv, readFile: readFile}
	c := &Config{
		Env:                  l.str("KOLIKAJA_ENV", "production"),
		HTTPAddr:             l.str("HTTP_ADDR", ":8080"),
		PublicBaseURL:        strings.TrimRight(l.required("PUBLIC_BASE_URL"), "/"),
		DBMaxConns:           int32(l.intRange("DB_MAX_CONNS", 10, 1, 200)), // #nosec G115 -- bounded to 1..200 by intRange.
		DBStatementTimeout:   l.duration("DB_STATEMENT_TIMEOUT", 5*time.Second),
		AccessTokenTTL:       l.duration("ACCESS_TOKEN_TTL", 10*time.Minute),
		RefreshTokenTTL:      l.duration("REFRESH_TOKEN_TTL", 720*time.Hour),
		SessionMaxAge:        l.duration("SESSION_MAX_AGE", 2160*time.Hour),
		DormitoryCreation:    l.str("DORMITORY_CREATION", "global_admins"),
		InstanceName:         l.str("INSTANCE_NAME", "KoliKaja"),
		SourceCodeURL:        l.str("SOURCE_CODE_URL", "https://github.com/madarlevi/kolikaja"),
		PrivacyPolicyURL:     l.str("PRIVACY_POLICY_URL", ""),
		TermsURL:             l.str("TERMS_URL", ""),
		TrademarksURL:        l.str("TRADEMARKS_URL", ""),
		OperatorContact:      l.str("OPERATOR_CONTACT", ""),
		Argon2MaxConcurrency: l.intRange("ARGON2_MAX_CONCURRENCY", 4, 1, 64),
		PhotoMaxBytes:        int64(l.intRange("PHOTO_MAX_BYTES", 10<<20, 1<<20, 25<<20)),
	}
	c.DatabaseURL = l.databaseURL()
	c.JWTSecret = l.secret("JWT_SECRET", true)
	c.JWTSecretPrevious = l.secret("JWT_SECRET_PREVIOUS", false)
	c.TrustedProxies = l.prefixes("TRUSTED_PROXIES")
	c.LogLevel = l.logLevel("LOG_LEVEL")

	if c.Env != "production" && c.Env != "development" {
		l.fail("KOLIKAJA_ENV must be production or development")
	}
	if c.DormitoryCreation != "global_admins" && c.DormitoryCreation != "all_users" {
		l.fail("DORMITORY_CREATION must be global_admins or all_users")
	}
	if u, err := url.Parse(c.PublicBaseURL); c.PublicBaseURL != "" && (err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "") {
		l.fail("PUBLIC_BASE_URL must be an absolute http(s) URL")
	}
	if u, err := url.Parse(c.SourceCodeURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		l.fail("SOURCE_CODE_URL must be an absolute http(s) URL (the AGPL source offer shown to users)")
	}
	if c.AccessTokenTTL < time.Minute || c.AccessTokenTTL > time.Hour {
		l.fail("ACCESS_TOKEN_TTL must be between 1m and 1h")
	}
	if c.RefreshTokenTTL > c.SessionMaxAge {
		l.fail("REFRESH_TOKEN_TTL must not exceed SESSION_MAX_AGE")
	}
	if err := errors.Join(l.errs...); err != nil {
		return nil, err
	}
	return c, nil
}

// Warnings returns configuration that works but deserves the operator's attention.
func (c *Config) Warnings() []string {
	var out []string
	if c.Env == "production" && len(c.TrustedProxies) == 0 {
		out = append(out, "TRUSTED_PROXIES is empty: X-Forwarded-For is ignored. This is right when clients "+
			"connect directly; behind a reverse proxy set it to the proxy's address range.")
	}
	if u, err := url.Parse(c.PublicBaseURL); err == nil && c.Env == "production" && u.Scheme == "http" {
		if host := u.Hostname(); host != "localhost" && host != "127.0.0.1" && host != "::1" {
			out = append(out, "PUBLIC_BASE_URL uses plain http: passwords and tokens cross the network unencrypted. "+
				"Put a TLS reverse proxy in front of the API for anything but local testing.")
		}
	}
	return out
}

// LogValue reports the effective non-secret configuration; secrets are never logged.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.String("http_addr", c.HTTPAddr),
		slog.String("public_base_url", c.PublicBaseURL),
		slog.Int("db_max_conns", int(c.DBMaxConns)),
		slog.Duration("access_token_ttl", c.AccessTokenTTL),
		slog.Duration("refresh_token_ttl", c.RefreshTokenTTL),
		slog.Duration("session_max_age", c.SessionMaxAge),
		slog.String("dormitory_creation", c.DormitoryCreation),
		slog.Int("trusted_proxies", len(c.TrustedProxies)),
		slog.Bool("jwt_previous_key", len(c.JWTSecretPrevious) > 0),
	)
}

type loader struct {
	getenv   Getenv
	readFile ReadFile
	errs     []error
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf("config: "+format, args...))
}

func (l *loader) str(name, def string) string {
	if v := strings.TrimSpace(l.getenv(name)); v != "" {
		return v
	}
	return def
}

func (l *loader) required(name string) string {
	v := strings.TrimSpace(l.getenv(name))
	if v == "" {
		l.fail("%s is required", name)
	}
	return v
}

// value returns NAME, or the content of the file named by NAME_FILE.
func (l *loader) value(name string) string {
	if path := strings.TrimSpace(l.getenv(name + "_FILE")); path != "" {
		data, err := l.readFile(path)
		if err != nil {
			l.fail("%s_FILE cannot be read", name)
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	return strings.TrimSpace(l.getenv(name))
}

func (l *loader) secret(name string, required bool) []byte {
	v := l.value(name)
	switch {
	case v == "" && required:
		l.fail("%s (or %s_FILE) is required; generate at least 32 random bytes", name, name)
	case v != "" && len(v) < 32:
		l.fail("%s must be at least 32 characters", name)
	}
	if v == "" {
		return nil
	}
	return []byte(v)
}

func (l *loader) databaseURL() string {
	if v := l.value("DATABASE_URL"); v != "" {
		return v
	}
	host := strings.TrimSpace(l.getenv("DB_HOST"))
	if host == "" {
		l.fail("DATABASE_URL or DB_HOST/DB_USER/DB_NAME/DB_PASSWORD(_FILE) is required")
		return ""
	}
	user := l.required("DB_USER")
	name := l.required("DB_NAME")
	password := l.value("DB_PASSWORD")
	if password == "" {
		l.fail("DB_PASSWORD or DB_PASSWORD_FILE is required")
	}
	port := l.str("DB_PORT", "5432")
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		l.fail("DB_PORT must be a port number")
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     host + ":" + port,
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {l.str("DB_SSLMODE", "prefer")}}.Encode(),
	}
	return u.String()
}

func (l *loader) intRange(name string, def, minV, maxV int) int {
	raw := strings.TrimSpace(l.getenv(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minV || n > maxV {
		l.fail("%s must be an integer between %d and %d", name, minV, maxV)
		return def
	}
	return n
}

func (l *loader) duration(name string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(l.getenv(name))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		l.fail("%s must be a positive duration such as 10m", name)
		return def
	}
	return d
}

func (l *loader) prefixes(name string) []netip.Prefix {
	var out []netip.Prefix
	for _, part := range strings.Split(l.getenv(name), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			l.fail("%s contains an invalid CIDR %q", name, part)
			continue
		}
		out = append(out, p.Masked())
	}
	return out
}

func (l *loader) logLevel(name string) slog.Level {
	switch strings.ToLower(l.str(name, "info")) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		l.fail("%s must be debug, info, warn or error", name)
		return slog.LevelInfo
	}
}
