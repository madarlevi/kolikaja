# Security policy

KoliKaja is in early development (phase 1 of 7 is complete; there is no released server yet). This is a
preliminary policy; the complete policy (supported versions, response process, update policy) is written in
phase 7.

## Reporting a vulnerability
Please **do not** open a public issue, discussion or pull request for a security problem.

Report privately to: **TODO(operator): security contact address – not yet provided by the maintainer.**

Until the contact exists, open a public issue that only says you have a security report and asks for a
private channel, without any technical details.

Include, if you can: the affected component (API, web app, deployment, contract) and version or commit,
the steps to reproduce, the impact you observed, and whether the issue is already public. Never send real
user data, passwords or tokens.

## Disclosure
Please give the maintainers a reasonable opportunity to investigate and fix the problem before any public
disclosure. We will credit reporters who want to be credited.

## Scope
This repository: the Go API, the web app, the deployment files and the API contract. The Android and iOS
apps have their own policies in madarlevi/kolikaja-android and madarlevi/kolikaja-ios.

Security tooling and the per-phase security gate are described in `docs/SECURITY-PLAN.md`.
