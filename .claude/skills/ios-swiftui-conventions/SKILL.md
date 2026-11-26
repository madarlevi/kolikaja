---
name: ios-swiftui-conventions
description: Use when building or changing the native iOS app in apps/ios (Swift, SwiftUI): architecture, Swift Concurrency networking with URLSession, Keychain token storage, navigation, localization, accessibility, photo upload and tests.
---

# iOS (Swift / SwiftUI) conventions

## Architecture
- Views are thin. View models (`@Observable` on iOS 17+, or `ObservableObject` if the minimum target requires it; decide in ARCHITECTURE.md) own state and call services.
- Services: protocols with a concrete `URLSession`-based implementation and a mock for tests. Dependencies injected through an environment container.
- Feature folders under `Features/<Name>/` with `Views`, `ViewModel`, `Models`.

## Networking
- `URLSession` with `async/await`. One `APIClient` actor-isolated where it holds mutable state (token refresh).
- Token refresh: single in-flight refresh task shared by concurrent requests.
- Decode generated models from `docs/api/openapi.yaml`. Dates decoded with an ISO 8601 strategy that accepts fractional seconds.
- Timeouts: request 30s, resource 120s for uploads.
- Errors map to domain cases: offline, unauthorized, forbidden, notFound, validation, server. Each has a localized message.

## Token storage
- Keychain Services generic password items with `kSecAttrAccessibleWhenUnlockedThisDeviceOnly` or stricter.
- Never in `UserDefaults`, plist files, or logs. Clear on logout.

## Navigation and UI
- `NavigationStack` with typed paths; `TabView` for the main sections; sheets for create and edit.
- Dynamic Type, VoiceOver labels, Reduce Motion respected, dark mode supported.
- Localization: `Localizable.xcstrings` with English and Hungarian. No hardcoded strings.
- Every screen: loading, empty, error (with retry), offline states.

## Photos and camera
- `PhotosPicker` for library selection. Camera capture via a `UIViewControllerRepresentable` wrapper if included; request camera permission with a usage description in Hungarian and English.
- Strip EXIF and downscale before upload; never upload original full-resolution files by default.

## Security settings
- App Transport Security enabled. No `NSAllowsArbitraryLoads`.
- Entitlements minimal. No background modes unless needed.

## Build and quality (once macOS/Xcode is available)
- `xcodebuild -scheme KoliKaja -destination 'generic/platform=iOS Simulator' build`
- `xcodebuild test -scheme KoliKaja -destination 'platform=iOS Simulator,name=iPhone 15'`
- SwiftLint if installed. Until macOS is available, write the code and record "not verified". Do not claim it compiles.
