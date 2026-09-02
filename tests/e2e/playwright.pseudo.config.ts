import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright config for the pseudo-locale sweep.
 *
 * Spins up its own vite dev server in mock + pseudo mode so the suite has
 * no infra dependencies — runs cleanly in CI on a single ubuntu-latest
 * runner. The base e2e config (playwright.config.ts) targets a deployed
 * infra VM and is left untouched.
 */
export default defineConfig({
  testDir: './tests',
  testMatch: /pseudo-locale\.spec\.ts$/,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [['github'], ['list']]
    : [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  timeout: 30000,
  expect: { timeout: 5000 },

  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: 'npm run dev -- --port 5173 --strictPort',
        cwd: '../../web',
        url: 'http://localhost:5173',
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        env: {
          VITE_I18N_PSEUDO: '1',
          VITE_USE_MOCK_DATA: 'true',
        },
      },
})
