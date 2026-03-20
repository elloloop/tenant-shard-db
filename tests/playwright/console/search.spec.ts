import { test, expect } from '@playwright/test';

test.describe('Console Search', () => {
  test('search page loads', async ({ page }) => {
    await page.goto('/search');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toContainText('Error');
  });

  test('search input is present', async ({ page }) => {
    await page.goto('/search');
    const searchInput = page.locator('input[type="text"], input[type="search"], [role="searchbox"]').first();
    await expect(searchInput).toBeVisible();
  });

  test('empty search shows appropriate state', async ({ page }) => {
    await page.goto('/search');
    await page.waitForLoadState('networkidle');
    // Should show empty state or prompt, not an error
    await expect(page.locator('body')).not.toContainText('500');
    await expect(page.locator('body')).not.toContainText('Internal Server Error');
  });
});
