# apps/web – React web app rules

## Stack
React, TypeScript (strict), Vite, Tailwind CSS, React Router, TanStack Query, i18next (or equivalent), accessible custom components or a maintained headless library (Radix UI or React Aria). PWA via `vite-plugin-pwa`.

## Layout
```
src/app/          providers, router, layout shell
src/features/     one folder per feature (auth, menus, ratings, dormitories, admin ...)
src/api/          generated client from docs/api/openapi.yaml (do not hand-edit)
src/components/ui/  design primitives
src/i18n/         hu.json (default), en.json
src/lib/          utilities, no UI
```

## Rules
- Use the generated API client only. No hand-written `fetch` to `/api/v1` outside `src/api/`.
- Server state lives in TanStack Query. No duplicated server state in global stores.
- Auth uses HttpOnly cookies. Never read, store or log tokens in JavaScript. Send the CSRF header on state-changing requests.
- Every route has loading, empty, error and offline states (SPEC §7).
- No user-facing string in TSX without an i18n key. Hungarian is the default locale.
- Dark mode default, optional light mode, respects `prefers-color-scheme` on first visit.
- Mobile-first layouts. Test at 360px width.
- Images: use `loading="lazy"`, `srcset` from the API, and never embed external tracking.
- No fake data, no hardcoded production menus. Use MSW only in tests.

## Commands
- `npm run typecheck`
- `npm run lint`
- `npm run test` (Vitest + Testing Library)
- `npm run build`
- Record each result in `docs/PROGRESS.md`. If `node_modules` cannot be installed, record "not verified".

## Accessibility
- Keyboard navigation for every interactive element.
- Visible focus states, WCAG AA contrast in both themes.
- Use the `frontend-design` and `web-frontend-conventions` skills for UI work.
