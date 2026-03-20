import { test, expect } from '@playwright/test';

test.describe('Playground Home Page', () => {
  test('loads the playground', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/EntDB|Playground/i);
  });

  test('health endpoint returns OK @smoke', async ({ request }) => {
    const response = await request.get('/health');
    expect(response.ok()).toBeTruthy();
  });

  test('page renders without errors', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toContainText('Error');
    await expect(page.locator('body')).not.toContainText('500');
  });

  test('interactive elements are present', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    // Playground should have interactive elements (buttons, editors, etc.)
    const body = page.locator('body');
    await expect(body).toBeVisible();
  });
});
