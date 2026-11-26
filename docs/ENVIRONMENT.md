# Environment capabilities

Update this file whenever the environment changes. The agent reads it before deciding what it can verify.
Last checked: 2026-11-26 (phase 1), Windows 11 Pro, Git Bash.

| Capability | Status | Notes |
|------------|--------|-------|
| Go toolchain | **available** | `go1.26.5 windows/amd64`. Go module proxy (`proxy.golang.org`) reachable. |
| Node.js + npm | **available** | Node `v24.13.1`, npm `11.21.0`. npm registry reachable. |
| PostgreSQL (local, for integration tests) | **no server installed**; embedded binaries work | No `psql` or local service. `github.com/fergusstrange/embedded-postgres` v1.34.0 downloaded and ran PostgreSQL 17.5 from Maven Central in a scratch harness (phase 1 migration verification). Candidate for phase 2 integration tests while Docker is unavailable. |
| Docker daemon | **not available** | Docker CLI 29.5.3 is installed (`C:\Program Files\Docker\Docker\resources\bin`, not on `PATH`), but the daemon is not running and `docker compose` is not available through that CLI. Per D-01, Compose build, image build and runtime checks are "not verified". |
| Android SDK / Gradle | **not available** | Required for `apps/android` build. Maintainer will list the tools to install. |
| macOS + Xcode | **not available** | Required for `apps/ios` build. Maintainer will list the tools to install. |
| Java | available | Eclipse Temurin JDK 21 (`java` on `PATH`). Usable later for `openapi-generator` (Kotlin and Swift clients). |
| Python | available | Python 3.13.5 with PyYAML 6.0.3 (used for ad-hoc contract checks). |
| Redocly CLI | via `npx` | `npx @redocly/cli@2.84.0 lint docs/api/openapi.yaml` works (downloads on first use). |
| `jq` | **not installed** | Both project hooks (`.claude/hooks/block-secret-files.sh`, `format-on-edit.sh`) exit early without `jq`, so the secret-file guard and auto-formatting are currently **inactive**. |
| `golangci-lint`, `migrate` CLI, `hadolint`, `yamllint`, `oasdiff` | not installed | Record the corresponding steps as "not run" until installed. |
| Outbound network | partially available | npm registry, Go proxy and Maven Central were reachable. Running npm packages that are not covered by the permission settings can be blocked by the Claude Code permission policy (one OpenAPI validator was blocked in phase 1). Do not work around such blocks. |

## When a capability becomes available
1. Change its status here.
2. Run the verification commands listed in the app's `CLAUDE.md`.
3. Update `docs/PROGRESS.md` items that were marked "not verified".
