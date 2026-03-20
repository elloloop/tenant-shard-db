import { test, expect } from '@playwright/test';

test.describe('Console Schema Browser', () => {
  test('types page loads', async ({ page }) => {
    await page.goto('/types');
    // Should show a list of node/edge types from the schema
    await page.waitForLoadState('networkidle');
    // The page should render without errors
    await expect(page.locator('body')).not.toContainText('Error');
  });

  test('schema API returns valid data', async ({ request }) => {
    const response = await request.get('/api/schema');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    // Schema response should have node_types and edge_types
    expect(data).toHaveProperty('node_types');
    expect(data).toHaveProperty('edge_types');
  });

  test('clicking a type shows its nodes', async ({ page }) => {
    await page.goto('/types');
    await page.waitForLoadState('networkidle');

    // If there are type cards/links, click the first one
    const typeLink = page.locator('[data-testid="type-card"], a[href*="/types/"]').first();
    if (await typeLink.isVisible()) {
      await typeLink.click();
      await page.waitForLoadState('networkidle');
      // Should navigate to the node list for that type
      await expect(page).toHaveURL(/types\/\d+/);
    }
  });
});
