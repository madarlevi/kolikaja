// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Owners inside the Compose stack: the postgres:17-alpine server runs as uid 70, the API,
// migrate and admin processes as the distroless "nonroot" user 65532.
const (
	postgresUID = 70
	appUID      = 65532
)

// secretFile is one generated secret. The database password (db/, its own volume) is
// readable by PostgreSQL (owner) and by the application (group); the JWT key (app/, a volume
// only the API mounts) only by the application.
type secretFile struct {
	dir, name string
	bytes     int
	uid, gid  int
	mode      fs.FileMode
}

func (s secretFile) path() string { return s.dir + "/" + s.name }

var instanceSecrets = []secretFile{
	{dir: "db", name: "postgres_password", bytes: 32, uid: postgresUID, gid: appUID, mode: 0o440},
	{dir: "app", name: "jwt_secret", bytes: 48, uid: appUID, gid: appUID, mode: 0o400},
}

// initSecrets creates every missing secret with fresh random bytes and leaves existing
// files untouched, so restarts keep the database password and sessions. Values are never
// printed. Each installation gets its own secrets; there are no default credentials.
// All file operations go through os.Root, so they cannot leave the directory.
func initSecrets(dir string, out io.Writer) error {
	root, err := os.OpenRoot(filepath.Clean(dir)) // #nosec G703 -- the operator chooses the secrets directory.
	if err != nil {
		return fmt.Errorf("%s is not a usable directory: %w", dir, err)
	}
	defer func() { _ = root.Close() }()
	for _, s := range instanceSecrets {
		if err := root.Mkdir(s.dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s: %w", s.dir, err)
		}
		created, err := writeSecret(root, s)
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		state := "kept"
		if created {
			state = "generated"
		}
		_, _ = fmt.Fprintln(out, s.name+": "+state)
	}
	return nil
}

func writeSecret(root *os.Root, s secretFile) (bool, error) {
	if _, err := root.Stat(s.path()); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	raw := make([]byte, s.bytes)
	if _, err := rand.Read(raw); err != nil {
		return false, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	tmp := s.path() + ".tmp"
	_ = root.Remove(tmp)
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(value); err != nil {
		_ = f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	// Permissions first: after the ownership change the file no longer belongs to us.
	if err := root.Chmod(tmp, s.mode); err != nil {
		return false, err
	}
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		if err := root.Chown(tmp, s.uid, s.gid); err != nil {
			return false, err
		}
	}
	return true, root.Rename(tmp, s.path())
}
