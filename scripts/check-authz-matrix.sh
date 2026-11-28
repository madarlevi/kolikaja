#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (C) 2026 madarlevi
#
# Checks that docs/api/authz-matrix.md has exactly one row per operation of
# docs/api/openapi.yaml and is up to date. Fails on a missing row, an unknown row,
# a dormitory-scoped operation without a planned or real cross-dormitory test, or
# a stale file.
#
# Usage:
#   scripts/check-authz-matrix.sh           check (CI)
#   scripts/check-authz-matrix.sh --write   regenerate after changing the contract
# With AUTHZ_REQUIRE_TESTS=1 the check also fails while any dormitory-scoped operation has
# only a planned test (phase 2 security gate, docs/SECURITY-PLAN.md).
set -euo pipefail

mode=()
case "${1:-}" in
  "") ;;
  --write) mode=(-write) ;;
  *) echo "usage: $0 [--write]" >&2; exit 2 ;;
esac
if [ "${AUTHZ_REQUIRE_TESTS:-0}" = "1" ]; then
  mode+=(-require-tests)
fi

cd "$(dirname "$0")/../apps/api"
go run ./tools/authzmatrix "${mode[@]}" \
  -spec ../../docs/api/openapi.yaml \
  -matrix ../../docs/api/authz-matrix.md \
  -tests ./tests/authz
