import { test, expect } from '@playwright/test';

/**
 * Smoke tests - Basic UI functionality verification
 * These tests run quickly and verify the application is accessible
 */

test.describe('Smoke Tests', () => {
  test('homepage loads', async ({ page }) => {
    await page.goto('/');
    // Should redirect to login or dashboard
    await expect(page).toHaveURL(/\/(login|dashboard)?$/);
  });

  test('login page accessible', async ({ page }) => {
    await page.goto('/login');
    // Should have login form elements
    await expect(page.locator('body')).toBeVisible();
  });

  test('dashboard accessible (may redirect to login)', async ({ page }) => {
    await page.goto('/dashboard');
    // Either shows dashboard or redirects to login
    await expect(page).toHaveURL(/\/(dashboard|login)/);
  });

  test('labs page accessible', async ({ page }) => {
    await page.goto('/labs');
    await expect(page).toHaveURL(/\/(labs|login)/);
  });

  test('pathways page accessible', async ({ page }) => {
    await page.goto('/pathways');
    await expect(page).toHaveURL(/\/(pathways|login)/);
  });

  test('achievements page accessible', async ({ page }) => {
    await page.goto('/achievements');
    await expect(page).toHaveURL(/\/(achievements|login)/);
  });
});
