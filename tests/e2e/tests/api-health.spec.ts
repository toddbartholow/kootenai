import { test, expect } from '@playwright/test';

/**
 * API Health tests - Verify backend API is accessible from frontend
 */

const API_BASE = process.env.E2E_API_URL || 'http://<INFRA_VM_IP>:8080';

test.describe('API Health', () => {
  test('API health endpoint responds', async ({ request }) => {
    const response = await request.get(`${API_BASE}/health`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.status).toBe('ok');
  });

  test('API ready endpoint responds', async ({ request }) => {
    const response = await request.get(`${API_BASE}/ready`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(['ready', 'healthy']).toContain(data.status);
  });

  test('labs API returns data', async ({ request }) => {
    const response = await request.get(`${API_BASE}/api/v1/labs`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.count).toBeGreaterThan(0);
    expect(data.labs).toBeInstanceOf(Array);
  });

  test('achievements API returns data', async ({ request }) => {
    const response = await request.get(`${API_BASE}/api/v1/achievements`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.achievements).toBeInstanceOf(Array);
    expect(data.achievements.length).toBeGreaterThan(0);
  });

  test('pathways API returns data', async ({ request }) => {
    const response = await request.get(`${API_BASE}/api/v1/pathways`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    // Pathways may be empty array if none configured
    expect(Array.isArray(data) || Array.isArray(data.pathways)).toBeTruthy();
  });

  test('API version endpoint responds', async ({ request }) => {
    // Version endpoint may be at different paths depending on API version
    let response = await request.get(`${API_BASE}/api/v1/version`);
    if (!response.ok()) {
      response = await request.get(`${API_BASE}/version`);
    }

    // Version endpoint is optional - skip if not available
    if (!response.ok()) {
      test.skip();
      return;
    }

    const data = await response.json();
    expect(data.version).toBeDefined();
  });
});
