<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 madarlevi -->

# KoliKaja

KoliKaja is a self-hostable menu and community rating platform for dormitories
(kollégiumok). Residents see today's and this week's menu of their dormitory and rate each
meal with one tap on a five-step scale:

| | |
|---|---|
| 1 | Le se menj |
| 2 | Ha nem vagy éhenhalás szélén, hagyd |
| 3 | Ehető, ha nincs más |
| 4 | Amúgy eskü nem olyan szar |
| 5 | Már határozottan ehetőnek nevezhető |

One server can host many dormitories. Menus, ratings and other content of a dormitory are
visible only to its members, and the server checks membership on every request; public
dormitories appear in the directory with their name and description.

This repository holds the Go API, the database migrations, the Docker Compose setup, the
OpenAPI contract and the documentation. The apps live in separate repositories:

- **Android**: [madarlevi/kolikaja-android](https://github.com/madarlevi/kolikaja-android)
- **iOS**: [madarlevi/kolikaja-ios](https://github.com/madarlevi/kolikaja-ios) (no app yet)

> **Status: early preview (0.x).** It is not ready for production use. What works today:
> accounts (register, log in, sessions with refresh-token rotation, log out), the dormitory
> directory, joining public dormitories, daily and weekly menus, menu entry for moderators,
> meal details, ratings, recommendations and meal statistics. Comments, photos, invitations,
> moderation, announcements, the web app and the iOS app come in later versions. Other
> operations in the [API contract](docs/api/openapi.yaml) answer `404` until they are
> implemented.

## Quick start (Docker)

You need Docker with the Compose plugin (Docker Desktop on Windows and macOS, or Docker
Engine with `docker compose` on Linux).

```sh
git clone https://github.com/madarlevi/kolikaja.git
cd kolikaja
docker compose up -d --build
```

You don't need a `.env` file and there are no default passwords. On the first start the
`secrets-init` job generates a random database password and a random JWT signing key into the
`db-secrets` and `app-secrets` Docker volumes. Every installation gets its own values, they are
never printed, and later starts reuse them.

The API listens on `http://127.0.0.1:8080`, so only this computer (and the Android emulator)
can reach it. To test with a phone on the same Wi-Fi, start it with
`HTTP_BIND=0.0.0.0 docker compose up -d` (see [Testing with the Android app](#testing-with-the-android-app)).

Check that the API is running:

```sh
curl http://localhost:8080/readyz      # {"status":"ok","checks":{"database":"ok"}}
curl http://localhost:8080/api/v1/meta # instance information
```

The stack has four services: `secrets-init` (one-shot), `postgres` (PostgreSQL 17, not
reachable from outside), `migrate` (one-shot, applies the database migrations) and `api`
(port 8080).

### First user, administrator and demo data

1. Register an account in the Android app (see below) or with curl:

   ```sh
   curl -X POST http://localhost:8080/api/v1/auth/register \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com","password":"choose a long password","display_name":"Your Name"}'
   ```

   Passwords need at least 12 characters.

2. Optionally make that account a global administrator (it can then create dormitories):

   ```sh
   docker compose exec api /app/admin promote --email you@example.com
   ```

3. For testing, create the public demo dormitory **Demó Kollégium** with lunch menus for this
   week and next week. The given account becomes its administrator:

   ```sh
   docker compose exec api /app/admin demo-data --owner-email you@example.com
   ```

   Other accounts can find the demo dormitory in the app's dormitory search and join it. The
   demo menus are fictional and carry no allergen data. The demo data is never created
   automatically, and running the command again only adds missing menus.

## Testing with the Android app

1. Download the debug APK: open the latest successful run of the
   [Android CI workflow](https://github.com/madarlevi/kolikaja-android/actions/workflows/ci.yml)
   and download the `kolikaja-debug-apk` artifact. You need to be signed in to GitHub. The
   artifact is a ZIP file with the APK inside.
2. Install it on the phone. Copy the APK to the device and open it, and allow installing from
   this source when Android asks. Or use `adb install app-debug.apk`.
3. On the first screen enter the server address:
   - Android emulator on the same computer: `http://10.0.2.2:8080`
   - Phone on the same Wi-Fi: start the stack with `HTTP_BIND=0.0.0.0 docker compose up -d`
     (or put `HTTP_BIND=0.0.0.0` into `.env`), then use `http://<your computer's LAN IP>:8080`
     (for example `http://192.168.1.20:8080`). Allow incoming connections on port 8080 in the
     computer's firewall. Do this only on a network you trust: the test setup uses plain HTTP.
4. Register or log in, join a dormitory (for example the demo dormitory) and rate today's
   meals.

The debug build accepts plain `http://` addresses so it can reach a test server on your
network. Release builds will require `https://`.

## Configuration

Every setting is optional. To change one, copy [`deploy/env.example`](deploy/env.example) to
`.env` in the repository root, edit it and run `docker compose up -d` again. The most useful
settings:

| Variable | Default | Meaning |
|---|---|---|
| `HTTP_BIND` | `127.0.0.1` | Host address the API port is published on; `0.0.0.0` for the local network |
| `HTTP_PORT` | `8080` | Host port of the API |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Public address of the instance (JWT issuer and audience); `https://` enables HSTS |
| `INSTANCE_NAME` | `KoliKaja` | Name shown in the apps |
| `DORMITORY_CREATION` | `global_admins` | `all_users` lets every user create dormitories |
| `TRUSTED_PROXIES` | empty | CIDR ranges of reverse proxies whose `X-Forwarded-For` header is trusted |
| `SOURCE_CODE_URL` | this repository | Where users can get the source code. Change it if you run a modified version (AGPL-3.0) |

Never put secrets into `.env`. The generated secrets live in the `db-secrets` and `app-secrets`
volumes.

## Running a server for real users

This preview is meant for testing. Before you let real users in:

- **Use HTTPS.** Put a TLS reverse proxy (for example Caddy or nginx) in front of the API, set
  `PUBLIC_BASE_URL=https://kolikaja.example.org` (your address) and `TRUSTED_PROXIES` to the
  proxy's address range, and keep `HTTP_BIND=127.0.0.1`. Ports published by Docker bypass host
  firewalls such as ufw, so never publish port 8080 on a public interface.
- **Back up** the database regularly:

  ```sh
  docker compose exec -T postgres pg_dump -U kolikaja -Fc kolikaja > kolikaja-$(date +%F).dump
  ```

  Keep the `db-secrets` volume as well (`docker volume ls`): it holds the database password
  that matches the restored data.
- **Updates:** `git pull` and `docker compose up -d --build`. Migrations run automatically.
- Logs: `docker compose logs -f api`. The API is designed not to log passwords, tokens or other
  secrets.
- Stop: `docker compose down`. `docker compose down -v` **deletes all data and the generated
  secrets**.

Before opening an instance to other people, also review the data protection duties that apply
to you as its operator, and fill in the legal links (`PRIVACY_POLICY_URL`, `TERMS_URL`,
`OPERATOR_CONTACT`).

Allergen information is entered by dormitory staff and may be wrong; users should always
confirm it with the food provider.

## Development

| Path | Content |
|---|---|
| `apps/api/` | Go API (chi, pgx, PostgreSQL 17, golang-migrate); see `apps/api/CLAUDE.md` |
| `apps/api/migrations/` | SQL migrations (embedded into the binaries) |
| `docs/api/openapi.yaml` | OpenAPI 3.1 contract (the source of truth for the apps) |
| `docs/api/authz-matrix.md` | Generated authorization matrix, one row per operation |
| `docs/` | Architecture, decisions, progress, security plan, design |
| `deploy/` | Dockerfile, optional settings, smoke test |

Common commands (from `apps/api`):

```sh
go test ./...                      # unit tests; database tests are skipped
KOLIKAJA_TEST_DATABASE_URL=postgres://user:pass@localhost:5432/kolikaja_test?sslmode=disable \
  go test -p 1 ./tests/...         # database, authorization and API flow tests
```

The database tests reset the schema, so they only run against a database whose name ends with
`_test`, `_ci` or `_it`. CI (GitHub Actions) runs the unit and database tests, golangci-lint,
gosec, govulncheck, Semgrep, CodeQL, gitleaks, the contract lint, the authorization matrix
check, the Docker Compose smoke test, an injection payload harness and an OWASP ZAP baseline
scan.

## Security

Please report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

KoliKaja is free software under the GNU Affero General Public License v3.0 only
(`AGPL-3.0-only`); see [LICENSE](LICENSE). If you run a modified version and let users interact
with it over a network, section 13 of the AGPL requires you to offer them its source code;
`SOURCE_CODE_URL` tells the apps where to find it. If you distribute copies (for example
container images or binaries), sections 4–6 apply. Third-party components and their licences
are listed in [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).

Copyright (C) 2026 madarlevi.

## Trademarks

The AGPL licenses the code, not the name "KoliKaja". The rules for the name will be published
in a separate trademark policy; they do not restrict any right the AGPL grants. Forks are
welcome under a different name.
