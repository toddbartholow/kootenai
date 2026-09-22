import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import {
  DEFAULT_LOCALE,
  i18n,
  isRtlLocale,
  resolveLocale,
  supportedLocales,
  type SupportedLocale,
} from '@/locales'
import { api } from '@/api'
import { logger } from '@/utils/logger'

function getBrowserLocale(): SupportedLocale {
  if (typeof navigator === 'undefined') return DEFAULT_LOCALE
  return resolveLocale(navigator.language)
}

/**
 * Persist the user's locale preference on the server. Fire-and-forget:
 * failures log but don't throw, because the UI has already switched locally
 * and the preference is stored in localStorage as a fallback. The pseudo
 * locale `en-XA` is a dev diagnostic and never sent to the server.
 */
async function persistPreference(next: SupportedLocale): Promise<void> {
  if (next === 'en-XA') return
  try {
    await api.put('/auth/me/preferred-locale', { preferredLocale: next })
  } catch (err) {
    // Common: unauthenticated (no token yet) — fine, the setLocale call may
    // have fired during public-page rendering before the user logs in.
    logger.warn('failed to persist preferred locale', { module: 'locale', next, err })
  }
}

export const useLocaleStore = defineStore(
  'locale',
  () => {
    // Initial value: persist plugin overrides this from localStorage if present.
    // Otherwise fall back to the browser preference.
    const locale = ref<SupportedLocale>(getBrowserLocale())

    watch(
      locale,
      next => {
        i18n.global.locale.value = next
        if (typeof document !== 'undefined') {
          document.documentElement.lang = next
          document.documentElement.dir = isRtlLocale(next) ? 'rtl' : 'ltr'
        }
      },
      { immediate: true },
    )

    /**
     * Switch the active locale. Emits the change to vue-i18n via the watcher
     * above, then fires a best-effort PUT to persist on the server. Callers
     * that are hydrating from a server-provided value (e.g. auth store after
     * /auth/me) should pass { persist: false } to avoid echoing back.
     */
    function setLocale(next: SupportedLocale, options: { persist?: boolean } = {}) {
      locale.value = next
      if (options.persist ?? true) {
        void persistPreference(next)
      }
    }

    return {
      locale,
      supportedLocales,
      setLocale,
    }
  },
  {
    persist: {
      key: 'locale',
      paths: ['locale'],
    },
  },
)
