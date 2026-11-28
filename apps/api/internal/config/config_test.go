// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package config

import (
	"errors"
	"strings"
	"testing"
)

func env(m map[string]string) Getenv { return func(k string) string { return m[k] } }

func files(m map[string]string) ReadFile {
	return func(p string) ([]byte, error) {
		if v, ok := m[p]; ok {
			return []byte(v), nil
		}
		return nil, errors.New("missing")
	}
}

func base() map[string]string {
	return map[string]string{ //nolint:gosec // #nosec G101 -- fake test DSN, not a credential.
		"PUBLIC_BASE_URL": "http://localhost:8080",
		"DATABASE_URL":    "postgres://u:p@localhost:5432/k_test?sslmode=disable",
		"JWT_SECRET":      strings.Repeat("s", 32),
	}
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(base()), files(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.DormitoryCreation != "global_admins" || c.AccessTokenTTL.Minutes() != 10 || c.HTTPAddr != ":8080" {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadRejectsMissingAndWeakSecrets(t *testing.T) {
	e := base()
	delete(e, "JWT_SECRET")
	if _, err := Load(env(e), files(nil)); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("missing secret not reported: %v", err)
	}
	e["JWT_SECRET"] = "short"
	if _, err := Load(env(e), files(nil)); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("weak secret not reported: %v", err)
	}
}

func TestLoadReadsSecretFilesAndBuildsDSN(t *testing.T) {
	e := base()
	delete(e, "DATABASE_URL")
	delete(e, "JWT_SECRET")
	e["JWT_SECRET_FILE"] = "/run/jwt"
	e["DB_HOST"] = "postgres"
	e["DB_USER"] = "kolikaja"
	e["DB_NAME"] = "kolikaja"
	e["DB_PASSWORD_FILE"] = "/run/db"
	e["DB_SSLMODE"] = "disable"
	c, err := Load(env(e), files(map[string]string{"/run/jwt": strings.Repeat("j", 48) + "\n", "/run/db": "p@ss/word\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if string(c.JWTSecret) != strings.Repeat("j", 48) {
		t.Errorf("secret file not trimmed: %q", c.JWTSecret)
	}
	if c.DatabaseURL != "postgres://kolikaja:p%40ss%2Fword@postgres:5432/kolikaja?sslmode=disable" { //nolint:gosec // #nosec G101 -- expected value of a fake test DSN.
		t.Errorf("dsn = %s", c.DatabaseURL)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	e := base()
	e["DORMITORY_CREATION"] = "everyone"
	e["TRUSTED_PROXIES"] = "not-a-cidr"
	e["PUBLIC_BASE_URL"] = "ftp://x"
	_, err := Load(env(e), files(nil))
	for _, want := range []string{"DORMITORY_CREATION", "TRUSTED_PROXIES", "PUBLIC_BASE_URL"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s not reported: %v", want, err)
		}
	}
}

func TestLogValueHasNoSecrets(t *testing.T) {
	c, err := Load(env(base()), files(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s := c.LogValue().String(); strings.Contains(s, "sss") || strings.Contains(s, "postgres://") {
		t.Errorf("secret material in log value: %s", s)
	}
}
