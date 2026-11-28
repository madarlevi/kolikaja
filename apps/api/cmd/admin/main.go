// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Command admin holds operator tasks:
//
//	admin promote   --email <address>        make a registered user a global administrator
//	admin demo-data --owner-email <address>  create the public demo dormitory "demo-kollegium"
//	                                         with published lunch menus for this week and next
//	admin init-secrets --dir <directory>     generate the instance secrets once (Compose
//	                                         "secrets-init" job; existing files are kept)
//
// In the Compose stack: docker compose exec api /app/admin <command> ...
// Demo data is never created automatically (docs/DECISIONS.md T-47).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata" // the demo dormitory's time zone works in minimal images

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/madarlevi/kolikaja/apps/api/internal/audit"
	"github.com/madarlevi/kolikaja/apps/api/internal/config"
)

const usage = `usage:
  admin promote      --email <address>
  admin demo-data    --owner-email <address>
  admin init-secrets --dir <directory>
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var email string
	switch cmd {
	case "init-secrets":
		dir := fs.String("dir", "", "directory of the secrets volume")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if *dir == "" {
			return errors.New("--dir is required")
		}
		return initSecrets(*dir, os.Stdout)
	case "promote":
		fs.StringVar(&email, "email", "", "e-mail address of a registered user")
	case "demo-data":
		fs.StringVar(&email, "owner-email", "", "e-mail address of the registered user who becomes the demo dormitory's administrator")
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("the e-mail address is required")
	}
	dsn, err := config.LoadDatabaseURL(os.Getenv, func(p string) ([]byte, error) { return os.ReadFile(filepath.Clean(p)) })
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return errors.New("cannot connect to the database (check the database settings)")
	}
	defer func() { _ = conn.Close(context.Background()) }()

	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		userID, err := userByEmail(ctx, tx, email)
		if err != nil {
			return err
		}
		if cmd == "promote" {
			return promote(ctx, tx, userID, email)
		}
		return demoData(ctx, tx, userID, time.Now())
	})
}

func userByEmail(ctx context.Context, tx pgx.Tx, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1) AND deleted_at IS NULL`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, errors.New("no registered user has this e-mail address; register first")
	}
	return id, err
}

func promote(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string) error {
	tag, err := tx.Exec(ctx, `UPDATE users SET is_global_admin = true, updated_at = now() WHERE id = $1 AND NOT is_global_admin`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		fmt.Printf("%s is already a global administrator\n", email)
		return nil
	}
	id := userID
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorType: audit.ActorSystem, Action: "admin.global_admin_granted", TargetType: "user", TargetID: &id,
		Metadata: map[string]any{"via": "admin_cli"},
	}); err != nil {
		return err
	}
	fmt.Printf("%s is now a global administrator\n", email)
	return nil
}

const demoSlug = "demo-kollegium"

// demoWeek is the demo lunch menu: soup, main course, dessert for each weekday.
var demoWeek = [7][3]string{
	{"Gulyásleves", "Rántott csirkemell rizzsel", "Túrós palacsinta"},
	{"Paradicsomleves", "Bolognai spagetti", "Almás pite"},
	{"Zöldborsóleves", "Paprikás csirke galuskával", "Gyümölcsjoghurt"},
	{"Húsleves cérnametélttel", "Sült hal petrezselymes burgonyával", "Kakaós csiga"},
	{"Babgulyás", "Lencsefőzelék fasírttal", "Somlói galuska"},
	{"Gombakrémleves", "Rakott krumpli", "Mákos guba"},
	{"Újházi tyúkhúsleves", "Sertéspörkölt tarhonyával", "Gyümölcssaláta"},
}

func demoData(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID, now time.Time) error {
	loc, err := time.LoadLocation("Europe/Budapest")
	if err != nil {
		return err
	}
	var dormID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM dormitories WHERE slug = $1 AND deleted_at IS NULL`, demoSlug).Scan(&dormID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if dormID, err = createDemoDormitory(ctx, tx, ownerID); err != nil {
			return err
		}
		fmt.Println("created the demo dormitory", demoSlug)
	case err != nil:
		return err
	default:
		fmt.Println("the demo dormitory already exists; adding missing menus")
	}
	if err := ensureAdmin(ctx, tx, dormID, ownerID); err != nil {
		return err
	}
	cats, err := ensureCategories(ctx, tx, dormID)
	if err != nil {
		return err
	}

	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	created := 0
	for i := 0; i < 14; i++ {
		day := monday.AddDate(0, 0, i)
		ok, err := createDemoMenu(ctx, tx, dormID, ownerID, day, demoWeek[i%7], cats)
		if err != nil {
			return err
		}
		if ok {
			created++
		}
	}
	fmt.Printf("added %d lunch menus (%s to %s)\n", created, monday.Format("2006-01-02"), monday.AddDate(0, 0, 13).Format("2006-01-02"))
	did := dormID
	return audit.Write(ctx, tx, audit.Entry{
		ActorType: audit.ActorSystem, DormitoryID: &did, Action: "admin.demo_data_created",
		TargetType: "dormitory", TargetID: &did, Metadata: map[string]any{"menus": created},
	})
}

func createDemoDormitory(ctx context.Context, tx pgx.Tx, ownerID uuid.UUID) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return id, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO dormitories (id, slug, name, description, city, visibility, join_policy, timezone, created_by)
		VALUES ($1, $2, 'Demó Kollégium', 'Bemutató kollégium teszteléshez. A menü kitalált, allergénadatok nincsenek megadva.', 'Budapest',
		        'public', 'open', 'Europe/Budapest', $3)`, id, demoSlug, ownerID)
	return id, err
}

func ensureAdmin(ctx context.Context, tx pgx.Tx, dormID, ownerID uuid.UUID) error {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM dormitory_members WHERE dormitory_id = $1 AND user_id = $2 AND left_at IS NULL`,
		dormID, ownerID).Scan(&role)
	if err == nil {
		if role != "admin" {
			_, err = tx.Exec(ctx, `UPDATE dormitory_members SET role = 'admin', updated_at = now()
				WHERE dormitory_id = $1 AND user_id = $2 AND left_at IS NULL`, dormID, ownerID)
		}
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO dormitory_members (id, dormitory_id, user_id, role, joined_via)
		VALUES ($1, $2, $3, 'admin', 'creator')`, id, dormID, ownerID)
	return err
}

func ensureCategories(ctx context.Context, tx pgx.Tx, dormID uuid.UUID) ([3]uuid.UUID, error) {
	var out [3]uuid.UUID
	for i, name := range []string{"Leves", "Főétel", "Desszert"} {
		err := tx.QueryRow(ctx, `SELECT id FROM meal_categories WHERE dormitory_id = $1 AND lower(name) = lower($2) AND deleted_at IS NULL`,
			dormID, name).Scan(&out[i])
		if err == nil {
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if out[i], err = uuid.NewV7(); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO meal_categories (id, dormitory_id, name, position) VALUES ($1, $2, $3, $4)`,
			out[i], dormID, name, i); err != nil {
			return out, err
		}
	}
	return out, nil
}

func createDemoMenu(ctx context.Context, tx pgx.Tx, dormID, ownerID uuid.UUID, day time.Time, names [3]string, cats [3]uuid.UUID) (bool, error) {
	menuID, err := uuid.NewV7()
	if err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO menus (id, dormitory_id, menu_date, meal_period, status, published_at, created_by, updated_by)
		VALUES ($1, $2, $3, 'lunch', 'published', now(), $4, $4)
		ON CONFLICT (dormitory_id, menu_date, meal_period) WHERE deleted_at IS NULL DO NOTHING`,
		menuID, dormID, day, ownerID)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	for i, name := range names {
		mealID, err := uuid.NewV7()
		if err != nil {
			return false, err
		}
		// Demo meals carry no allergen data: invented allergen values could mislead testers.
		if _, err := tx.Exec(ctx, `
			INSERT INTO meals (id, dormitory_id, menu_id, category_id, name, position)
			VALUES ($1, $2, $3, $4, $5, $6)`, mealID, dormID, menuID, cats[i], name, i); err != nil {
			return false, err
		}
	}
	return true, nil
}
