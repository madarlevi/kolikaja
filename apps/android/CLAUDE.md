# apps/android – native Android rules

## Stack
Kotlin, Jetpack Compose, Material 3, Kotlin Coroutines + Flow, Retrofit (or Ktor Client, decided in ARCHITECTURE.md), kotlinx.serialization, Hilt for DI, Navigation Compose, Coil for images, AndroidX Security / Android Keystore for token storage.

## Layout (feature-first, MVVM with UDF)
```
app/src/main/java/hu/…/kolikaja/
  core/network/      client, interceptors, error mapping
  core/auth/         token store (Keystore-backed), session manager
  core/i18n/         string resources (values/ = English, values-hu/ = Hungarian)
  feature/<name>/    ui/ (composables), vm/ (ViewModel), data/ (repo + API), model/
```

## Rules
- Tokens: encrypted with a Keystore-backed AES-GCM key. Never in plain SharedPreferences or logs.
- Use only the generated/typed API client derived from `docs/api/openapi.yaml`.
- Every screen has loading, empty, error and offline states.
- All user-visible text is in `res/values` and `res/values-hu`. Hungarian is the default for the maintainer.
- Material 3 theming with dark theme support.
- Photo upload uses the Android photo picker (`ActivityResultContracts.PickVisualMedia`).
- Enable network security config: cleartext traffic is disallowed except for local debug builds.
- Release builds: R8 enabled, no debug logging.

## Commands (only when the Android SDK is available, see docs/ENVIRONMENT.md)
- `./gradlew ktlintCheck detekt`
- `./gradlew testDebugUnitTest`
- `./gradlew assembleDebug`
- Until the SDK is available, write the code and record "not verified" in `docs/PROGRESS.md`. Do not claim the app compiles.

## Legal screen
Add an About / Legal screen with copyright, AGPL notice, source link placeholder (`TODO(operator)`) and a link to `TRADEMARKS.md`.
