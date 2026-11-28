// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 madarlevi

// Command authzmatrix generates or checks docs/api/authz-matrix.md.
// Use scripts/check-authz-matrix.sh instead of calling it directly.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/madarlevi/kolikaja/apps/api/internal/authzmatrix"
	"github.com/madarlevi/kolikaja/apps/api/internal/contract"
)

func main() {
	specPath := flag.String("spec", "", "path to docs/api/openapi.yaml")
	matrixPath := flag.String("matrix", "", "path to docs/api/authz-matrix.md")
	testsDir := flag.String("tests", "", "directory with TestAuthz_* tests")
	write := flag.Bool("write", false, "rewrite the matrix instead of checking it")
	requireTests := flag.Bool("require-tests", false, "fail while any dormitory-scoped operation has only a planned test")
	flag.Parse()
	if *specPath == "" || *matrixPath == "" || *testsDir == "" {
		fmt.Fprintln(os.Stderr, "usage: authzmatrix -spec <openapi.yaml> -matrix <authz-matrix.md> -tests <dir> [-write] [-require-tests]")
		os.Exit(2)
	}
	if err := run(*specPath, *matrixPath, *testsDir, *write, *requireTests); err != nil {
		fmt.Fprintln(os.Stderr, "authzmatrix:", err)
		os.Exit(1)
	}
}

func run(specPath, matrixPath, testsDir string, write, requireTests bool) error {
	doc, err := contract.Load(specPath)
	if err != nil {
		return err
	}
	if unknown := authzmatrix.UnknownExtraChecks(doc); len(unknown) > 0 {
		return fmt.Errorf("object-level checks defined for unknown operations: %v", unknown)
	}
	existing, err := authzmatrix.FindTests(testsDir)
	if err != nil {
		return err
	}
	rows, err := authzmatrix.BuildRows(doc, existing)
	if err != nil {
		return err
	}
	rendered := authzmatrix.Render(rows)
	if write {
		if err := os.WriteFile(filepath.Clean(matrixPath), []byte(rendered), 0o600); err != nil {
			return err
		}
		fmt.Printf("wrote %s: %d operations, %d rows\n", matrixPath, len(doc.Operations), len(authzmatrix.ParseOperationIDs(rendered)))
		return nil
	}
	committed, err := os.ReadFile(filepath.Clean(matrixPath))
	if err != nil {
		return fmt.Errorf("read matrix: %w", err)
	}
	problems := authzmatrix.Check(doc, rows, string(committed), requireTests)
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, "FAIL:", p)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d problem(s)", len(problems))
	}
	mode := ""
	if requireTests {
		mode = " (all scoped operations have implemented tests)"
	}
	fmt.Printf("ok: %d operations in the contract, %d rows in the matrix%s\n", len(doc.Operations), len(authzmatrix.ParseOperationIDs(string(committed))), mode)
	return nil
}
