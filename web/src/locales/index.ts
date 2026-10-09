import { createI18n } from 'vue-i18n'
import en from './en.json'
import es from './es.json'
import { pseudoizeCatalog } from './pseudo'
import { logger } from '@/utils/logger'

// The pseudo-locale (`en-XA`) is a dev/QA diagnostic, not a real
// language. It's only added to the supported set when VITE_I18N_PSEUDO=1
// is set at build/dev time. Prod bundles built without the flag do not
// include it, and Vite tree-shakes the pseudoized catalog.
const PSEUDO_ENABLED = import.meta.env['VITE_I18N_PSEUDO'] === '1'

/**
 * User-facing locales. `en` and `es` are real languages; `en-XA` is a
 * QA-only pseudo-locale (see pseudo.ts). The language switcher reads
 * this list and filters `en-XA` out unless pseudo mode is on.
 */
export const supportedLocales = (
  PSEUDO_ENABLED ? ['en', 'es', 'en-XA'] : ['en', 'es']
) as readonly ('en' | 'es' | 'en-XA')[]
export type SupportedLocale = (typeof supportedLocales)[number]

/**
 * Human-readable labels for the language switcher. Each locale's label is
 * written in its own language (endonym) so users recognize their own
 * tongue regardless of current UI locale.
 */
export const localeLabels: Record<SupportedLocale, string> = {
  en: 'English',
  es: 'Español',
  'en-XA': 'Pseudo (QA)',
}

export const DEFAULT_LOCALE: SupportedLocale = 'en'

/**
 * Locales whose script reads right-to-left. Consumed by the locale store to
 * flip `document.documentElement.dir` and by `.rtl-mirror` CSS utilities. The
 * list is an explicit allow-list (rather than `Intl.Locale.textInfo.direction`)
 * because the API is not universally supported across the browsers we target
 * and the RTL set is small and stable. See ADR-0004.
 *
 * `en-XA` is the LTR pseudo-locale and deliberately excluded. A future
 * `en-XB` pseudo-RTL locale (ADR-0004 Phase B) will be added here.
 */
export const RTL_LOCALES: readonly SupportedLocale[] = [] as const

export function isRtlLocale(value: string | undefined): boolean {
  return !!value && (RTL_LOCALES as readonly string[]).includes(value)
}

export function isSupportedLocale(value: string | undefined): value is SupportedLocale {
  return !!value && (supportedLocales as readonly string[]).includes(value)
}

/**
 * Resolve a locale string (possibly a BCP 47 tag like "en-US") to one of our
 * supported locales, or fall back to the default.
 */
export function resolveLocale(candidate: string | undefined): SupportedLocale {
  if (!candidate) return DEFAULT_LOCALE
  if (isSupportedLocale(candidate)) return candidate
  const base = candidate.split('-')[0]
  if (base && isSupportedLocale(base)) return base
  return DEFAULT_LOCALE
}

// Real catalogs. Pseudoization is only applied when enabled.
const messages: Record<string, typeof en> = { en, es }
if (PSEUDO_ENABLED) {
  messages['en-XA'] = pseudoizeCatalog(en)
}

/**
 * Each unique `${locale}:${key}` pair fires once per session. Prevents a
 * broken view from flooding analytics with thousands of duplicate warnings.
 */
const reportedMisses = new Set<string>()

/**
 * vue-i18n `missing` handler — called whenever a key can't be resolved in the
 * current locale (before fallback). Routes to the shared logger so the log
 * pipeline (Sentry hook, console in dev) picks up catalog gaps. Sampling is
 * per-session in memory; counts reset on reload.
 */
function reportMissingKey(locale: string, key: string): void {
  const fingerprint = `${locale}:${key}`
  if (reportedMisses.has(fingerprint)) return
  reportedMisses.add(fingerprint)
  logger.warn('i18n: missing translation', { module: 'i18n', locale, key })
}

export const i18n = createI18n({
  legacy: false,
  locale: DEFAULT_LOCALE,
  fallbackLocale: DEFAULT_LOCALE,
  globalInjection: true,
  messages,
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV,
  missing: (locale, key) => {
    reportMissingKey(locale, key)
    return key
  },
})
