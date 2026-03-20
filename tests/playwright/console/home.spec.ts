import { test, expect } from '@playwright/test';

test.describe('Console Home Page', () => {
  test('loads the home page', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/EntDB/i);
  });

  test('displays navigation links', async ({ page }) => {
    await page.goto('/');
    // Console should have navigation to types, search, etc.
    await expect(page.locator('nav')).toBeVisible();
  });

  test('navigates to types page', async ({ page }) => {
    await page.goto('/');
    // Find and click the types link
    const typesLink = page.locator('a[href*="types"]').first();
    if (await typesLink.isVisible()) {
      await typesLink.click();
      await expect(page).toHaveURL(/types/);
    }
  });
});
