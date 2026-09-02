import { test, expect, Page } from '@playwright/test'

/**
 * Pseudo-locale regression sweep.
 *
 * Requires a dev server built with VITE_I18N_PSEUDO=1 and the locale set to
 * en-XA (seeded via localStorage in beforeEach). See
 * tests/e2e/PSEUDO_LOCALE.md for setup.
 *
 * What this catches:
 *   - Hardcoded English strings that bypass vue-i18n entirely. Real en-XA
 *     output surrounds every catalog string with ⟦ … ⟧, so any run of ≥4
 *     ASCII letters NOT enclosed in brackets is an unmigrated literal.
 *
 * What this does NOT catch (yet):
 *   - Layout overflow from longer translations. A follow-up can diff bounding
 *     boxes between en and en-XA for the same viewport.
 */
const ROUTES_TO_SWEEP = [
  '/login',
  '/dashboard',
  '/labs',
  '/pathways',
  '/achievements',
  '/leaderboard',
  '/settings',
  '/pods/create',
  '/progress',
  '/reservations',
]

const PSEUDO_BRACKETED = /⟦[^⟧]*⟧/g
const BARE_ENGLISH = /[A-Za-z]{4,}/g

const ALLOW_LIST = new Set<string>([
  'Virtual',
  'Lab',
  'Kootenai',
  'Chrome',
  'Mozilla',
  'WebKit',
  'iframe',
  'html',
  'body',
  'main',
])

const MOCK_USER = {
  id: '00000000-0000-0000-0000-000000000001',
  email: 'demo@example.com',
  name: 'Demo User',
  roles: ['student', 'instructor'],
}

async function collectBareEnglish(page: Page): Promise<string[]> {
  const visibleText = await page.evaluate(() => {
    // Elements (and their descendants) marked with [data-pseudo-skip] are
    // ignored by the sweep. Use this for surfaces that intentionally render
    // raw API/mock data (lab/pathway/achievement names, leaderboard rows,
    // org names) — per the i18n rollout plan, mock/demo content is not
    // translated. Also covers locale endonyms in the LanguageSwitcher
    // (e.g. "Pseudo (QA)") which are intentionally always-English.
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    const out: string[] = []
    let node = walker.nextNode()
    while (node) {
      const el = node.parentElement
      if (el && el.offsetParent !== null && !el.closest('[data-pseudo-skip]')) {
        const text = node.textContent?.trim()
        if (text) out.push(text)
      }
      node = walker.nextNode()
    }
    return out
  })

  const leaks: string[] = []
  for (const raw of visibleText) {
    const stripped = raw.replace(PSEUDO_BRACKETED, '')
    const matches = stripped.match(BARE_ENGLISH)
    if (!matches) continue
    for (const m of matches) {
      if (ALLOW_LIST.has(m)) continue
      leaks.push(`"${m}" in "${raw.slice(0, 80)}"`)
    }
  }
  return leaks
}

test.describe('Pseudo-locale sweep (en-XA)', () => {
  test.beforeEach(async ({ page }) => {
    // Seed before any app script runs. addInitScript fires on every
    // navigation in the page, so the locale store and auth store both
    // hydrate from localStorage on first render.
    //
    // The locale store uses pinia-plugin-persistedstate, which serializes
    // the tracked subset as JSON under key 'locale'. The auth store reads
    // 'auth_token' and 'auth_user' directly. Mock auth lets us skip the
    // login form; the API runs in mock mode, so any token value works.
    await page.addInitScript((user) => {
      localStorage.setItem('locale', JSON.stringify({ locale: 'en-XA' }))
      localStorage.setItem('auth_token', 'mock-jwt-token-pseudo-test')
      localStorage.setItem('auth_user', JSON.stringify(user))
    }, MOCK_USER)
  })

  for (const route of ROUTES_TO_SWEEP) {
    test(`no bare English on ${route}`, async ({ page }) => {
      await page.goto(route)
      await page.waitForLoadState('networkidle')

      const leaks = await collectBareEnglish(page)
      expect(
        leaks,
        `Unmigrated strings on ${route}:\n  - ${leaks.join('\n  - ')}`,
      ).toEqual([])
    })
  }
})
