#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (C) 2026 madarlevi
#
# Smoke test of a running Compose stack (used by CI, also handy after an update):
# health probes, registration, the admin CLI, demo data, login, menus, a rating and a
# cross-check that anonymous access is refused.
#
#   deploy/scripts/smoke-test.sh [base-url]        (default http://localhost:8080)
#
# Creates a throw-away user and the demo dormitory, so run it only on test instances.
# When SMOKE_TOKEN_FILE is set, the access token of the test user is written there (CI
# passes it to the payload harness). Requires curl, jq and the docker compose CLI.
set -euo pipefail

base="${1:-http://localhost:8080}"
api="$base/api/v1"
email="smoke-$(date +%s)-${RANDOM}@example.test"
password="smoke-$(od -An -N12 -tx1 /dev/urandom | tr -d ' \n')"

fail() { echo "smoke test FAILED: $*" >&2; exit 1; }
json() { jq -n --arg e "$email" --arg p "$password" "$1"; }

echo "waiting for $base/readyz"
for _ in $(seq 1 90); do
  if curl -fsS "$base/readyz" >/dev/null 2>&1; then break; fi
  sleep 2
done
curl -fsS "$base/readyz" | jq -e '.status == "ok"' >/dev/null || fail "readyz"
curl -fsS "$base/healthz" | jq -e '.status == "ok"' >/dev/null || fail "healthz"
curl -fsS "$api/meta" | jq -e '.license == "AGPL-3.0-only" and .api_version == "v1"' >/dev/null || fail "meta"

headers="$(curl -fsS -D - -o /dev/null "$base/healthz")"
grep -qi '^x-content-type-options: nosniff' <<<"$headers" || fail "nosniff header"
grep -qi "^content-security-policy: default-src 'none'" <<<"$headers" || fail "CSP header"

code="$(curl -sS -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' \
  --data "$(json '{email: $e, password: $p, display_name: "Smoke Test"}')" "$api/auth/register")"
[ "$code" = 201 ] || fail "register returned $code"

docker compose exec -T api /app/admin demo-data --owner-email "$email"

token="$(curl -fsS -H 'Content-Type: application/json' \
  --data "$(json '{email: $e, password: $p, device_label: "smoke test"}')" "$api/auth/native/login" | jq -r '.access_token')"
if [ -z "$token" ] || [ "$token" = null ]; then fail "login"; fi
auth=(-H "Authorization: Bearer $token")

dorm="$(curl -fsS "${auth[@]}" "$api/me/dormitories" |
  jq -r '.data[] | select(.dormitory.slug == "demo-kollegium") | .dormitory.id')"
[ -n "$dorm" ] || fail "demo dormitory membership"

meal="$(curl -fsS "${auth[@]}" "$api/dormitories/$dorm/menus/today" | jq -r '.menus[0].meals[0].id')"
if [ -z "$meal" ] || [ "$meal" = null ]; then fail "today's menu"; fi
curl -fsS "${auth[@]}" "$api/dormitories/$dorm/menus/week" | jq -e '.days | length == 7' >/dev/null || fail "week"

code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT "${auth[@]}" -H 'Content-Type: application/json' \
  --data '{"value":4}' "$api/dormitories/$dorm/meals/$meal/my-rating")"
[ "$code" = 201 ] || fail "rating returned $code"
curl -fsS "${auth[@]}" "$api/dormitories/$dorm/meals/$meal" | jq -e '.my_rating == 4' >/dev/null || fail "my_rating"

code="$(curl -sS -o /dev/null -w '%{http_code}' "$api/dormitories/$dorm/menus/today")"
[ "$code" = 401 ] || fail "anonymous request returned $code"

if [ -n "${SMOKE_TOKEN_FILE:-}" ]; then
  umask 077
  printf '%s' "$token" >"$SMOKE_TOKEN_FILE"
fi
echo "smoke test passed"
