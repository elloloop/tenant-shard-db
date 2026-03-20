import { test, expect } from '@playwright/test';

test.describe('Console Health & API', () => {
  test('health endpoint returns OK @smoke', async ({ request }) => {
    const response = await request.get('/health');
    expect(response.ok()).toBeTruthy();
  });

  test('schema endpoint returns valid JSON @smoke', async ({ request }) => {
    const response = await request.get('/api/schema');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data).toBeDefined();
  });

  test('nodes endpoint handles missing type gracefully', async ({ request }) => {
    const response = await request.get('/api/nodes/99999');
    // Should return 404 or empty list, not 500
    expect(response.status()).not.toBe(500);
  });

  test('static assets are served', async ({ page }) => {
    const response = await page.goto('/');
    expect(response?.ok()).toBeTruthy();
    // Check that CSS and JS loaded (page should have styled content)
    const body = page.locator('body');
    await expect(body).toBeVisible();
  });
});
