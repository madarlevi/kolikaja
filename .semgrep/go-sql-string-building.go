// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Test cases for .semgrep/go-sql-string-building.yml. Run: semgrep --test .semgrep
// This file is not part of any Go module and is never compiled.
package semgreptests

import (
	"context"
	"fmt"
	"strings"
)

type pool struct{}

func (pool) Query(ctx context.Context, sql string, args ...any) (any, error) { return nil, nil }
func (pool) QueryRow(ctx context.Context, sql string, args ...any) any        { return nil }
func (pool) Exec(ctx context.Context, sql string, args ...any) (any, error)  { return nil, nil }

type batch struct{}

func (batch) Queue(sql string, args ...any) {}

func bad(ctx context.Context, db pool, b batch, name, column string) {
	// ruleid: kolikaja-go-sql-string-building
	db.Query(ctx, fmt.Sprintf("SELECT id FROM users WHERE display_name = '%s'", name))

	// ruleid: kolikaja-go-sql-string-building
	db.Exec(ctx, "DELETE FROM users WHERE display_name = '"+name+"'")

	// ruleid: kolikaja-go-sql-string-building
	q := "SELECT id FROM users ORDER BY " + column
	db.QueryRow(ctx, q)

	// ruleid: kolikaja-go-sql-string-building
	b.Queue(fmt.Sprintf("UPDATE users SET locale = '%s'", name))

	var sb strings.Builder
	sb.WriteString("SELECT id FROM users WHERE email = '")
	sb.WriteString(name)
	// ruleid: kolikaja-go-sql-string-building
	db.Query(ctx, sb.String())
}

func good(ctx context.Context, db pool, b batch, name string) {
	// ok: kolikaja-go-sql-string-building
	db.Query(ctx, "SELECT id FROM users WHERE display_name = $1", name)

	const byEmail = "SELECT id FROM users WHERE lower(email) = lower($1)"
	// ok: kolikaja-go-sql-string-building
	db.QueryRow(ctx, byEmail, name)

	// ok: kolikaja-go-sql-string-building
	b.Queue("UPDATE users SET locale = $1 WHERE id = $2", "hu", name)
}
