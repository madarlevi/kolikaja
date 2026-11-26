# KoliKaja – legal and licensing audit log

These checks compare project files for consistency. They are not legal advice and do not establish
legal compliance. Checklist: `.claude/skills/legal-audit/SKILL.md`.

## 2026-11-26 – phase 1 (scoped audit)

Scope: files created in phase 1 (SQL migrations, OpenAPI contract, lint and git configuration,
`docs/ARCHITECTURE.md`, additions to `docs/DECISIONS.md`, `docs/ENVIRONMENT.md`). The legal documents
themselves are phase 7 deliverables and do not exist yet.

| # | Check | Result | Evidence |
|---|-------|--------|----------|
| 1 | Same license identifier everywhere | PASS (scope) | All 36 phase 1 files with comment support carry `SPDX-License-Identifier: AGPL-3.0-only` (D-04). Other `AGPL-3.0` mentions refer to the license text (section 13) or are the decision log itself. `docs/SPEC.md` still says AGPL-3.0-or-later; D-04 overrides it. |
| 2 | Same copyright holder and year | PASS (scope) | 36 × `Copyright (C) 2026 madarlevi`; no other holder. |
| 3 | `TODO(operator)` in contact fields | NOT CHECKED | SECURITY, CODE_OF_CONDUCT, PRIVACY, TRADEMARKS do not exist yet (phase 7). Operator fields in the design (`PRIVACY_POLICY_URL`, `TERMS_URL`, `TRADEMARKS_URL`, `OPERATOR_CONTACT`) default to empty and are marked `TODO(operator)` in `docs/ARCHITECTURE.md` §3.4. |
| 4 | No ® symbol | PASS | Only occurrences are the rules that forbid it (`SPEC.md`, skill, agent). |
| 5 | README trademark notice | NOT CHECKED | README does not exist yet (phase 7). |
| 6 | Footer / About screens | NOT CHECKED | Clients not built yet; the design requires them (`docs/ARCHITECTURE.md` §14). |
| 7 | Disclaimer liability wording | NOT CHECKED | Phase 7. |
| 8 | Food / allergen limitation | NOT CHECKED (design PASS) | DISCLAIMER is phase 7; the contract and architecture require clients to tell users to verify allergen data with the provider. |
| 9 | THIRD_PARTY_LICENSES | NOT CHECKED | No dependencies are committed yet. The scratch verification harness (embedded-postgres, golang-migrate, pgx) lives outside the repository and is not distributed. |
| 10 | Incompatible dependencies | NOT CHECKED | As 9. Open question: MinIO server distribution terms (ARCHITECTURE §18). |
| 11 | Unsupported legal claims | PASS (scope) | No "compliant", "certified", "registered trademark", "guarantee", "fully secure", "bug-free" or "GDPR" in the new documents. ARCHITECTURE §16 says explicitly that it is not legal advice. |
| – | LICENSE file and checksum | NOT CHECKED | `LICENSE` does not exist yet; it must be the unmodified FSF text (phase 7). Source headers already reference `AGPL-3.0-only`. |

Remaining `TODO(operator)` items from this phase: privacy policy URL, terms URL, trademarks URL and operator
contact (instance configuration), and `SOURCE_CODE_URL` for operators running modified versions.
