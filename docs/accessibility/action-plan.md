# Accessibility Action Plan

Rewritten from prose into a file-anchored checklist in Phase 5 of the
frontend remediation plan. Each item below is either **enforced** (ESLint
gates it), **shipped** (a concrete fix already landed), or **tracked**
(known issue with a specific file and line).

When you finish a tracked item, flip the ESLint rule from `off` to `error`
in `web/eslint.config.js` (the "Tracked-but-not-gated" block) if one
applies — that gives us a ratchet against regression.

---

## Enforced by ESLint (`error` in `web/eslint.config.js`)

`eslint-plugin-vuejs-accessibility` is installed and CI runs
`npm run lint --max-warnings 0`. The following rules fail the build:

- `alt-text` — `<img>` must have alt
- `anchor-has-content` — `<a>` must have text/label
- `aria-props` — only valid ARIA props
- `aria-role` — only valid ARIA roles
- `aria-unsupported-elements` — no ARIA on elements that don't support it
- `heading-has-content` — no empty `<h1>`-`<h6>`
- `iframe-has-title` — `<iframe>` must have title
- `no-access-key` — no `accesskey=` (shortcut conflicts with AT)
- `no-autofocus` — no `autofocus` on elements (except PrimeVue Volt
  wrappers which proxy the prop to a library)
- `no-distracting-elements` — no `<marquee>`/`<blink>`
- `no-redundant-roles` — no `role="list"` on `<ul>`, etc.
- `role-has-required-aria-props` — `role="combobox"` et al must have
  the required ARIA props
- `tabindex-no-positive` — no `tabindex` > 0

## Shipped in Phase 5

- [x] `SessionView.vue:319-341` — removed blanket `aria-live="polite"`
      from the progress region (was reading out every WebSocket tick).
      Added a scoped `<span class="sr-only" aria-live="polite" role="status">`
      that announces only meaningful transitions: each completed
      checkpoint, and 25% milestone crossings.
- [x] `useFocusRestore` composable added at
      `web/src/composables/useFocusRestore.ts`. Wired into SessionView
      (instructions panel, console modal) and PodDetailView (snapshot,
      revert, console modals). Captures `document.activeElement` on
      open, restores focus via `queueMicrotask` on close.
- [x] `AppLayout.vue` — the main content region now gets `inert` when
      the mobile sidebar is open, so screen readers and keyboard
      navigation can't reach through the overlay.
- [x] `AppSidebar.vue` — on mobile sidebar open, focus moves to the
      `<aside>` (which now has `tabindex="-1"` and an
      `aria-label="Primary navigation"`). Escape closes.
- [x] `DashboardView.vue` — HTML5 drag-and-drop now has a keyboard
      alternative: when the layout is unlocked, each widget container is
      focusable (`tabindex="0"`) and handles Alt+ArrowUp / Alt+ArrowDown
      to reorder. The new position is announced via the existing
      `announce()` helper.
- [x] Redundant-role cleanup: removed `role="list"` from four `<ul>`
      sites, `role="listitem"` from one `<li>` site, and `role="main"`
      from the `<main>` element in AppLayout.

## Tracked — flip ESLint rule to `error` after fixing

### `form-control-has-label` (16 sites)

The rule can't see through PrimeVue wrappers. Most of these inputs DO
have a proper `<label for="id">` in surrounding markup; the rule just
doesn't recognize PrimeVue's component as a form control. Audit each and
either add an `aria-label` directly on the wrapper or refactor to a
native `<input>` inside a `<label>`:

- `src/views/ProgressDashboardView.vue:374`
- `src/views/SessionsView.vue:222`
- `src/views/SettingsView.vue:180`
- `src/views/UsersView.vue:428`, `:438`, `:632`, `:702`
- `src/views/organizations/AuditLogView.vue:258`, `:268`
- `src/views/organizations/CreateTeamView.vue:95`
- `src/views/teams/TeamView.vue:288`
- `src/components/…` (run `npm run lint -- --rule 'vuejs-accessibility/form-control-has-label: error'` for the full list)

### `click-events-have-key-events` (4 sites)

Non-interactive elements (`<div>`/`<span>`) with `@click` but no
corresponding `@keydown`. Either convert to `<button>` or add Enter/Space
handlers mirroring the click behavior.

- `src/views/PodDetailView.vue:744` — expandable VM row
- `src/views/ProfileView.vue:138` — profile edit row
- `src/views/SettingsView.vue:191`, `:198` — settings toggle cards

### `interactive-supports-focus`

Any element with an interactive ARIA role (`button`, `link`, `menuitem`)
needs to be reachable by keyboard. Usually co-occurs with
`click-events-have-key-events` above — fix both together.

### `mouse-events-have-key-events`

Hover/drag handlers without keyboard equivalents. Usually tooltips and
drag affordances. Audit each site and add `@focus`/`@blur` for tooltip
reveal, or `@keydown.arrow` for drag alternatives.

### Focus restoration for the other nine modal-heavy views

`useFocusRestore` is wired into SessionView and PodDetailView. The
following views also open Dialogs and should get the same treatment:

- `src/views/UsersView.vue`
- `src/views/LabTemplateManagementView.vue`
- `src/views/LabVersionHistoryView.vue`
- `src/views/PathwayInteractiveView.vue`
- `src/views/PathwayDetailView.vue`
- `src/views/teams/TeamView.vue`
- `src/views/organizations/AuditLogView.vue`
- `src/views/ReservationsView.vue`
- `src/views/InstructorDashboardView.vue`

## Manual-audit items (ESLint can't catch these)

- [ ] **Heading level order.** Grep for `<h1>` through `<h6>` across
      views; ensure each page has exactly one `<h1>` and headings step
      down without skipping (no `<h1>` → `<h3>`).
- [ ] **Landmark uniqueness.** Exactly one `<main>` per page, one
      `<header>`, one `<footer>`. Side nav should use `<nav>` with an
      `aria-label` distinct from any other nav.
- [ ] **Color contrast.** All text must meet WCAG AA (4.5:1 for normal
      text, 3:1 for large). Run `@axe-core/cli` against the build in CI
      as a follow-up.
- [ ] **Keyboard-only walkthrough.** End-to-end: login → dashboard →
      start lab → session view → pod console → submit. Every action
      should be reachable without touching the mouse.
- [ ] **Screen-reader walkthrough.** Same path with VoiceOver or NVDA.
      Particular attention to:
      - Dynamic progress updates (the Phase 5 fix should be verified).
      - Modal open/close announcements.
      - Widget reorder announcements.
- [ ] **Extended-time accommodation.** The reservation model currently
      has no hook for granting extended time to a specific student.
      Tracked separately in the educator-review report.
- [ ] **Captioning in instruction Markdown.** If lab authors embed
      video, ensure the lab-template schema requires a captions-url
      companion field.

## How to run the audit locally

```bash
cd web
npm run lint                                # strict: fails on any warning
npm run lint -- --rule 'vuejs-accessibility/form-control-has-label: warn'
                                            # see all label issues as warnings
```

For SR testing: macOS VoiceOver is built in (Cmd+F5). NVDA is free for
Windows and considered the industry-standard test target.
