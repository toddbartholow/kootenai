# Pseudo-locale regression sweep

`tests/pseudo-locale.spec.ts` walks a handful of user-facing routes under the
`en-XA` pseudo-locale and fails the build if any visible text contains a run
of ≥4 ASCII letters that isn't wrapped in the pseudo-locale's `⟦ … ⟧`
markers. In practice, that's the strongest cheap signal that a string
bypassed `vue-i18n` and was hardcoded inline.

## Running locally

```bash
cd tests/e2e
npm run test:pseudo
```

This uses `playwright.pseudo.config.ts`, which:

- Auto-starts a vite dev server in `web/` with `VITE_I18N_PSEUDO=1` (enables
  the `en-XA` pseudo catalog) and `VITE_USE_MOCK_DATA=true` (no infra
  dependencies).
- Seeds `localStorage` with mock auth (token + user) and the `en-XA`
  locale via `addInitScript` so the spec can reach authenticated routes
  without going through the login form.

To target a server that is already running (skipping the auto-start), set
`E2E_BASE_URL`:

```bash
E2E_BASE_URL=http://localhost:5173 npm run test:pseudo
```

## CI

Wired into `.github/workflows/ci.yml` as the `web-i18n-pseudo` job. The
job installs web + e2e deps, installs the chromium Playwright browser,
and runs `npm run test:pseudo`. On failure, the Playwright HTML report is
uploaded as the `pseudo-locale-report` artifact.

## What it catches, what it doesn't

Catches: hardcoded English strings in templates, attribute values, or
`createTextVNode` calls that never reach vue-i18n.

Does **not** catch:
- Layout overflow (pseudo strings are ~30% longer than English, which is
  realistic for `es`/`de`). A follow-up spec can compare bounding boxes
  between `en` and `en-XA` to flag truncation.
- Strings inside elements marked with `data-pseudo-skip` (see below).

## The `data-pseudo-skip` escape hatch

Per the i18n rollout plan, mock/demo data (lab names, achievement names,
leaderboard rows, organization names, etc.) is intentionally not
translated — it's content, not UI chrome. The sweep ignores text inside
any element (or descendant) carrying the `data-pseudo-skip` attribute.

Add it to:
- Containers that render API/mock data (the immediate parent of a
  `v-for` over an API result list works well).
- Locale endonyms in the `LanguageSwitcher` (the pseudo locale's own
  endonym is intentionally always-English).

Don't add it to UI chrome — the gate is supposed to catch leaks there.

## Tuning the allow-list

The spec's `ALLOW_LIST` enumerates bare English tokens that are legitimate
(brand names, HTML tag names surfaced via `textContent` on fallback
containers). Add sparingly — every entry is a blind spot, and it's usually
better to rename the element, wrap the text in `t()`, or use
`data-pseudo-skip` than to extend the list.
