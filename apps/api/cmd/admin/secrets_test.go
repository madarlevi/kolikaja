// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitSecrets(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := initSecrets(dir, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "generated") != 2 {
		t.Fatalf("output: %s", out.String())
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	first := map[string]string{}
	for _, s := range instanceSecrets {
		b, err := root.ReadFile(s.path())
		if err != nil {
			t.Fatal(err)
		}
		if len(b) < 32 || strings.ContainsAny(string(b), "\n /+=") {
			t.Errorf("%s: unexpected value format (%d bytes)", s.name, len(b))
		}
		if strings.Contains(out.String(), string(b)) {
			t.Errorf("%s: the value was printed", s.name)
		}
		first[s.path()] = string(b)
		if runtime.GOOS != "windows" {
			info, _ := root.Stat(s.path())
			if info.Mode().Perm() != s.mode {
				t.Errorf("%s: mode %v, want %v", s.name, info.Mode().Perm(), s.mode)
			}
		}
	}
	if first["db/postgres_password"] == first["app/jwt_secret"] {
		t.Error("secrets are not independent")
	}

	// A second run keeps the existing values.
	out.Reset()
	if err := initSecrets(dir, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "kept") != 2 {
		t.Fatalf("second run: %s", out.String())
	}
	for name, v := range first {
		b, _ := root.ReadFile(name)
		if string(b) != v {
			t.Errorf("%s changed on the second run", name)
		}
	}
	if err := initSecrets(filepath.Join(dir, "missing"), &out); err == nil {
		t.Error("missing directory accepted")
	}
}
