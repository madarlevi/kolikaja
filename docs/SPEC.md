# KoliKaja – Full Open Source Platform

You are a senior software architect and full-stack engineering team. Build a complete, production-oriented, open-source application called **KoliKaja**.

Do not create just a prototype or UI mockup. Build a functional application with a secure backend API, web frontend, native Android application, native iOS application, database, file storage and complete Docker Compose deployment.

Make independent technical decisions where necessary. Prioritize security, performance, maintainability, low resource consumption and excellent user experience.

Make many git commits with good comments so it is easy to use. You don't need tu push.

# 1. PROJECT VISION

KoliKaja is a multi-dormitory food menu and community rating platform.

Students can:
- Discover and join dormitories.
- View daily and weekly food menus.
- Rate meals from 1 to 5 stars.
- Upload photos of meals.
- Comment on meals.
- See average ratings and community feedback.
- Recommend or warn others about meals.
- Manage their profiles.
- Join multiple dormitories.

Dormitories are independent communities inside the platform.

The system must support multiple independently managed dormitories with strict tenant isolation.

The project is open-source-first and must be completely self-hostable.

# 2. TECHNOLOGY STACK

Use the following technologies.

## Backend
- Go (latest stable version).
- Chi HTTP router.
- PostgreSQL.
- pgx for PostgreSQL access.
- SQL migrations using golang-migrate or an equivalent maintained migration tool.
- Argon2id password hashing.
- JWT access tokens.
- Rotating refresh tokens.
- OpenAPI 3.1 API documentation.

Keep the backend modular and easy to maintain.

Do not introduce unnecessary microservices.

## Web frontend
- React.
- TypeScript.
- Vite.
- Tailwind CSS.
- React Router.
- TanStack Query for server state.
- A maintained component library or custom accessible components.

The web application must be fully responsive and mobile-first.

Support PWA installation.

## Android
- Kotlin.
- Jetpack Compose.
- Material 3.
- Retrofit or Ktor Client.
- Kotlin Coroutines.
- Secure token storage using Android Keystore-backed mechanisms.

Build a genuinely native Android application.

## iOS
- Swift.
- SwiftUI.
- URLSession.
- Swift Concurrency.
- Keychain for secure token storage.

Build a genuinely native iOS application.

## Infrastructure
- Docker Compose.
- PostgreSQL.
- MinIO (S3-compatible object storage).
- Go API.
- React web application.
- Reverse proxy if appropriate.

All infrastructure must be self-hostable.

# 3. BACKEND API

The Go backend is the central source of truth.

All clients must communicate through the same documented REST API.

Use versioned routes:

/api/v1/

Implement the following API modules.

## Authentication
- User registration.
- Login.
- Logout.
- Refresh access tokens.
- Revoke refresh tokens.
- Retrieve current user.
- Password change.
- Account deletion.
- Session management.

Use Argon2id for password hashing.

Never store plaintext passwords.

Use short-lived access tokens and rotating refresh tokens.

Implement refresh-token reuse detection and session revocation.

Support secure HTTP-only cookies for the web client and bearer-token authentication for native applications.

## Users
- User profiles.
- Profile pictures.
- Display names.
- User statistics.
- Rating history.
- Reputation.
- Trusted reviewer status.
- Account settings.

## Dormitories
- Create dormitories.
- Search public dormitories.
- Retrieve dormitory details.
- Update dormitory information.
- Delete dormitories with appropriate authorization.
- Join and leave dormitories.
- Public and private dormitories.
- Invitation codes.
- Membership management.
- Dormitory-specific settings.

Users can belong to multiple dormitories.

## Dormitory roles

Implement:
- Member.
- Moderator.
- Dormitory administrator.
- Global administrator.

Implement role-based access control.

Never trust roles or permissions supplied by clients.

## Menus and meals

Implement:
- Create menus.
- Edit menus.
- Delete menus.
- Daily menus.
- Weekly menus.
- Meal categories.
- Individual meal details.
- Optional descriptions.
- Optional allergen information.
- Meal photos.
- Menu publishing status.

PDF import is NOT required yet.

Menus must be manageable manually through the admin dashboard and API.

## Ratings

Use the following exact Hungarian rating labels:

1. Le se menj
2. Ha nem vagy éhenhalás szélén, hagyd
3. Ehető, ha nincs más
4. Amúgy eskü nem olyan szar
5. Már határozottan ehetőnek nevezhető

Implement:
- One active rating per user per meal.
- Rating creation.
- Rating editing.
- Rating deletion.
- Average rating.
- Total rating count.
- Rating distribution.
- Rating timestamps.

Prevent duplicate ratings through database constraints.

## Comments

Implement:
- Create comments.
- Edit own comments.
- Delete own comments.
- Moderator deletion.
- Comment pagination.
- Basic reporting functionality.

## Photos

Implement:
- Upload meal photos.
- Retrieve photos.
- Delete own photos.
- Moderator photo removal.
- File type validation.
- File size limits.
- Safe object storage.
- Image metadata handling.

Use MinIO through its S3-compatible API.

Use presigned upload URLs where appropriate.

Never expose private storage credentials to clients.

## Recommendations

Implement community recommendations and warnings.

Users should be able to flag a meal as:
- Recommended.
- Not recommended.

Make sure these are separate from the numerical rating.

Implement useful community statistics.

## Moderation

Implement:
- Report users.
- Report comments.
- Report photos.
- Moderator review queue.
- Content removal.
- User suspension.
- Audit logging.

## Notifications

Prepare an extensible notification architecture.

Implement in-app notifications for:
- New comments.
- Important dormitory announcements.
- Moderation-related account notifications.

Push notifications can be added later.

# 4. API KEY AND AUTHENTICATION SECURITY

Security is a major priority.

IMPORTANT:

Never embed a privileged API key in the React, Android or iOS applications.

Client applications are public software. Any embedded secret can be extracted.

Use two authentication mechanisms.

## User authentication
- JWT access tokens.
- Rotating refresh tokens.
- Secure web cookies.
- Bearer tokens for mobile clients.
- Session revocation.

## Server-to-server API keys

Implement API keys for external integrations.

Requirements:
- Cryptographically secure random generation.
- Display the full key only once.
- Store only a secure hash of each API key.
- Support revocation.
- Support expiration.
- Support scopes and permissions.
- Associate keys with a dormitory or service account.
- Record usage and audit events.
- Rate limiting.

Use the HTTP header:

X-API-Key: YOUR_API_KEY

API keys must never bypass tenant isolation or authorization.

# 5. SECURITY REQUIREMENTS

Implement security as a core feature, not an afterthought.

Include:
- SQL injection protection using parameterized queries.
- Strict authorization checks.
- Tenant isolation.
- Input validation.
- Output encoding.
- CORS configuration.
- CSRF protection for cookie-authenticated web requests.
- Rate limiting.
- Secure HTTP headers.
- Request size limits.
- File upload validation.
- Password hashing with Argon2id.
- Refresh token rotation.
- Audit logging.
- Secure error responses.
- No secret values in logs.
- No hardcoded credentials.
- Secure production configuration.

Do not expose stack traces or internal database errors to clients.

Validate every request on the backend.

Create automated authorization tests, especially for cross-dormitory access attempts.

# 6. DATABASE DESIGN

Use PostgreSQL.

Create proper relational database schemas and migrations.

At minimum implement:
- users
- user_sessions
- dormitories
- dormitory_members
- invitations
- menus
- meals
- ratings
- comments
- meal_photos
- recommendations
- notifications
- reports
- api_keys
- audit_logs

Use:
- Foreign keys.
- Appropriate unique constraints.
- Database indexes.
- Transactions.
- Timestamps.
- Soft deletion where appropriate.

Make tenant ownership explicit.

Never rely exclusively on frontend filtering for tenant isolation.

# 7. WEB APPLICATION

Build a polished, modern, responsive web application.

Design direction:
- Dark mode as default.
- Optional light mode.
- Modern typography.
- Rounded cards.
- Excellent mobile layout.
- Smooth transitions.
- Accessible components.
- Clean navigation.
- Attractive food photography.

Main pages:

1. Home / Today's menu.
2. Weekly menu.
3. Meal details.
4. Ratings and comments.
5. Photo gallery.
6. Dormitory discovery.
7. My dormitories.
8. User profile.
9. Notifications.
10. Settings.
11. Authentication.
12. Dormitory administration.
13. Global administration.

The frontend must use the actual backend API.

No fake data or hardcoded production menus.

Include useful loading, empty, error and offline states.

# 8. ANDROID APPLICATION

Build a native Android application with Kotlin and Jetpack Compose.

Include:
- Authentication.
- Dormitory selection.
- Today's menu.
- Weekly menu.
- Meal details.
- Ratings.
- Comments.
- Photo upload.
- User profiles.
- Notifications.
- Settings.

Use the same API as the web application.

Implement proper loading and error states.

Use secure token storage.

Follow Android architectural best practices.

# 9. IOS APPLICATION

Build a native iOS application with SwiftUI.

Include the same essential functionality as the Android application.

Use:
- URLSession.
- Swift Concurrency.
- Keychain.
- Native navigation.
- Native photo selection.
- Native camera/photo upload support.

Use the same API contract as Android and web.

Follow Apple's platform conventions and accessibility guidelines.

# 10. API CONTRACT

Create a complete OpenAPI 3.1 specification.

Document:
- All endpoints.
- Request schemas.
- Response schemas.
- Authentication.
- Authorization errors.
- Validation errors.
- Pagination.
- Rate limits.

Use the OpenAPI specification as the source of truth.

Generate or maintain typed API clients for TypeScript, Kotlin and Swift where practical.

Keep all three clients compatible with the same API version.

# 11. DOCKER COMPOSE

Docker Compose is mandatory.

The project must start with:

```bash
git clone https://github.com/madarlevi/kolikaja.git
cd kolikaja
cp .env.example .env
docker compose up -d --build
```

Provide:
- docker-compose.yml.
- API Dockerfile.
- Web Dockerfile.
- PostgreSQL.
- MinIO.
- Persistent volumes.
- Health checks.
- Automatic database migrations.
- Restart policies.
- Environment configuration.
- Production deployment configuration.
- Backup and restore instructions.

Use minimal production images.

Run containers as non-root users whenever possible.

Never ship default production credentials.

The first installation must be clearly documented.

Native Android and iOS applications are built separately and must be able to connect to any compatible self-hosted KoliKaja API.

# 12. OPEN SOURCE

Use AGPL-3.0.

Include:
- LICENSE.
- README.md.
- CONTRIBUTING.md.
- SECURITY.md.
- CODE_OF_CONDUCT.md.
- .env.example.
- API documentation.
- Docker documentation.
- Development instructions.

LICENSE is (c) madarlevi

The project must not require any paid external service to function.

No cloud subscription or payment implementation yet.

No telemetry without explicit user consent.

# 13. EXPLICITLY OUT OF SCOPE FOR THIS VERSION

Do NOT implement:
- PDF parsing.
- OCR.
- Automatic menu scraping.
- n8n workflows.
- Payment systems.
- Cloud subscription management.
- AI recommendations.
- External automation services.

Design the architecture so these can be added later without major rewrites.

# 14. DEVELOPMENT AND DELIVERY PROCESS

Work directly in the current repository.

First:
1. Inspect the repository.
2. Write an architecture document.
3. Create the folder structure.
4. Define the database schema.
5. Define the OpenAPI contract.
6. Implement the backend.

Then implement:
7. Docker Compose infrastructure.
8. Web application.
9. Android application.
10. iOS application.
11. Automated tests.
12. Documentation.

Do not stop after creating a plan.

Actually implement the code.

Do not replace real functionality with placeholder buttons.

Run backend tests and frontend tests.

Build the Docker images.

Verify that the API starts and the database migrations execute successfully.

Verify authentication and authorization.

Test cross-tenant isolation.

If you cannot run Android or iOS builds in the current environment, document this limitation and ensure the project files are complete and buildable.

Keep the code clean, modular, documented and maintainable.

Make sensible decisions independently instead of asking unnecessary questions.

# 15. OPEN-SOURCE LEGAL, COMPLIANCE AND COMMUNITY REQUIREMENTS

Treat open-source compliance and legal documentation as mandatory project deliverables.

Do not merely create empty files or generic placeholder documents. Create complete, carefully written and internally consistent documentation.

## 15.1 License

Use the GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later) as the primary project license.

Create a complete LICENSE file containing the official, unmodified license text.

Requirements:
- Add appropriate copyright notices.
- Add SPDX license identifiers to source files where appropriate.
- Clearly explain the rights and obligations of users, contributors and operators.
- Explain the network-use source code disclosure requirements.
- Make clear that commercial use is permitted subject to the license terms.
- Do not claim that AGPL prohibits commercial use.
- Do not invent additional restrictions that conflict with AGPL.

Use the official license text from the Free Software Foundation.

## 15.2 Contributor documentation

Create a comprehensive CONTRIBUTING.md.

Document:
- How to set up the development environment.
- How to run the project locally.
- How to run Docker Compose.
- How to run tests.
- Code formatting and linting requirements.
- Branch naming conventions.
- Commit message conventions.
- Pull request requirements.
- Issue reporting.
- Code review expectations.
- Database migration requirements.
- API compatibility requirements.
- Security expectations.
- Documentation expectations.

Require contributors to ensure that their contributions can legally be distributed under the project's AGPL-3.0-or-later license.

Do not introduce a Contributor License Agreement unless explicitly justified and documented.

## 15.3 Code of Conduct

Create a CODE_OF_CONDUCT.md.

Use the Contributor Covenant as the foundation.

Include:
- Expected community behavior.
- Unacceptable behavior.
- Reporting procedures.
- Moderation and enforcement.
- Consequences of violations.

## 15.4 Security policy

Create a comprehensive SECURITY.md.

Include:
- Supported versions.
- Responsible vulnerability disclosure.
- How to report vulnerabilities privately.
- What information a security report should contain.
- Expected response process.
- Security update policy.
- Dependency vulnerability handling.

Do not instruct users to publicly disclose exploitable vulnerabilities before maintainers have had a reasonable opportunity to investigate.

Do not invent a security contact email address. Use a clearly marked placeholder until the maintainer supplies one.

## 15.5 Disclaimer and limitation of liability

Create a DISCLAIMER.md and include an appropriate warranty disclaimer consistent with AGPL-3.0.

Clearly state that:
- The software is provided "AS IS", without warranties of any kind, to the extent permitted by applicable law.
- The maintainers do not guarantee uninterrupted operation, correctness, availability, security or fitness for a particular purpose.
- Users and self-hosting operators are responsible for their own deployment, configuration, backups, access controls, security updates and legal compliance.
- The maintainers are not responsible for user-generated content, ratings, comments, uploaded images or information supplied by dormitories.
- The software must not be relied upon as the sole source of information about food ingredients, allergens, nutritional values or food safety.
- Users should independently verify important food and allergen information with the relevant provider.
- No guarantee is made that the software is free of bugs or vulnerabilities.

Include a limitation of liability clause to the maximum extent permitted by applicable law.

Do not claim that the maintainers can exclude liability in situations where applicable law does not allow it.

Do not imply that this document removes mandatory consumer rights, data protection obligations or other non-waivable legal responsibilities.

## 15.6 Privacy documentation

Create a PRIVACY.md template.

Document the types of personal data the application may process, including:
- Account information.
- Email addresses.
- Profile information.
- Dormitory memberships.
- Ratings and comments.
- Uploaded images.
- Session and security logs.
- API key metadata.
- Notifications.

Explain that self-hosting operators are responsible for configuring and providing the appropriate privacy information for their own deployment.

Include sections for:
- Data controller identification.
- Legal basis for processing.
- Data retention.
- Data deletion.
- User rights.
- Cookies and sessions.
- Security measures.
- Third-party services.
- Data exports.

Do not invent a legal entity, company address or privacy contact.

Clearly mark information requiring operator completion.

## 15.7 Third-party licenses and dependencies

Create THIRD_PARTY_LICENSES.md.

Audit all dependencies used by:
- Go backend.
- React and TypeScript frontend.
- Android application.
- iOS application.
- Docker infrastructure.

Document:
- Dependency name.
- Version.
- License identifier.
- Source repository or official project URL.

Check for incompatible or restrictive licenses.

Do not falsely claim that every dependency is open source without checking its actual license.

Do not include proprietary or incompatible dependencies without explicit justification.

Generate a repeatable dependency license audit process where practical.

## 15.8 Copyright and trademarks

Create COPYRIGHT and TRADEMARKS.md.

Explain:
- The project copyright notice.
- How third-party copyrights are handled.
- That the KoliKaja name and logo may be subject to separate trademark protection.
- That the AGPL license does not automatically grant unrestricted trademark rights.

Do not claim trademark registration unless it actually exists.

## 15.9 GitHub community files

Create:
- .github/ISSUE_TEMPLATE/bug_report.yml
- .github/ISSUE_TEMPLATE/feature_request.yml
- .github/PULL_REQUEST_TEMPLATE.md
- .github/SECURITY.md

Create clear and useful templates.

## 15.10 Legal consistency audit

Before completing the project:
1. Verify that all license references are consistent.
2. Verify that the LICENSE file contains the complete official AGPL text.
3. Verify that copyright notices are consistent.
4. Verify that third-party dependencies have been audited.
5. Verify that documentation does not make unsupported legal claims.
6. Verify that disclaimers do not contradict the actual license.
7. Verify that all placeholder contact details are clearly identified.
8. Verify that the project does not claim legal compliance merely because documentation files exist.

Important: These documents are software project documentation templates, not a substitute for legal advice. Do not fabricate legal entities, registrations, contact details or compliance certifications.

# 16. FINAL OPEN-SOURCE RELEASE CHECKLIST

Before declaring the repository complete, verify that:

- [ ] LICENSE exists and contains the complete AGPL-3.0-or-later text.
- [ ] README explains the license.
- [ ] CONTRIBUTING.md is complete.
- [ ] CODE_OF_CONDUCT.md exists.
- [ ] SECURITY.md exists.
- [ ] DISCLAIMER.md exists.
- [ ] PRIVACY.md exists.
- [ ] THIRD_PARTY_LICENSES.md exists.
- [ ] COPYRIGHT exists.
- [ ] TRADEMARKS.md exists.
- [ ] GitHub issue and pull request templates exist.
- [ ] All dependencies have been checked for license compatibility.
- [ ] No secrets, private keys or credentials are committed.
- [ ] The Docker deployment instructions are complete.
- [ ] The self-hosting documentation explains operator responsibilities.

Do not mark any checklist item as complete unless the corresponding file and its contents have actually been created and checked.

## Additional Trademark Implementation Requirements

Implement the KoliKaja trademark policy consistently throughout the repository.

1. Create a complete `TRADEMARKS.md` file.
2. Add a short trademark notice to the README.
3. Add a copyright and trademark notice to the web application's footer.
4. Add an About / Legal section to the Android and iOS applications.
5. Add a link to the trademark policy in the legal information sections.
6. Keep the AGPL software license legally separate from trademark permissions.
7. Do not claim that KoliKaja is a registered trademark.
8. Do not use the ® symbol unless registration has been independently verified.
9. Use a configurable maintainer contact placeholder rather than inventing contact information.
10. Ensure that forks can freely modify and redistribute the AGPL-licensed code under their own names and branding, subject to the AGPL.
11. Do not add trademark restrictions to the AGPL license itself.
12. Do not remove existing third-party copyright, attribution or license notices.

The project must clearly distinguish between:
- Open-source software rights.
- Copyright ownership.
- Trademark and branding permissions.

# FINAL GOAL

Deliver a fully functional, secure, self-hostable, open-source KoliKaja platform.

One central API.

Three clients:
- Web.
- Native Android.
- Native iOS.

Multiple independent dormitories.

Docker Compose deployment.

No mandatory external services.

The result must be a real software project that developers can clone, run, modify and contribute to.