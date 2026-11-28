// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Package migrations embeds the SQL migrations so that cmd/migrate, the readiness probe and
// the tests use the exact files of the build.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// FS holds the NNNN_name.up.sql / NNNN_name.down.sql files.
//
//go:embed *.sql
var FS embed.FS

// Latest returns the highest migration version in FS.
func Latest() (uint, error) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return 0, err
	}
	var latest uint64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return 0, fmt.Errorf("migrations: bad file name %q", name)
		}
		v, err := strconv.ParseUint(prefix, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("migrations: bad version in %q: %w", name, err)
		}
		latest = max(latest, v)
	}
	return uint(latest), nil
}
