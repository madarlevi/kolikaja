---
name: android-compose-conventions
description: Use when building or changing the native Android app in apps/android (Kotlin, Jetpack Compose, Material 3): architecture layers, ViewModels and state, Keystore token storage, networking, localization, photo upload and tests.
---

# Android (Kotlin / Compose) conventions

## Architecture
- Unidirectional data flow: composables receive a `UiState` data class and emit events. ViewModels expose `StateFlow<UiState>` and handle one-shot effects through a `Channel`.
- Repository per feature, the only class that calls the API client. Repositories return `Result`-like sealed types (`Success`, `Error` with a mapped user-facing reason).
- Hilt for dependency injection; modules in `core/di/`.
- Navigation Compose with type-safe routes; deep links defined for menu and meal detail.

## State and errors
- Map HTTP errors and network failures to domain errors: `Offline`, `Unauthorized`, `Forbidden`, `NotFound`, `Validation`, `Server`. Each maps to a string resource.
- Every screen shows loading, empty, error (with retry) and offline states.

## Token storage
- Access and refresh tokens are encrypted with an AES-GCM key in the Android Keystore. Store ciphertext in DataStore.
- Keys are non-exportable and require the app to be unlocked where practical.
- Tokens never appear in logs, crash reports, or intents.
- Logout clears the token store and cancels pending requests.

## Networking
- Client generated from `docs/api/openapi.yaml`. Do not hand-write endpoints.
- OkHttp interceptors: auth header injection, one refresh attempt on 401 (mutex-protected so parallel requests refresh once), request ID header, logging interceptor disabled in release builds.
- TLS: default system trust. Certificate pinning is not added in this version; document the decision in `ARCHITECTURE.md`.
- Network security config disallows cleartext outside debug.

## Localization and UI
- `values/strings.xml` (English) and `values-hu/strings.xml` (Hungarian). No hardcoded text in composables.
- Material 3 theme with dynamic color optional; dark theme supported.
- Content descriptions for all icons and images. Touch targets 48dp.
- Photo upload with the system photo picker; compress before upload; never request broad storage permission.

## Build and quality (once the SDK is available)
- Gradle Kotlin DSL, version catalog in `gradle/libs.versions.toml`.
- Commands: `./gradlew ktlintCheck detekt testDebugUnitTest assembleDebug`.
- Release: R8 on, `minifyEnabled true`, ProGuard/R8 rules kept minimal and documented.
- Until the SDK is available: write code, keep imports and signatures consistent, and record "not verified".
