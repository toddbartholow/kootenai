# ADR 0001: Internationalization & Localization Strategy

- **Status**: Accepted
- **Date**: 2026-04-14
- **Deciders**: Kootenai maintainers

## Context

Kootenai ships with zero internationalization infrastructure today. Every user-facing string is hardcoded English — Vue templates, Go API error responses, validator messages, date formats. We want to stop adding more hardcoded strings we will later have to rip out, and we want a well-defined pattern for new code.

This ADR records the first-pass decisions. A pilot slice (login flow + shared formatters) is migrated in the same branch that adds this ADR so the plumbing is exercised end-to-end.

## Decisions

### Frontend: `vue-i18n@9`

- Composition API mode (`legacy: false`).
- Catalogs live at `web/src/locales/<locale>.json` (JSON — simplest tooling, IDE-friendly, no extra parser dep).
- Factory at `web/src/locales/index.ts` exports `createI18n()` and the typed `supportedLocales` const.
- A persisted Pinia store `web/src/stores/locale.ts` owns the current locale. An axios request interceptor in `web/src/api/config.ts` sets `Accept-Language: <locale>` on every request.
- PrimeVue locale bundles are wired alongside vue-i18n. Only `en` ships in this branch; adding a locale is a one-line import.

### Backend: `github.com/nicksnyder/go-i18n/v2`

- Catalogs live at `api/internal/i18n/translations/active.<locale>.toml`, embedded via `//go:embed` so the single-binary deploy story is unaffected.
- A global `Bundle` is constructed at startup (`api/internal/i18n/bundle.go`).
- A request-scoped `*i18n.Localizer` is attached to `context.Context` by `api/internal/middleware/locale.go`.
- Handlers resolve messages via `httputil.LocalizedErrorResponse(ctx, w, status, msgID, templateData)`.

### Locale negotiation

Precedence, highest first:

1. Authenticated user's `preferred_locale` (stubbed; schema change is deferred).
2. `Accept-Language` header (parsed via `golang.org/x/text/language.Matcher` on the backend; read from `navigator.language` on the frontend).
3. Default: `en`.

`Accept-Language` is the single wire contract — no custom header — so curl, CLI tests, and other non-browser clients get localization for free.

### Message key naming

Dotted, domain-first. Lowercase, camelCase terminal segment.

- `auth.errors.invalidCredentials`
- `validation.required`, `validation.email`, `validation.minLength`
- `format.duration.hours`, `format.duration.minutes`
- `difficulty.beginner`, `platform.proxmox`, `visibility.global`
- `login.email.label`, `login.submit.idle`, `login.submit.pending`

Named template parameters over positional: `{{.Field}}`, `{field}`.

### What is NOT localized

- Operator logs (slog). Logs are for humans reading production; they stay English for grep-ability.
- Internal errors wrapped via `fmt.Errorf("context: %w", err)` — those are developer context, not user messages.
- Identifiers, URL slugs, database enum values.
- Email templates and PDF certificates (deferred to follow-up).

### Fallback policy

- Every message key MUST exist in the `en` catalog. CI should fail if a key is referenced in code but missing from `en`.
- Missing keys in non-default locales fall through to `en`. In dev, missing-key warnings are logged to the browser console / server logs; in production they are silent (but still tracked via the library's missing-key hook).

### Developer workflow

**Adding a user-facing string**

1. Add the key to `web/src/locales/en.json` (or `api/internal/i18n/translations/active.en.toml`).
2. Use `t('namespace.key', { param })` in a `.vue` file or `LocalizedErrorResponse(ctx, w, status, "namespace.key", map[string]any{"Param": value})` in a handler.
3. Commit.

**Adding a new locale**

1. Copy `en.json` → `<locale>.json` (and `active.en.toml` → `active.<locale>.toml`).
2. Register the locale in `web/src/locales/index.ts` `supportedLocales`.
3. Import the matching PrimeVue locale bundle.
4. Translate. Dev warnings will surface any keys you missed.

## Consequences

### Positive

- New code has an obvious pattern. Reviewers can reject hardcoded strings in PRs with a one-line comment.
- A second locale can ship without touching application code.
- Operator-facing output (logs, debug metadata) stays stable and grep-able.
- Single `Accept-Language` contract works for the browser and non-browser clients uniformly.

### Negative / costs

- Every new user-facing string requires a catalog edit alongside the code edit.
- Until the mass migration is done, the codebase has a mix of `t()`-routed and hardcoded strings. A `// TODO(i18n):` comment is left at each known un-migrated call site to make them grep-able.
- Build size grows modestly (vue-i18n + an extra JSON catalog per locale).

## Pilot migration (this branch)

- Frontend: `LoginView.vue`, `format.ts` (via new `useFormatters` composable), enum labels in `formOptions.ts` + `status.ts::getDifficultyLabel`, date call sites in `ReservationCalendar.vue` and `CertificateModal.vue`.
- Backend: `httputil/responder.go` `ErrMsg*` constants, `auth_handlers.go` hardcoded messages, `validation.go::formatErrorMessage` validator tags.

End-to-end smoke: browser → axios `Accept-Language` → chi middleware → handler returns localized message → Vue renders via `t()`.

## Follow-up work

Tracked as separate issues; all explicitly out of scope for this branch.

- [ ] Mass string extraction across remaining `.vue` components (one issue per top-level area: dashboard, labs, instructor, admin, pathways, reservations, settings, topology).
- [ ] Ship a second locale (pick `es` or `de`; needs translator in the loop).
- [ ] Certificate PDF localization (`api/internal/certificate/service.go` — `"January 2, 2006"` and hardcoded copy).
- [ ] Email template localization (`api/internal/email/`).
- [ ] Python `tools/labtest` CLI — only if we decide operator CLI should be localized (currently: no).
- [ ] Build-time message compilation via `@intlify/unplugin-vue-i18n`.
- [ ] Schema change: add `preferred_locale` column on users + profile API + settings UI.
- [ ] RTL CSS / locale-specific layout.
- [ ] Currency formatting.
- [ ] CI check: missing-key detection (fail build if `t('foo.bar')` is used but `foo.bar` is absent from `en.json`).
