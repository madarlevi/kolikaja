// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Fixture IDs (UUIDv7 layout, fixed for readability).
const (
	userA      = "00000000-0000-7000-8000-000000000001"
	userB      = "00000000-0000-7000-8000-000000000002"
	dormA      = "00000000-0000-7000-8000-00000000000a"
	dormB      = "00000000-0000-7000-8000-00000000000b"
	menuA      = "00000000-0000-7000-8000-0000000000a1"
	mealA      = "00000000-0000-7000-8000-0000000000a2"
	commentA   = "00000000-0000-7000-8000-0000000000a3"
	menuB      = "00000000-0000-7000-8000-0000000000b1"
	mealB      = "00000000-0000-7000-8000-0000000000b2"
	reportA    = "00000000-0000-7000-8000-0000000000f1"
	sessionA   = "00000000-0000-7000-8000-0000000000e1"
	sessionB   = "00000000-0000-7000-8000-0000000000e2"
	apiKeyInst = "00000000-0000-7000-8000-0000000000c1"
)

// Not a real password hash: a syntactically valid Argon2id PHC string for fixtures.
const fakeArgon2 = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA"

var fixture = []string{
	`INSERT INTO users (id, email, password_hash, display_name) VALUES
	   ('` + userA + `', 'Anna@example.test', '` + fakeArgon2 + `', 'Anna'),
	   ('` + userB + `', 'bela@example.test', '` + fakeArgon2 + `', 'Béla')`,
	`INSERT INTO dormitories (id, slug, name, visibility, join_policy, created_by) VALUES
	   ('` + dormA + `', 'kollegium-a', 'Kollégium A', 'public', 'open', '` + userA + `'),
	   ('` + dormB + `', 'kollegium-b', 'Kollégium B', 'private', 'invite_only', '` + userB + `')`,
	`INSERT INTO dormitory_members (id, dormitory_id, user_id, role, joined_via) VALUES
	   ('00000000-0000-7000-8000-0000000000d1', '` + dormA + `', '` + userA + `', 'admin', 'creator'),
	   ('00000000-0000-7000-8000-0000000000d2', '` + dormB + `', '` + userB + `', 'admin', 'creator')`,
	`INSERT INTO menus (id, dormitory_id, menu_date, status, published_at) VALUES
	   ('` + menuA + `', '` + dormA + `', DATE '2026-11-26', 'published', now()),
	   ('` + menuB + `', '` + dormB + `', DATE '2026-11-26', 'published', now())`,
	`INSERT INTO meals (id, dormitory_id, menu_id, name, allergens) VALUES
	   ('` + mealA + `', '` + dormA + `', '` + menuA + `', 'Rántott sajt', ARRAY['milk','gluten','eggs']),
	   ('` + mealB + `', '` + dormB + `', '` + menuB + `', 'Gulyásleves', ARRAY[]::text[])`,
	`INSERT INTO comments (id, dormitory_id, meal_id, user_id, body) VALUES
	   ('` + commentA + `', '` + dormA + `', '` + mealA + `', '` + userA + `', 'Finom volt.')`,
}

func rating(dorm, meal, user, value string) string {
	return `INSERT INTO ratings (id, dormitory_id, meal_id, user_id, value) VALUES (gen_random_uuid(), '` +
		dorm + `', '` + meal + `', '` + user + `', ` + value + `)`
}

func report(dorm, reporter, targetType, targetColumn, targetID, subject, reason string) string {
	return `INSERT INTO reports (id, dormitory_id, reporter_id, target_type, ` + targetColumn +
		`, subject_user_id, reason) VALUES (gen_random_uuid(), '` + dorm + `', '` + reporter + `', '` +
		targetType + `', '` + targetID + `', '` + subject + `', '` + reason + `')`
}

func userReport(id, dorm, reporter, target, subject string) string {
	return `INSERT INTO reports (id, dormitory_id, reporter_id, target_type, target_user_id, subject_user_id, reason) VALUES ('` +
		id + `', '` + dorm + `', '` + reporter + `', 'user', '` + target + `', '` + subject + `', 'spam')`
}

func sanction(user, dorm, reportID, createdBy string) string {
	return `INSERT INTO user_sanctions (id, user_id, dormitory_id, kind, reason, report_id, created_by) VALUES (gen_random_uuid(), '` +
		user + `', '` + dorm + `', 'suspension', 'x', '` + reportID + `', '` + createdBy + `')`
}

func refreshToken(session, hexHash string) string {
	return `INSERT INTO refresh_tokens (id, session_id, token_hash, expires_at) VALUES (gen_random_uuid(), '` +
		session + `', '\x` + hexHash + `'::bytea, now() + interval '30 days')`
}

func apiKey(id, scope string) string {
	return `INSERT INTO api_keys (id, name, key_prefix, key_hash, scopes, created_by, expires_at) VALUES ('` + id +
		`', 'x', 'abcdefghijk2', '\x` + strings.Repeat("ab", 32) + `'::bytea, ARRAY['` + scope + `'], '` + userA +
		`', now() + interval '30 days')`
}

// TestSchemaConstraints proves the database-level rules of docs/ARCHITECTURE.md §6:
// tenant backstops, uniqueness, checks and the append-only audit log. Each case runs in
// its own transaction that is rolled back.
func TestSchemaConstraints(t *testing.T) {
	dsn := databaseURL(t)
	conn := connect(t, dsn) // refuses non-disposable databases before anything is migrated
	m := newMigrate(t, dsn)
	if err := ignoreNoChange(m.Up()); err != nil {
		t.Fatalf("up: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name    string
		sql     []string
		wantErr string // SQLSTATE; empty = must succeed
	}{
		{"fixture inserts cleanly (UTF-8 names)", nil, ""},
		{"rating accepted", []string{rating(dormA, mealA, userA, "5")}, ""},
		{"duplicate active rating rejected", []string{rating(dormA, mealA, userA, "5"), rating(dormA, mealA, userA, "3")}, "23505"},
		{"re-rating after soft delete allowed", []string{
			`INSERT INTO ratings (id, dormitory_id, meal_id, user_id, value, deleted_at) VALUES (gen_random_uuid(), '` + dormA + `', '` + mealA + `', '` + userA + `', 2, now())`,
			rating(dormA, mealA, userA, "4")}, ""},
		{"upsert on the partial unique index", []string{
			rating(dormA, mealA, userA, "5"),
			rating(dormA, mealA, userA, "1") + ` ON CONFLICT (meal_id, user_id) WHERE deleted_at IS NULL DO UPDATE SET value = EXCLUDED.value, updated_at = now()`}, ""},
		{"rating value 6 rejected", []string{rating(dormA, mealA, userA, "6")}, "23514"},
		{"cross-tenant rating rejected", []string{rating(dormB, mealA, userB, "1")}, "23503"},
		{"cross-tenant comment rejected", []string{`INSERT INTO comments (id, dormitory_id, meal_id, user_id, body) VALUES (gen_random_uuid(), '` + dormB + `', '` + mealA + `', '` + userB + `', 'x')`}, "23503"},
		{"cross-tenant meal rejected", []string{`INSERT INTO meals (id, dormitory_id, menu_id, name) VALUES (gen_random_uuid(), '` + dormA + `', '` + menuB + `', 'x')`}, "23503"},
		{"cross-tenant report rejected", []string{report(dormB, userB, "comment", "target_comment_id", commentA, userA, "spam")}, "23503"},
		{"report in same dormitory accepted", []string{report(dormA, userB, "comment", "target_comment_id", commentA, userA, "spam")}, ""},
		{"duplicate open report rejected", []string{
			report(dormA, userB, "comment", "target_comment_id", commentA, userA, "spam"),
			report(dormA, userB, "comment", "target_comment_id", commentA, userA, "other")}, "23505"},
		{"report target and type mismatch rejected", []string{report(dormA, userB, "user", "target_comment_id", commentA, userA, "spam")}, "23514"},
		{"self report rejected", []string{userReport(reportA, dormA, userA, userA, userA)}, "23514"},
		{"report on own comment rejected", []string{report(dormA, userA, "comment", "target_comment_id", commentA, userA, "spam")}, "23514"},
		{"user report with a different subject rejected", []string{userReport(reportA, dormA, userA, userB, userA)}, "23514"},
		{"membership via global-admin assignment rejected", []string{`INSERT INTO dormitory_members (id, dormitory_id, user_id, joined_via) VALUES (gen_random_uuid(), '` + dormA + `', '` + userB + `', 'admin_assigned')`}, "23514"},
		{"second active membership rejected", []string{`INSERT INTO dormitory_members (id, dormitory_id, user_id, joined_via) VALUES (gen_random_uuid(), '` + dormA + `', '` + userA + `', 'open_join')`}, "23505"},
		{"unknown role rejected", []string{`INSERT INTO dormitory_members (id, dormitory_id, user_id, role, joined_via) VALUES (gen_random_uuid(), '` + dormA + `', '` + userB + `', 'owner', 'open_join')`}, "23514"},
		{"private dormitory with open join rejected", []string{`INSERT INTO dormitories (id, slug, name, visibility, join_policy, created_by) VALUES (gen_random_uuid(), 'zart', 'Zárt', 'private', 'open', '` + userA + `')`}, "23514"},
		{"case-insensitive duplicate e-mail rejected", []string{`INSERT INTO users (id, email, password_hash, display_name) VALUES (gen_random_uuid(), 'anna@EXAMPLE.test', '` + fakeArgon2 + `', 'Anna2')`}, "23505"},
		{"plaintext password rejected", []string{`INSERT INTO users (id, email, password_hash, display_name) VALUES (gen_random_uuid(), 'c@example.test', 'not-a-hash', 'Cili')`}, "23514"},
		{"deleted account must be scrubbed", []string{`UPDATE users SET deleted_at = now() WHERE id = '` + userB + `'`}, "23514"},
		{"scrubbed deleted account accepted", []string{`UPDATE users SET deleted_at = now(), email = NULL, password_hash = NULL, display_name = NULL WHERE id = '` + userB + `'`}, ""},
		{"unknown allergen rejected", []string{`INSERT INTO meals (id, dormitory_id, menu_id, name, allergens) VALUES (gen_random_uuid(), '` + dormA + `', '` + menuA + `', 'x', ARRAY['bacon'])`}, "23514"},
		{"second menu for the same date and period rejected", []string{`INSERT INTO menus (id, dormitory_id, menu_date) VALUES (gen_random_uuid(), '` + dormA + `', DATE '2026-11-26')`}, "23505"},
		{"published menu without published_at rejected", []string{`INSERT INTO menus (id, dormitory_id, menu_date, meal_period, status) VALUES (gen_random_uuid(), '` + dormA + `', DATE '2026-11-27', 'dinner', 'published')`}, "23514"},
		{"second unused refresh token per session rejected", []string{
			`INSERT INTO user_sessions (id, user_id, client_type, expires_at) VALUES ('` + sessionA + `', '` + userA + `', 'native', now() + interval '90 days')`,
			refreshToken(sessionA, strings.Repeat("01", 32)),
			refreshToken(sessionA, strings.Repeat("02", 32))}, "23505"},
		{"short token hash rejected", []string{
			`INSERT INTO user_sessions (id, user_id, client_type, expires_at) VALUES ('` + sessionB + `', '` + userA + `', 'web', now() + interval '90 days')`,
			refreshToken(sessionB, "01")}, "23514"},
		{"instance API key with write scope rejected", []string{apiKey(apiKeyInst, "menus:write")}, "23514"},
		{"instance API key with directory scope accepted", []string{apiKey(apiKeyInst, "dormitories:read")}, ""},
		{"audit log insert accepted", []string{`INSERT INTO audit_logs (id, actor_type, actor_user_id, dormitory_id, action) VALUES (gen_random_uuid(), 'user', '` + userA + `', '` + dormA + `', 'comment.removed')`}, ""},
		{"audit log update rejected", []string{
			`INSERT INTO audit_logs (id, actor_type, actor_user_id, action) VALUES (gen_random_uuid(), 'user', '` + userA + `', 'auth.login_succeeded')`,
			`UPDATE audit_logs SET action = 'auth.tampered'`}, "42501"},
		{"audit log delete of old rows allowed", []string{
			`INSERT INTO audit_logs (id, actor_type, action, occurred_at) VALUES (gen_random_uuid(), 'system', 'retention.purged', now() - interval '31 days')`,
			`DELETE FROM audit_logs WHERE action = 'retention.purged'`}, ""},
		{"audit log delete of recent rows rejected", []string{
			`INSERT INTO audit_logs (id, actor_type, action) VALUES (gen_random_uuid(), 'system', 'retention.purged')`,
			`DELETE FROM audit_logs WHERE action = 'retention.purged'`}, "42501"},
		{"audit log truncate rejected", []string{`TRUNCATE audit_logs`}, "42501"},
		{"audit log malformed action rejected", []string{`INSERT INTO audit_logs (id, actor_type, action) VALUES (gen_random_uuid(), 'system', 'DROP TABLE')`}, "23514"},
		{"sanction citing a report of another dormitory rejected", []string{
			userReport(reportA, dormA, userA, userB, userB), sanction(userB, dormB, reportA, userA)}, "23503"},
		{"sanction citing a report about another user rejected", []string{
			userReport(reportA, dormA, userA, userB, userB), sanction(userA, dormA, reportA, userB)}, "23503"},
		{"sanction citing a report about the same user accepted", []string{
			userReport(reportA, dormA, userA, userB, userB), sanction(userB, dormA, reportA, userA)}, ""},
		{"password reset notification kind accepted", []string{`INSERT INTO notifications (id, user_id, kind) VALUES (gen_random_uuid(), '` + userA + `', 'account.password_reset_issued')`}, ""},
		{"admin oversight notification kind accepted", []string{`INSERT INTO notifications (id, user_id, kind) VALUES (gen_random_uuid(), '` + userA + `', 'admin.oversight')`}, ""},
		{"unknown notification kind rejected", []string{`INSERT INTO notifications (id, user_id, kind) VALUES (gen_random_uuid(), '` + userA + `', 'marketing.newsletter')`}, "23514"},
		{"notification payload must be an object", []string{`INSERT INTO notifications (id, user_id, kind, payload) VALUES (gen_random_uuid(), '` + userA + `', 'announcement.published', '[1,2]'::jsonb)`}, "23514"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			var got error
			for _, stmt := range append(append([]string{}, fixture...), tt.sql...) {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					got = err
					break
				}
			}
			state := ""
			var pgErr *pgconn.PgError
			if errors.As(got, &pgErr) {
				state = pgErr.Code
			} else if got != nil {
				t.Fatalf("non-database error: %v", got)
			}
			if state != tt.wantErr {
				t.Fatalf("SQLSTATE = %q, want %q (%v)", state, tt.wantErr, got)
			}
		})
	}
}
