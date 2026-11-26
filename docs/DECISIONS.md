# KoliKaja – decision log

These decisions override `docs/SPEC.md` where they conflict. Add new decisions at the bottom with a date and a reason.

## Confirmed by the maintainer

| # | Topic | Decision | Source |
|---|-------|----------|--------|
| D-01 | Docker | Not available in the current working environment. Runtime Docker steps are recorded as "not verified". | Maintainer answer |
| D-02 | Mobile builds | No Android SDK or macOS/Xcode available yet. Android and iOS code is written to be buildable, but builds are not verified until the maintainer lists the required tools in `docs/ENVIRONMENT.md`. | Maintainer answer |
| D-03 | UI language | Hungarian is the default language. English is supported through i18n resource files in every client (web, Android, iOS). No hardcoded user-facing strings in components. | Maintainer answer |
| D-04 | License version | GNU AGPL-3.0 (single version, `AGPL-3.0-only`) for now. Replace `AGPL-3.0-only` with `AGPL-3.0-or-later` in all files only if the maintainer asks. | Maintainer answer |
| D-05 | Relicensing right | The copyright holder (madarlevi) must be able to change the project license later. This is stated in `COPYRIGHT` and `CONTRIBUTING.md`, **not** inside `LICENSE`, because `LICENSE` must stay the unmodified official text. See the open question below. | Maintainer answer |

## Open question (needs legal review, not a technical choice)
- D-05 is only fully effective for code that madarlevi owns. Contributions from others remain under the
  inbound terms in `CONTRIBUTING.md`. A relicensing right for contributions requires either a contributor
  license grant or a CLA, which the project spec forbids unless justified and documented. Until this is
  decided, `CONTRIBUTING.md` states that contributors keep their copyright and license their contributions
  under the project license, and that the maintainer can relicense only code they own. This is not legal advice.

## Technical decisions (made by the agent, to be reviewed)
- Language of code, comments and API identifiers: English.
- API version prefix: `/api/v1/`.
- Web authentication: HttpOnly, Secure, SameSite cookies with CSRF double-submit token.
- Mobile authentication: short-lived bearer access token plus rotating refresh token, both stored in Keystore / Keychain.
- Refresh tokens are opaque random values, stored only as SHA-256 hashes, rotated on every use.
- Password hashing: Argon2id, parameters defined in `apps/api/internal/auth/argon2.go` and documented in `ARCHITECTURE.md`.
- Photos: presigned upload to MinIO, validated by magic bytes, size-limited, metadata stripped (EXIF) before serving.
- Migrations: golang-migrate, files named `NNNN_description.up.sql` / `NNNN_description.down.sql`.

## Rating labels (exact text, UTF-8, do not change)
1. Le se menj
2. Ha nem vagy éhenhalás szélén, hagyd
3. Ehető, ha nincs más
4. Amúgy eskü nem olyan szar
5. Már határozottan ehetőnek nevezhető
