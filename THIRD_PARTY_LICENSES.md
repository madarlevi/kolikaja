<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<!-- Copyright (C) 2026 madarlevi -->

# Third-party licences

KoliKaja itself is licensed under the GNU Affero General Public License v3.0 only (see
[LICENSE](LICENSE)). The API binaries (`api`, `migrate`, `admin`) link the third-party Go
modules below. The full licence texts are shipped in the API container image under
`/usr/share/doc/kolikaja/licenses/` (collected at build time by `deploy/docker/api.Dockerfile`
from the modules that are actually linked).

This list was compiled from `go list -deps ./cmd/...` and the licence files in the module cache
on 2026-11-28. Update it whenever `apps/api/go.mod` changes. This is a list of notices, not legal
advice.

## Go modules linked into the API binaries

| Module | Version | Licence | Source |
|---|---|---|---|
| github.com/go-chi/chi/v5 | v5.5.0 | MIT | https://github.com/go-chi/chi |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | MIT | https://github.com/golang-jwt/jwt |
| github.com/golang-migrate/migrate/v4 | v4.20.1 | MIT | https://github.com/golang-migrate/migrate |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | https://github.com/google/uuid |
| github.com/jackc/pgx/v5 | v5.13.0 | MIT | https://github.com/jackc/pgx |
| github.com/jackc/pgerrcode | v0.0.0-20220416144525-469b46aa5efa | MIT | https://github.com/jackc/pgerrcode |
| github.com/jackc/pgpassfile | v1.0.0 | MIT | https://github.com/jackc/pgpassfile |
| github.com/jackc/pgservicefile | v0.0.0-20240606120523-5a60cdf6a761 | MIT | https://github.com/jackc/pgservicefile |
| github.com/jackc/puddle/v2 | v2.2.3 | MIT | https://github.com/jackc/puddle |
| golang.org/x/crypto | v0.59.0 | BSD-3-Clause (with PATENTS grant) | https://go.googlesource.com/crypto |
| golang.org/x/text | v0.44.0 | BSD-3-Clause (with PATENTS grant) | https://go.googlesource.com/text |
| golang.org/x/sync | v0.25.0 | BSD-3-Clause (with PATENTS grant) | https://go.googlesource.com/sync |
| golang.org/x/sys | v0.50.0 | BSD-3-Clause (with PATENTS grant) | https://go.googlesource.com/sys |
| Go standard library (including `time/tzdata`) | go1.27.4 | BSD-3-Clause (with PATENTS grant) | https://go.dev |

Test-only dependency (not in the binaries): gopkg.in/yaml.v3 (MIT and Apache-2.0).

## Container base image

The API image is built on `gcr.io/distroless/static-debian12:nonroot`, which contains Debian
packages under their own licences; their copyright files are in the image under
`/usr/share/doc/`. The `postgres:17-alpine` image (PostgreSQL License) is pulled by the operator
from Docker Hub and is not redistributed by this project.
