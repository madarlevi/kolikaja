// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package integration holds tests that need a real PostgreSQL database.
// They are skipped unless KOLIKAJA_TEST_DATABASE_URL points at an EMPTY, disposable database
// whose name ends with _test, _ci or _it (for example the PostgreSQL service container of the
// CI "migrations" job); any other database name makes them fail before touching it.
package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

const envDatabaseURL = "KOLIKAJA_TEST_DATABASE_URL"

const latestVersion = 15

func databaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		t.Skipf("%s is not set", envDatabaseURL)
	}
	return dsn
}

func newMigrate(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()
	src, err := iofs.New(os.DirFS(filepath.Join("..", "..", "migrations")), ".")
	if err != nil {
		t.Fatal(err)
	}
	target := dsn
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(target, scheme) {
			target = "pgx5://" + strings.TrimPrefix(target, scheme)
		}
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

// disposableSuffixes are the database name endings these destructive tests accept.
var disposableSuffixes = []string{"_test", "_ci", "_it"}

// connect opens a connection and refuses to continue unless the database is clearly a
// disposable test database, because the tests drop every table.
func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	var name string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range disposableSuffixes {
		if strings.HasSuffix(name, suffix) {
			return conn
		}
	}
	t.Fatalf("refusing to run destructive migration tests on database %q: its name must end with one of %v", name, disposableSuffixes)
	return nil
}

func ignoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

// TestMigrationsUpDownUp applies all migrations, reverts them one by one and applies
// them again, and requires the schema to be identical every time.
func TestMigrationsUpDownUp(t *testing.T) {
	dsn := databaseURL(t)
	conn := connect(t, dsn)
	m := newMigrate(t, dsn)
	ctx := context.Background()

	if err := ignoreNoChange(m.Down()); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if v, dirty, err := m.Version(); err != nil || v != latestVersion || dirty {
		t.Fatalf("version = %d dirty=%v err=%v, want %d clean", v, dirty, err, latestVersion)
	}
	want := fingerprint(t, ctx, conn)

	if err := m.Steps(-1); err != nil {
		t.Fatalf("down 1: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up after down 1: %v", err)
	}
	if got := fingerprint(t, ctx, conn); got != want {
		t.Fatal("schema differs after down 1 + up")
	}

	for i := latestVersion; i >= 1; i-- {
		if err := m.Steps(-1); err != nil {
			t.Fatalf("step down from %d: %v", i, err)
		}
	}
	if n := countTables(t, ctx, conn); n != 0 {
		t.Fatalf("%d tables left after all down migrations", n)
	}
	for i := 1; i <= latestVersion; i++ {
		if err := m.Steps(1); err != nil {
			t.Fatalf("step up to %d: %v", i, err)
		}
	}
	if got := fingerprint(t, ctx, conn); got != want {
		t.Fatal("schema differs after full down + step-wise up")
	}
}

func countTables(t *testing.T, ctx context.Context, conn *pgx.Conn) int {
	t.Helper()
	var n int
	err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name <> 'schema_migrations'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func fingerprint(t *testing.T, ctx context.Context, conn *pgx.Conn) string {
	t.Helper()
	queries := []string{
		`SELECT table_name || '.' || column_name || ':' || data_type || ':' || is_nullable || ':' || coalesce(column_default, '')
		   FROM information_schema.columns
		  WHERE table_schema = 'public' AND table_name <> 'schema_migrations' ORDER BY 1`,
		`SELECT conrelid::regclass::text || ':' || conname || ':' || pg_get_constraintdef(oid)
		   FROM pg_constraint
		  WHERE connamespace = 'public'::regnamespace AND conrelid::regclass::text <> 'schema_migrations' ORDER BY 1`,
		`SELECT indexdef FROM pg_indexes WHERE schemaname = 'public' AND tablename <> 'schema_migrations' ORDER BY 1`,
		`SELECT tgname || ':' || pg_get_triggerdef(oid) FROM pg_trigger WHERE NOT tgisinternal ORDER BY 1`,
	}
	var sb strings.Builder
	for _, q := range queries {
		rows, err := conn.Query(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			sb.WriteString(s)
			sb.WriteByte('\n')
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return sb.String()
}
