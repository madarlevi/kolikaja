---
name: legal-audit
description: Use when creating or changing LICENSE, COPYRIGHT, TRADEMARKS.md, DISCLAIMER.md, PRIVACY.md, SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, THIRD_PARTY_LICENSES.md, SPDX headers, or before declaring a release. Runs the consistency checklist from the project spec.
---

# Legal and licensing audit

These documents are project templates. They are not legal advice and must not claim legal compliance. Do not invent legal entities, addresses, contacts or registrations; use `TODO(operator)`.

## LICENSE
- Obtain the official GNU AGPL-3.0 text from the Free Software Foundation (gnu.org/licenses/agpl-3.0.txt). Copy it byte-for-byte. Never retype or edit it.
- Record `sha256sum LICENSE` in `docs/PROGRESS.md` and re-check it at every audit.
- Variant: `AGPL-3.0-only` as stated in `docs/DECISIONS.md` (D-04).
- Do not add trademark restrictions or extra terms to LICENSE. Reason: the AGPL grant must not be modified.

## COPYRIGHT
- States: `Copyright (C) <year> madarlevi`, the project name, and that copyright stays with the holder.
- Explains how third-party copyrights are handled and that third-party files keep their notices.
- Explains the relicensing position from `docs/DECISIONS.md` D-05: the copyright holder may change the license for code they own; contributions are covered by `CONTRIBUTING.md`; this is not legal advice.

## Source headers
- Each source file starts with a license header comment:
  `SPDX-License-Identifier: AGPL-3.0-only` and `Copyright (C) <year> madarlevi`.
- Check with grep across `apps/` and `deploy/`. Generated files get the header from the generator template, or are listed in a generated-files note.

## Consistency checks (run all, record results)
1. Same license identifier everywhere: `grep -R "AGPL" --include=*.md --include=*.go --include=*.ts --include=*.kt --include=*.swift` and compare.
2. Same copyright holder and year everywhere.
3. No `TODO(operator)` is missing from contact fields in SECURITY, CODE_OF_CONDUCT, PRIVACY, TRADEMARKS.
4. No ® symbol anywhere unless registration is verified (it is not).
5. README trademark notice links to `TRADEMARKS.md`.
6. Web footer, Android About screen and iOS About screen contain the copyright and trademark notice and link to `TRADEMARKS.md`.
7. Disclaimer does not exclude liability that law does not allow excluding (no claim of excluding consumer rights).
8. DISCLAIMER states the food/allergen limitation and that users must verify allergen information with the provider.
9. THIRD_PARTY_LICENSES lists name, version, SPDX license identifier and source URL for every dependency in Go, npm, Gradle and SwiftPM. Generate the list with tools where possible (`go-licenses`, `license-checker`, Gradle license plugin); flag unknown or incompatible licenses.
10. No dependency with a license incompatible with AGPL-3.0 without explicit justification in THIRD_PARTY_LICENSES.
11. The final report says which items are verified, which are `TODO(operator)`, and which were not checked.

## Output
Write findings to `docs/LEGAL_AUDIT.md` with date, checklist results and open items. Do not mark the release checklist complete unless each file exists and has been checked.
