import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Kootenai E2E tests
 *
 * Run tests against the deployed infra VM (set E2E_BASE_URL to its
 * address, e.g. http://<INFRA_IP>:3000)
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['list'],
    ['html', { open: 'never' }],
    ['json', { outputFile: 'test-results.json' }],
  ],

  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://192.0.2.10:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
    // Ignore HTTPS certificate errors (self-signed certs in dev/staging)
    ignoreHTTPSErrors: true,
  },

  // Define test projects for different browsers
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    // Optionally test on Firefox and Safari
    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },
    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },
  ],

  // Timeout for each test
  timeout: 30000,

  // Expect options
  expect: {
    timeout: 5000,
  },
});
