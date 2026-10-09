# ADR 0004: RTL Infrastructure for Right-to-Left Locales

- **Status**: Accepted
- **Date**: 2026-04-16
- **Deciders**: Kootenai maintainers
- **Extends**: ADR-0001 (i18n strategy), ADR-0003 (rollout order)
- **Tracking issue**: #120

## Context

ADR-0001 and ADR-0003 committed the project to progressive locale rollout and named Arabic (`ar`) as a future locale. Neither ADR resolved *how* the frontend would support right-to-left (RTL) scripts. The current codebase is implicitly left-to-right (LTR): 188 Tailwind directional utility occurrences (`ml-*`, `mr-*`, `pl-*`, `pr-*`, `left-*`, `right-*`) are spread across 63 files in `web/src/`. Chevrons, back-arrows, and progress indicators use fixed-direction glyphs. There is no `dir` handling on `<html>`.

Shipping an RTL locale (Arabic, Hebrew, Persian, Urdu) on top of this state would produce a visibly broken UI: padding on the wrong side, arrows pointing the wrong way, flex rows reversed mid-page, and charts with LTR-locked axes. Worse, fixing it *after* shipping means every subsequent frontend PR becomes a potential regression — the cost only grows.

This ADR records the direction policy and the enabling infrastructure so that adding `ar` later is a matter of populating a catalog plus a smoke-test pass, not a six-week migration.

**Scope note:** This ADR does not commit the project to shipping any specific RTL locale. It commits the project to the *direction-neutral authoring policy* that makes shipping one feasible.

## Decisions

### 1. Tailwind logical properties are mandatory for new code

All direction-aware Tailwind utilities in new code MUST use the logical-property variants:

| Physical (blocked) | Logical (required) |
| --- | --- |
| `ml-*` / `mr-*` | `ms-*` / `me-*` |
| `pl-*` / `pr-*` | `ps-*` / `pe-*` |
| `left-*` / `right-*` | `start-*` / `end-*` |
| `border-l-*` / `border-r-*` | `border-s-*` / `border-e-*` |
| `rounded-l-*` / `rounded-r-*` | `rounded-s-*` / `rounded-e-*` |
| `text-left` / `text-right` | `text-start` / `text-end` |

Logical utilities resolve against the active `dir` attribute at render time: under `dir="ltr"` they compile to physical left/right (identical bytes to today's output); under `dir="rtl"` they mirror automatically. There is no runtime cost and no visual change for existing LTR users.

**Enforcement:** An ESLint `no-restricted-syntax` rule in `web/eslint.config.*` blocks the physical variants in `.vue` and `.ts` files under `web/src/`. This mirrors the existing zero-arg-`.toLocaleString()` rule (ADR-0003). **The rule lands in Phase B, after the existing 188 occurrences are migrated.** `web` CI runs with `--max-warnings 0`, so shipping the rule before the migration would break CI; the phase split below reflects this.

**Exceptions:**
- Generated code (Docusaurus build output, vendored CSS) — excluded from the rule.
- `data-pseudo-skip` regions where physical direction is semantically correct (e.g. a hardware topology diagram that renders a physical cable plugged into the *left* port) — opted out via an `eslint-disable-next-line` with a comment explaining why.

### 2. Existing physical utilities migrate incrementally

The 188 existing occurrences are not a blocker. They migrate on the same schedule as ongoing i18n work: any file touched for another reason flips its physical utilities to logical in the same PR. Once the ESLint rule lands, new physical utilities cannot be introduced, so the count is strictly monotonic-decreasing. A final cleanup sweep closes the remaining occurrences before any RTL locale ships.

### 3. `dir` attribute is set by the locale store

`web/src/stores/locale.ts` already watches `locale` and updates `document.documentElement.lang`. The watcher extends to also set `document.documentElement.dir`:

```ts
const RTL_LOCALES: readonly SupportedLocale[] = ['ar', 'he', 'fa', 'ur'] as const
// inside the locale watcher:
document.documentElement.dir = RTL_LOCALES.includes(next) ? 'rtl' : 'ltr'
```

Rationale for a static allow-list over `Intl.Locale(...).textInfo.direction`: `textInfo` is not yet universally supported (Safari 16.3+), the RTL locale set is small and stable, and explicit is easier to audit. Revisit when the set grows past ~8.

### 4. Direction-aware iconography uses a single CSS class

A utility class — `.rtl-mirror { html[dir="rtl"] & { transform: scaleX(-1); } }` — is applied to the handful of glyphs whose meaning depends on reading direction: chevrons in breadcrumbs, back/forward arrows, step-progress arrows, right-arrow "go" icons in list rows.

Glyphs that are **not** direction-dependent — search icons, gear icons, user avatars, checkmarks, warning triangles, language-selector globe — are never mirrored. Misapplying `.rtl-mirror` to a magnifying-glass or a checkmark is a regression, not an oversight; PR review gates on this.

No JavaScript. No per-component logic. One class.

### 5. Pseudo-RTL locale (`en-XB`) for regression testing

An `en-XB` pseudo-locale extends the existing `en-XA` pseudo-locale (ADR-0003, `web/src/locales/pseudo.ts`). `en-XB` renders the English catalog in ASCII (no character transforms) but forces `dir="rtl"`. This exposes every layout bug an RTL locale would hit, without requiring a translated catalog or a native speaker.

The existing Playwright pseudo-locale sweep (`tests/e2e/tests/pseudo-locale.spec.ts`) gains a second run configured for `en-XB`. The sweep asserts, in addition to its existing English-leak check:

1. No horizontal scroll on the 10 sweep routes at 1280×800.
2. The first inline text node on each page is positioned to the right of the viewport midline (crude but effective check that `dir="rtl"` took effect).
3. Back/forward arrows in `.rtl-mirror` regions have `transform: scaleX(-1)` computed style.

`en-XB` is never exposed to users and never added to `supportedLocales`.

### 6. Chart and data-viz library policy

- **Chart.js** supports RTL natively via `options.locale` plus logical-axis configuration. Wrappers in `web/src/composables/useChart.ts` (or equivalent) pass the active locale through.
- **vis-network** (lab topology diagrams) has no semantic direction — nodes and edges are spatial, not directional. Only the *labels* need to be RTL-readable, which falls out of the browser's native text rendering once `dir="rtl"` is set on an ancestor. No library configuration required. Verified via pseudo-RTL sweep.
- **Any new chart library** must be evaluated for RTL support before adoption. This joins the existing evaluation checklist (license, bundle size, accessibility).

### 7. Rollout phases

RTL infrastructure ships in three phases, each independently mergeable:

1. **Phase A — Enablers.** `.rtl-mirror` CSS class defined; `RTL_LOCALES` allow-list + `isRtlLocale()` helper exported from `web/src/locales/`; `dir` watcher extended in `stores/locale.ts`. No code migration, no ESLint rule. Zero CI risk. Puts the scaffolding in place so Phase B can land the rule without churn-and-rule arriving in the same PR.
2. **Phase B — Migration + enforcement + pseudo-RTL gate.** Existing 188 physical utilities migrated to logical variants (codemod + manual review). ESLint `no-restricted-syntax` rule lands at `error` level once the tree is clean. `en-XB` pseudo-locale implemented; Playwright sweep configured; pseudo-RTL becomes a blocking CI check. No user-visible change.
3. **Phase C — First RTL locale.** `ar` (or `he`) added to `supportedLocales`, catalog drafted, SME review, target-locale smoke test. User-visible.

Phases A and B are prerequisites for C. C is not a prerequisite for A and B — A and B are valuable *even if we never ship an RTL locale*, because they prevent direction drift and give us a cheap regression gate.

## Consequences

### Positive

- Frontend PRs cannot silently regress RTL-readiness once Phase A lands; the cost of adding Arabic drops from "project" to "feature."
- Logical properties are a zero-cost substitution for new code — existing Tailwind compilation produces identical bytes under LTR.
- The pseudo-RTL sweep catches direction bugs without needing a translated catalog or a native-speaker reviewer.
- `.rtl-mirror` centralizes the mirroring policy; reviewers have one class to audit rather than scattered CSS transforms.

### Negative

- The 188 existing physical utilities accumulate as technical debt until migrated. The incremental-migration policy is slower than a big-bang codemod but avoids a 63-file review PR that no one wants to audit.
- `textInfo.direction` would be a cleaner RTL check than a static allow-list once Safari catches up; we will carry the allow-list for ~2 years.
- Pseudo-RTL catches layout bugs but not typographic bugs specific to Arabic (ligatures, kashida, context-sensitive forms). Target-locale smoke test is still required before shipping Arabic.
- Phase B's "first inline text node is right of midline" heuristic is an approximation. A visual-regression snapshot suite would be strictly better but costs more to maintain; defer until we have a second visible reason to add one.

### Neutral

- The RTL locale list (`ar`, `he`, `fa`, `ur`) is not exhaustive. Yiddish, Dhivehi, and others exist; they can be added to the allow-list when there is demand. No architectural change required.
- This ADR does not specify Arabic-first or Hebrew-first for the initial RTL rollout. That call belongs with ADR-0003's rollout sequencing, informed by customer demand at the time.

## References

- ADR-0001: i18n/l10n strategy
- ADR-0003: i18n glossary and rollout (pseudo-locale pattern)
- [Tailwind logical properties (v3.3+)](https://tailwindcss.com/docs/margin#using-logical-properties) — `ms-*`, `me-*`, `ps-*`, `pe-*`, `start-*`, `end-*`
- [MDN: CSS logical properties](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_logical_properties_and_values)
- [MDN: `Intl.Locale.prototype.getTextInfo`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Locale/getTextInfo) — browser-support context for §3
- `web/src/locales/pseudo.ts` — existing `en-XA` implementation, template for `en-XB`
- `tests/e2e/tests/pseudo-locale.spec.ts` — existing sweep, extended in Phase B
- Issue #120 — tracking implementation of phases A, B, C
