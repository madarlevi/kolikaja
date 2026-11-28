// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Command migrate applies the embedded database migrations. It is run by the "migrate"
// job of docker-compose.yml before the API starts.
//
//	migrate up        apply all pending migrations (default)
//	migrate version   print the current schema version
//
// Reverting migrations is intentionally not offered here; restore a backup instead
// (docs/ARCHITECTURE.md §15).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/config"
	"github.com/madarlevi/kolikaja/apps/api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd, logger); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(cmd string, logger *slog.Logger) error {
	if cmd != "up" && cmd != "version" {
		return fmt.Errorf("unknown command %q (use up or version)", cmd)
	}
	dsn, err := config.LoadDatabaseURL(os.Getenv, func(p string) ([]byte, error) { return os.ReadFile(filepath.Clean(p)) })
	if err != nil {
		return err
	}
	if err := waitForDatabase(dsn, logger); err != nil {
		return err
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5(dsn))
	if err != nil {
		return errors.New("cannot open the migration target (check the database settings)")
	}
	defer func() { _, _ = m.Close() }()

	if cmd == "up" {
		start := time.Now()
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		logger.Info("migrations applied", "duration_ms", time.Since(start).Milliseconds())
	}
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		logger.Info("schema version", "version", 0)
		return nil
	}
	if err != nil {
		return err
	}
	logger.Info("schema version", "version", v, "dirty", dirty)
	if dirty {
		return errors.New("the schema is dirty; restore a backup or fix the failed migration manually")
	}
	return nil
}

func toPgx5(dsn string) string {
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, scheme) {
			return "pgx5://" + strings.TrimPrefix(dsn, scheme)
		}
	}
	return dsn
}

func waitForDatabase(dsn string, logger *slog.Logger) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			err = conn.Ping(ctx)
			_ = conn.Close(ctx)
		}
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("database unavailable")
		}
		logger.Warn("database not ready, retrying")
		time.Sleep(2 * time.Second)
	}
}
