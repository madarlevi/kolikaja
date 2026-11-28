# KoliKaja – UI direction

Status: direction only (2026-11-26). No UI code exists yet; the web app is built in phase 4, the mobile apps
in madarlevi/kolikaja-android and madarlevi/kolikaja-ios. This document applies to all three clients; the
mobile apps follow their platform conventions (Material 3, SwiftUI) and share the information architecture,
the rating control, the wording and the colour tokens, not the component code.

## 1. Principles
1. **Today first.** The home view is today's menu of the selected dormitory. Everything else is one step away.
2. **Photo first.** Meals are shown as poster-style cards: the photo is the largest element, metadata below it.
3. **One tap to rate.** Tapping a value of the 1–5 control saves the rating. No confirm dialog, undo instead.
4. **Numbers you can trust.** Every meal shows the community average together with the number of ratings.
   An average without its count is never shown.
5. **Calm and dense.** Clear hierarchy, generous spacing inside cards, many meals visible at once.
6. **Dark by default**, light optional, both meeting WCAG AA.

**Reference.** The layout clarity of trakt.tv – poster grids, metadata at a glance, the rating visible on every
item, few top-level sections – adapted for food. Only these layout ideas are borrowed. No name, logo,
colour scheme, icon, illustration, copy or other asset of trakt.tv is used, and KoliKaja does not suggest
any affiliation.

## 2. Information architecture

### Top-level sections (mobile: bottom navigation; web: top navigation, bottom navigation below 640 px)
| # | Section (hu / en) | Content |
|---|-------------------|---------|
| 1 | **Ma** / Today | Home. Today's menus of the current dormitory, meal period switcher, announcements banner. |
| 2 | **Hét** / Week | ISO week view, day selector, compact meal cards; the photo gallery of the dormitory is reached from here. |
| 3 | **Értesítések** / Notifications | In-app notifications with unread badge. |
| 4 | **Profil** / Profile | My dormitories (switch, join, invitation code), my ratings, settings, sessions, data export, account deletion, About / Legal. Moderators and administrators also find the dormitory administration here. |

- **Dormitory switcher** in the top bar on every section (dormitory name + chevron). Users without a
  dormitory land on onboarding: search public dormitories or enter an invitation code.
- **Meal detail** is pushed from any card: photos, rating control, community statistics, recommendation,
  allergens with the verification notice, comments, report actions.
- **Administration** (web first): dormitory administration (menus, categories, members, invitations,
  moderation queue, announcements, API keys, audit log) and global administration are separate areas reached
  from the profile menu, never mixed into the member views.

### SPEC §7 pages → sections
Home / Today's menu, Meal details, Ratings and comments → *Ma* and meal detail · Weekly menu, Photo gallery →
*Hét* · Notifications → *Értesítések* · Dormitory discovery, My dormitories, User profile, Settings,
Authentication → *Profil* (authentication before entry) · Dormitory and global administration → admin areas.

## 3. Key screens

**Today (Ma).** Top bar: dormitory switcher, date in the dormitory's time zone ("nov. 26., csütörtök"),
segmented control for meal periods when there is more than one. Body: meal poster cards grouped by category
(dormitory-defined, e.g. "Leves", "A menü"), two columns on phones, 3–6 on wider screens. Important
announcements appear as one dismissible banner above the cards.

**Week (Hét).** Seven-day selector (H K Sze Cs P Szo V) with today marked; per day a compact list of
cards (smaller photo, same rating summary). Days without a published menu show an empty state.

**Meal detail.** Photo carousel (swipe, "photo by" credit), name, category, menu date; the rating control;
community statistics (average, count, distribution, trusted reviewers' average when available,
recommendation counts); recommend / not recommend toggle; allergen list with the notice that allergen data
comes from the dormitory and must be verified with the food provider; comments with pagination; add photo.

## 4. Components

### 4.1 Meal poster card
- Photo area with a **3:4** aspect ratio (portrait poster), `object-fit: cover`, lazy loaded, fixed size to
  avoid layout shift. A bottom gradient scrim carries the meal name (max. 2 lines) in high contrast.
- Below the photo: **community average and count** ("★ 4,2 · 37"), the recommendation share when at least
  3 people voted ("👍 82 %" rendered as an icon plus text), and the user's own rating as a small badge
  ("Te: 4") when present.
- Without a photo: a neutral placeholder in the surface colour with the category icon and the meal name.
  Never stock photography (no fake data, SPEC §7).
- The whole card is one link to the meal detail (one focus stop). Rating happens in the detail view or in
  the rating sheet opened by the card's secondary "Értékelés" button, because five 44 px targets do not fit
  on a phone-width card.

### 4.2 Rating control (1–5)
- A row of five segments, each at least **44 × 44 px** (Android 48 dp). Each shows its number and a
  colour from tokens `rating-1` … `rating-5`; colour is never the only signal.
- **One tap saves** (optimistic update, undo snackbar for 5 s). Tapping the selected value again does nothing;
  "Értékelés törlése" removes the rating.
- The selected value's label is shown in full below the row, and every segment carries its full label for
  screen readers. Semantics: radio group (web `role="radiogroup"`, Compose `selectableGroup`,
  SwiftUI accessibility representation as a picker). Arrow keys move, Space/Enter selects.
- Exact labels (from docs/DECISIONS.md, via i18n resources only):

  | Value | Label (hu) |
  |-------|------------|
  | 1 | Le se menj |
  | 2 | Ha nem vagy éhenhalás szélén, hagyd |
  | 3 | Ehető, ha nincs más |
  | 4 | Amúgy eskü nem olyan szar |
  | 5 | Már határozottan ehetőnek nevezhető |

  English resources translate the labels for English-language users; the Hungarian text above is the source.
- When rating is not open (`rating_open = false`), the control is disabled and the reason is shown
  ("A tálalás napjától lehet értékelni." / "Az értékelési idő lejárt.").

### 4.3 Community statistics
- Average with one decimal in the active locale ("4,2" in Hungarian, "4.2" in English) and the count
  ("37 értékelés"). With no ratings: "Még nincs értékelés".
- Detail view: horizontal distribution bars for 1–5 with counts, trusted reviewers' average when the API
  returns it, recommended / not recommended counts. Individual ratings of other users are never shown.

### 4.4 Other building blocks
Buttons (primary, secondary, text), chips (category, allergen), segmented control, bottom sheet / dialog,
snackbar with undo, banner (announcement, offline), skeleton loaders shaped like the final content,
avatar with initials fallback, empty-state block (icon, sentence, next action), error block (message,
"Újra" button, request ID in small text).

## 5. Visual language

### 5.1 Colour tokens
Defined once (Tailwind theme / Compose `ColorScheme` / SwiftUI asset catalog). Contrast ratios below were
computed with the WCAG 2.x formula against `bg`, `surface` and `raised`; the lowest value is shown.

| Token | Dark (default) | min. contrast | Light | min. contrast |
|-------|----------------|---------------|-------|---------------|
| `bg` | `#0E1116` | – | `#FAFAF7` | – |
| `surface` (cards) | `#171B22` | – | `#FFFFFF` | – |
| `raised` (sheets, menus) | `#1F2530` | – | `#F1F2F4` | – |
| `text` | `#F2F4F7` | 13.96 | `#12161C` | 16.20 |
| `text-muted` | `#A3ACBA` | 6.71 | `#525B69` | 6.13 |
| `accent` (links, primary) | `#FF8A3D` | 6.56 | `#B93D0B` | 5.02 |
| `on-accent` (text on accent) | `#0E1116` | 8.06 on accent | `#FFFFFF` | 5.62 on accent |
| `focus` (focus ring) | `#7AB8FF` | 7.42 | `#1D5FD1` | 5.19 |
| `border-strong` (input borders, controls) | `#6B7587` | 3.31 | `#7A8494` | 3.37 |
| `rating-1` | `#F47C7C` | 5.87 | `#B3261E` | 5.84 |
| `rating-2` | `#F59E5B` | 7.27 | `#B4470F` | 4.87 |
| `rating-3` | `#E8C547` | 9.16 | `#7A6000` | 5.36 |
| `rating-4` | `#9BD36A` | 8.73 | `#356E1B` | 5.52 |
| `rating-5` | `#4CC38A` | 6.94 | `#1B6E45` | 5.57 |

All text tokens reach 4.5:1 and all control tokens 3:1 on every surface in both themes. Decorative borders
may be lower but must never be the only boundary of an interactive control. Photo scrims guarantee 4.5:1 for
the title text over any image (gradient to 80 % `bg`).

### 5.2 Typography, shape, motion
- Web: Inter, self-hosted (SIL Open Font License 1.1, listed in THIRD_PARTY_LICENSES in phase 7); no font
  CDN, no external requests. Android: Roboto via Material 3 type scale. iOS: SF Pro with Dynamic Type.
- Scale (web): 12 / 14 / 16 (body) / 20 / 24 / 32 px; line height 1.4–1.5; numbers in statistics use tabular figures.
- Radius: cards 16 px, controls 12 px, chips full. Spacing on a 4 px grid. Dark mode uses surface tones
  instead of shadows for elevation.
- Motion 150–250 ms ease-out for state changes; no motion when the user prefers reduced motion.
- Icons: Lucide (ISC licence) on the web, Material Symbols on Android, SF Symbols on iOS (licences recorded
  in phase 7).

## 6. Component library (T-30, to confirm by the maintainer)
**Tailwind CSS + Radix UI primitives, composed in the shadcn/ui style** (components copied into
`apps/web/src/components/ui/` and owned by the project, not installed as a black-box library).
Reasons: Radix primitives provide the hard accessibility parts (focus management, keyboard interaction, ARIA
for dialogs, menus, radio groups, tabs, toasts); Tailwind keeps styling in design tokens with no runtime
CSS-in-JS; owning the component code avoids a heavy dependency and lets the rating control and poster cards
be built exactly as specified; MIT licences are compatible with AGPL-3.0; small bundles suit low-end phones.
Alternatives considered: React Aria Components (equally strong accessibility; a good fallback if Radix gaps
appear), MUI (heavy, Material look on the web), Chakra UI (runtime styling), Headless UI (fewer primitives).
Mobile apps do not share component code; they use Material 3 Compose and SwiftUI with the same tokens.

## 7. Accessibility baseline (all clients)
- WCAG 2.2 AA contrast in both themes (text 4.5:1, large text 3:1, controls and focus indicators 3:1);
  the tokens above satisfy this.
- Touch targets at least 44 × 44 px (iOS 44 pt, Android 48 dp), including the rating segments.
- Full keyboard navigation on the web, logical focus order, focus moved to the page heading on route change
  and into dialogs on open, returned on close.
- Visible focus: 2 px `focus` outline with 2 px offset on every interactive element; never removed.
- Information is never conveyed by colour alone (rating numbers, icons with text).
- Screen readers: semantic HTML first; labelled controls; the rating control announces value and label;
  images have alternative text ("Fotó: <étel neve>, <feltöltő>").
- Text resizes to 200 % (web) and Dynamic Type / font scale (mobile) without loss of content.
- `lang` is set to `hu` or `en`; dates and numbers use the active locale.
- Every data view has loading (skeleton), empty, error (retry, request ID) and offline (banner, stale data)
  states.

## 8. Not decided yet
Logo, app icon and illustrations (they depend on the trademark policy written in phase 7); whether the web
app offers a compact list view in addition to the poster grid.
