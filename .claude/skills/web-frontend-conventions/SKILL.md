---
name: web-frontend-conventions
description: Use when building or changing the React web app in apps/web: routing, feature folders, TanStack Query data access, Tailwind styling, i18n keys, forms, loading and error states, PWA setup, accessibility and tests.
---

# Web frontend conventions

## Structure
Feature-first folders under `apps/web/src/features/<name>/` with `pages/`, `components/`, `hooks/` and `queries.ts`. Shared UI in `src/components/ui/`. Routes registered in `src/app/router.tsx`, lazy-loaded per feature.

## Data access
- Query keys are arrays built by factory functions in `queries.ts`, e.g. `['dormitories', dormitoryId, 'menus', date]`.
- Mutations invalidate the narrowest affected keys. Optimistic updates only for rating and recommendation toggles, with rollback.
- API calls go through generated typed client functions only.
- Handle 401 centrally: attempt one refresh, then redirect to login with a return URL.

## Forms
- Controlled inputs with schema validation (zod or valibot) mirroring server rules.
- Show field errors next to fields, link them with `aria-describedby`. Server validation errors map to fields from the error envelope.

## States
Every data view implements four states: loading (skeleton, not spinner-only), empty (explanation and next action), error (retry button, request ID shown), offline (banner, cached data read-only where available).

## i18n
- Keys are semantic (`menu.today.empty`), never the English sentence.
- `hu.json` is the source; `en.json` must have the same key set. A test fails when keys differ.
- Dates and numbers via `Intl` with the active locale.

## Styling and theme
- Tailwind tokens for colors, radii and spacing defined once in `tailwind.config`. Dark mode default via a class on `<html>`; user choice stored in localStorage (non-sensitive).
- Rounded cards, generous spacing, food photos with `aspect-ratio` to avoid layout shift.
- Motion respects `prefers-reduced-motion`.

## Accessibility
- Semantic HTML first, ARIA only when needed. Focus is managed on route change and dialog open or close.
- Contrast WCAG AA in both themes. Touch targets at least 44px.

## PWA
- Web manifest with name, icons (192, 512, maskable), theme color.
- Service worker caches the app shell and static assets only. API responses are not cached by default; cached menu data is shown with a stale indicator when offline.
- Install prompt is user-triggered.

## Tests
- Vitest + Testing Library for components and hooks; MSW for API mocks.
- At least one smoke test per route and tests for the four data states of each feature.
- Accessibility check with `jest-axe` or `vitest-axe` on key pages.
