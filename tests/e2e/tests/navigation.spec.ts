import { test, expect } from '@playwright/test';

/**
 * Navigation tests - Verify main navigation works correctly
 * Requires demo mode to be enabled on the API (AUTH_DEMO_MODE=true)
 */

test.describe('Navigation', () => {
  // Run serially to avoid rate-limiting on demo login
  test.describe.configure({ mode: 'serial' });

  test.beforeEach(async ({ page }) => {
    // Log in via demo account to access authenticated pages
    await page.goto('/login');
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: /demo/i }).click();
    await page.waitForURL(/\/$|\/dashboard/, { timeout: 10000 });
    // Wait for sidebar to render
    await page.locator('aside').waitFor({ state: 'visible', timeout: 10000 });
  });

  test('sidebar navigation items are visible', async ({ page }) => {
    const nav = page.locator('nav');
    await expect(nav).toBeVisible();

    const aside = page.locator('aside');
    await expect(aside).toBeVisible();

    // Verify key navigation items exist in the sidebar
    const expectedItems = ['Dashboard', 'Pathways', 'Lab Catalog', 'Achievements'];
    for (const item of expectedItems) {
      const button = page.locator('aside button', { hasText: new RegExp(`^\\s*${item}\\s*$`) });
      await expect(button).toBeVisible();
    }
  });

  test('can navigate to labs from sidebar', async ({ page }) => {
    await page.locator('nav button', { hasText: 'Lab Catalog' }).click();
    await expect(page).toHaveURL(/labs/);
  });

  test('can navigate to pathways from sidebar', async ({ page }) => {
    await page.locator('nav button', { hasText: 'Pathways' }).click();
    await expect(page).toHaveURL(/pathways/);
  });

  test('can navigate to achievements from sidebar', async ({ page }) => {
    await page.locator('nav button', { hasText: 'Achievements' }).click();
    await expect(page).toHaveURL(/achievements/);
  });
});
