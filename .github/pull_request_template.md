## What and why

<!-- One or two sentences. Link the issue if there is one. -->

## Type

- [ ] feat · [ ] fix · [ ] docs · [ ] ci · [ ] build · [ ] refactor · [ ] test · [ ] chore
  (Conventional Commits; the squash-merge title must follow the same format)

## Checklist

- [ ] CI is green (Go, web, OpenAPI, authorization matrix, security jobs).
- [ ] Contract first: `docs/api/openapi.yaml` changed before the server; `scripts/check-authz-matrix.sh --write` run and committed.
- [ ] Every new or changed dormitory-scoped endpoint has cross-dormitory tests in `apps/api/tests/authz/`.
- [ ] Migrations: new up and down files only; no released migration edited.
- [ ] No secrets, credentials, tokens or personal data in code, tests, fixtures, logs or docs.
- [ ] Only parameterized SQL; no string building passed to database calls.
- [ ] User-facing text goes through i18n (Hungarian default, English); rating labels unchanged.
- [ ] `docs/PROGRESS.md` updated; `docs/DECISIONS.md` updated if a decision was made.
- [ ] Security gate: auth, sessions, uploads or authorization changed → `security-reviewer` subagent run and findings recorded.
- [ ] Licensing, headers or documentation changed → `legal-audit` run; new source files carry the SPDX header.

## Verification

<!-- Commands run and their results, or the CI run URL. Mark anything not verified and why. -->
