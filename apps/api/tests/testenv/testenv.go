// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package testenv runs the real HTTP router against a disposable PostgreSQL database for
// the authorization and flow tests. It is configured by KOLIKAJA_TEST_DATABASE_URL, whose
// database name must end with _test, _ci or _it because the schema is reset.
package testenv

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/madarlevi/kolikaja/apps/api/internal/config"
	httpapi "github.com/madarlevi/kolikaja/apps/api/internal/http"
	"github.com/madarlevi/kolikaja/apps/api/migrations"
)

// EnvDatabaseURL names the variable with the test database URL.
const EnvDatabaseURL = "KOLIKAJA_TEST_DATABASE_URL"

// V1 is the API base path.
const V1 = "/api/v1"

// Clock is an adjustable clock for expiry tests.
type Clock struct {
	mu     sync.Mutex
	offset time.Duration
}

// Now returns the current time plus the offset.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Add(c.offset)
}

// Advance moves the clock forward.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset += d
}

// Env is a running test API.
type Env struct {
	Pool    *pgxpool.Pool
	Handler http.Handler
	Clock   *Clock
	Config  config.Config
}

var disposableSuffixes = []string{"_test", "_ci", "_it"}

// Setup migrates a fresh schema and builds the router. It returns (nil, nil) when the
// database URL is not set, so that TestMain can skip.
func Setup(cfgOverride func(*config.Config)) (*Env, error) {
	dsn := os.Getenv(EnvDatabaseURL)
	if dsn == "" {
		return nil, nil
	}
	ctx := context.Background()
	if err := guard(ctx, dsn); err != nil {
		return nil, err
	}
	if err := resetSchema(dsn); err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	cfg := config.Config{
		Env:                  "development",
		HTTPAddr:             ":0",
		PublicBaseURL:        "http://kolikaja.test",
		DatabaseURL:          dsn,
		DBMaxConns:           10,
		DBStatementTimeout:   5 * time.Second,
		JWTSecret:            []byte(hex.EncodeToString(secret)),
		AccessTokenTTL:       10 * time.Minute,
		RefreshTokenTTL:      720 * time.Hour,
		SessionMaxAge:        2160 * time.Hour,
		DormitoryCreation:    "global_admins",
		InstanceName:         "KoliKaja test",
		SourceCodeURL:        "https://github.com/madarlevi/kolikaja",
		LogLevel:             slog.LevelError,
		Argon2MaxConcurrency: 8,
		PhotoMaxBytes:        10 << 20,
	}
	if cfgOverride != nil {
		cfgOverride(&cfg)
	}
	clock := &Clock{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := httpapi.NewRouter(httpapi.Deps{Config: cfg, Pool: pool, Logger: logger, Now: clock.Now, ServerVersion: "test"})
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Env{Pool: pool, Handler: h, Clock: clock, Config: cfg}, nil
}

// Main is a TestMain helper: it sets up the environment, runs the tests and cleans up.
func Main(m *testing.M, env **Env, cfgOverride func(*config.Config)) {
	e, err := Setup(cfgOverride)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testenv:", err)
		os.Exit(1)
	}
	*env = e
	code := m.Run()
	if e != nil {
		e.Pool.Close()
	}
	os.Exit(code)
}

// Require skips the test when no database is configured.
func Require(t *testing.T, e *Env) {
	t.Helper()
	if e == nil {
		t.Skipf("%s is not set", EnvDatabaseURL)
	}
}

func guard(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	var name string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		return err
	}
	for _, s := range disposableSuffixes {
		if strings.HasSuffix(name, s) {
			return nil
		}
	}
	return fmt.Errorf("refusing to reset database %q: its name must end with one of %v", name, disposableSuffixes)
}

func resetSchema(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	target := dsn
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(target, scheme) {
			target = "pgx5://" + strings.TrimPrefix(target, scheme)
		}
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, target)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return m.Up()
}

// Response is a recorded HTTP response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Decode unmarshals the JSON body.
func (r Response) Decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %d response: %v; body: %s", r.Status, err, r.Body)
	}
}

// ErrorCode returns error.code of an error envelope, or "".
func (r Response) ErrorCode() string {
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.Body, &env)
	return env.Error.Code
}

// RequestOption changes a request before it is sent.
type RequestOption func(*http.Request)

// WithHeader sets a header.
func WithHeader(name, value string) RequestOption {
	return func(r *http.Request) { r.Header.Set(name, value) }
}

// FromAddr sets the peer address.
func FromAddr(addr string) RequestOption {
	return func(r *http.Request) { r.RemoteAddr = addr }
}

// Do sends a request. body may be nil, a string (sent raw) or a value encoded as JSON.
// Each request comes from a random peer address unless FromAddr is given, so that the
// per-IP rate limits do not interfere with unrelated tests.
func (e *Env) Do(t *testing.T, method, path, token string, body any, opts ...RequestOption) Response {
	t.Helper()
	var rd io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			rd = strings.NewReader(b)
		default:
			raw, err := json.Marshal(b)
			if err != nil {
				t.Fatal(err)
			}
			rd = bytes.NewReader(raw)
		}
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = randomAddr()
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	e.Handler.ServeHTTP(rec, req)
	return Response{Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

func randomAddr() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<24))
	if err != nil {
		return "10.0.0.1:40000"
	}
	v := n.Int64()
	return fmt.Sprintf("10.%d.%d.%d:40000", (v>>16)&0xff, (v>>8)&0xff, v&0xff)
}

// User is a registered and logged-in test user.
type User struct {
	ID       uuid.UUID
	Email    string
	Password string
	Token    string
	Refresh  string
}

// Tokens is the part of NativeTokens the tests use.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Session      struct {
		ID uuid.UUID `json:"id"`
	} `json:"session"`
	User struct {
		ID uuid.UUID `json:"id"`
	} `json:"user"`
}

// NewUser registers a user through the API and logs in with a native session.
func (e *Env) NewUser(t *testing.T) User {
	t.Helper()
	u := User{Email: "u" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@example.test", Password: "correct horse battery staple"}
	res := e.Do(t, http.MethodPost, V1+"/auth/register", "", map[string]any{
		"email": u.Email, "password": u.Password, "display_name": "Teszt Elek",
	})
	if res.Status != http.StatusCreated {
		t.Fatalf("register: %d %s", res.Status, res.Body)
	}
	tok := e.Login(t, u.Email, u.Password)
	u.ID, u.Token, u.Refresh = tok.User.ID, tok.AccessToken, tok.RefreshToken
	return u
}

// Login logs in with a native session.
func (e *Env) Login(t *testing.T, email, password string) Tokens {
	t.Helper()
	res := e.Do(t, http.MethodPost, V1+"/auth/native/login", "", map[string]any{"email": email, "password": password})
	if res.Status != http.StatusOK {
		t.Fatalf("login: %d %s", res.Status, res.Body)
	}
	var tok Tokens
	res.Decode(t, &tok)
	return tok
}

// Exec runs SQL in the test database.
func (e *Env) Exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

// NewDormitory creates a dormitory owned (administered) by owner.
func (e *Env) NewDormitory(t *testing.T, owner User, visibility, joinPolicy string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	slug := "d-" + strings.ReplaceAll(id.String(), "-", "")[:20]
	e.Exec(t, `INSERT INTO dormitories (id, slug, name, visibility, join_policy, timezone, created_by)
		VALUES ($1, $2, $3, $4, $5, 'Europe/Budapest', $6)`, id, slug, "Kollégium "+slug, visibility, joinPolicy, owner.ID)
	e.AddMember(t, id, owner, "admin")
	return id
}

// AddMember adds an active membership.
func (e *Env) AddMember(t *testing.T, dormitoryID uuid.UUID, u User, role string) {
	t.Helper()
	via := "open_join"
	if role == "admin" {
		via = "creator"
	}
	e.Exec(t, `INSERT INTO dormitory_members (id, dormitory_id, user_id, role, joined_via) VALUES ($1, $2, $3, $4, $5)`,
		uuid.Must(uuid.NewV7()), dormitoryID, u.ID, role, via)
}

// Today returns today's date in Europe/Budapest (the test dormitories' time zone).
func (e *Env) Today() time.Time {
	loc, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		loc = time.UTC
	}
	n := e.Clock.Now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// NewMenu creates a lunch menu with one meal and returns both IDs.
func (e *Env) NewMenu(t *testing.T, dormitoryID uuid.UUID, date time.Time, period, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	menuID, mealID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	var published *time.Time
	if status == "published" {
		now := time.Now()
		published = &now
	}
	e.Exec(t, `INSERT INTO menus (id, dormitory_id, menu_date, meal_period, status, published_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		menuID, dormitoryID, date, period, status, published)
	e.Exec(t, `INSERT INTO meals (id, dormitory_id, menu_id, name) VALUES ($1, $2, $3, 'Rakott krumpli')`, mealID, dormitoryID, menuID)
	return menuID, mealID
}

// Sanction adds an active sanction; dormitoryID nil means instance-wide.
func (e *Env) Sanction(t *testing.T, u User, dormitoryID *uuid.UUID, kind string, by User) {
	t.Helper()
	e.Exec(t, `INSERT INTO user_sanctions (id, user_id, dormitory_id, kind, reason, created_by) VALUES ($1, $2, $3, $4, 'test', $5)`,
		uuid.Must(uuid.NewV7()), u.ID, dormitoryID, kind, by.ID)
}

// Count returns a single integer from SQL.
func (e *Env) Count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
