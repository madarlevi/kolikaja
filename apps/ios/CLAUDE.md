# apps/ios – native iOS rules

## Stack
Swift (latest stable), SwiftUI, URLSession with async/await, Swift Concurrency (actors where state is shared), Keychain for tokens, `PhotosPicker` for photo selection, `Observation` or `ObservableObject` view models (decided in ARCHITECTURE.md). Localization via `Localizable.xcstrings` (Hungarian and English).

## Layout
```
KoliKaja/App/           app entry, environment, dependency container
KoliKaja/Core/Network/  URLSession client, endpoints, error mapping, generated models from docs/api/openapi.yaml
KoliKaja/Core/Auth/     Keychain token store, session manager
KoliKaja/Features/<Name>/  Views, ViewModels, Services
KoliKaja/Resources/     Localizable.xcstrings, assets
KoliKajaTests/
```

## Rules
- Tokens only in Keychain, with `kSecAttrAccessibleWhenUnlockedThisDeviceOnly` or stricter. Never in UserDefaults.
- No hardcoded user-facing strings. Use `String(localized:)` with keys.
- Every screen: loading, empty, error and offline states.
- ATS stays enabled (no `NSAllowsArbitraryLoads`).
- Respect Dynamic Type, VoiceOver labels, Reduce Motion, and dark mode.
- Native navigation (`NavigationStack`), native sheets, native `PhotosPicker` and camera via `UIImagePickerController` wrapper if camera upload is included.
- Photo upload: resize and strip EXIF before sending.

## Commands (only on macOS with Xcode, see docs/ENVIRONMENT.md)
- `xcodebuild -scheme KoliKaja -destination 'generic/platform=iOS Simulator' build`
- `xcodebuild test -scheme KoliKaja -destination 'platform=iOS Simulator,name=iPhone 15'`
- Until macOS/Xcode is available, write the code, keep it syntactically careful, and record "not verified".
- Do not claim the app compiles without running the build.

## Legal screen
About / Legal screen with copyright, AGPL notice, source link placeholder (`TODO(operator)`) and a link to `TRADEMARKS.md`.
