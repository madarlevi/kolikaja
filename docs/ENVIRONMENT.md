# Environment capabilities

Update this file whenever the environment changes. The agent reads it before deciding what it can verify.

| Capability | Status | Notes |
|------------|--------|-------|
| Go toolchain | to check | Needed for `apps/api`. Record version here once confirmed. |
| Node.js + npm | to check | Needed for `apps/web`. |
| PostgreSQL (local, for integration tests) | to check | Can be replaced by a test container if Docker exists. |
| Docker daemon | **not available** | Compose build, image build and runtime checks are "not verified". |
| Android SDK / Gradle | **not available** | Required for `apps/android` build. Maintainer will list the tools to install. |
| macOS + Xcode | **not available** | Required for `apps/ios` build. Maintainer will list the tools to install. |
| Outbound network | restricted | Package registries may be blocked. Record failures in `PROGRESS.md`; do not work around restrictions. |

## When a capability becomes available
1. Change its status here.
2. Run the verification commands listed in the app's `CLAUDE.md`.
3. Update `docs/PROGRESS.md` items that were marked "not verified".
