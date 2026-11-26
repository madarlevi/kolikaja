# Environment capabilities

Update this file whenever the environment changes. The agent reads it before deciding what it can verify.
Last checked: 2026-11-26 (session 2), Windows 11 Pro, Git Bash.

**Verification environment (D-08):** everything that cannot run locally – Docker images, the Compose stack,
the ZAP scan, PostgreSQL-based tests in a container, Android and iOS builds – is verified by GitHub Actions.
Record the run URL in `docs/PROGRESS.md`; a step without a local run or a CI run URL is "not verified".

| Capability | Local status | Notes |
|------------|--------------|-------|
| Go toolchain | **available** | `go.mod` pins `toolchain go1.27.4`; Go downloads it automatically (installed: go1.26.5). |
| Node.js + npm | **available** | Node `v24.13.1`, npm `11.21.0`. npm registry reachable. |
| PostgreSQL (integration tests) | no server installed; embedded binaries work | `fergusstrange/embedded-postgres` (scratch helper, outside the repository) ran PostgreSQL 17.5; the in-repo tests take any empty database via `KOLIKAJA_TEST_DATABASE_URL`. CI uses a PostgreSQL 17 service container. |
| Docker daemon | **not available** | Docker CLI 29.5.3 is installed but not on `PATH`; the daemon is not running (D-01). CI builds images and runs Compose. |
| Android SDK / Gradle | not available locally | Out of this repository since the split (D-06); kolikaja-android builds in GitHub Actions. |
| macOS + Xcode | not available locally | Out of this repository since the split (D-06); kolikaja-ios builds on `macos-latest` runners. |
| Java | available | Eclipse Temurin JDK 21; runs the OpenAPI Generator for the mobile clients. |
| Python | available | Python 3.13.5 with PyYAML; Semgrep 1.187.0 works in a scratch virtual environment. |
| golangci-lint, gosec, govulncheck, actionlint | via `go run` | Versions pinned in `.github/workflows/ci.yml`; all ran locally in session 2. |
| Redocly CLI | via `npx` | `npx @redocly/cli@2.84.0 lint docs/api/openapi.yaml`. |
| `jq` | **not installed** | Both project hooks exit early without `jq`, so the secret-file guard and auto-formatting are **inactive**. |
| `shellcheck` | not installed | actionlint uses it in CI (ubuntu runners have it). |
| GitHub CLI `gh` | not installed at the start of session 2 | Needed for pull requests, merges, run URLs, description and topics; the maintainer installs and authenticates it (`winget install GitHub.cli`, `gh auth login`). |
| Git credentials | Git Credential Manager | Pushes to `origin` use the maintainer's stored credentials. |
| Outbound network | available | npm, Go proxy, Maven Central, PyPI, the Semgrep registry and gnu.org were reachable. Commands outside the permission settings can still be blocked by the Claude Code permission policy; do not work around such blocks. |

## When a capability becomes available
1. Change its status here.
2. Run the verification commands listed in the app's `CLAUDE.md`.
3. Update `docs/PROGRESS.md` items that were marked "not verified".
