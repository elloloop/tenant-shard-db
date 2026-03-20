import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for EntDB Console and Playground E2E tests.
 *
 * Prerequisites:
 *   docker compose up -d   (starts the full EntDB stack)
 *
 * Run:
 *   npx playwright test                    # all tests
 *   npx playwright test --project=console  # console only
 *   npx playwright test --project=playground  # playground only
 */
export default defineConfig({
  testDir: '.',
  testMatch: ['console/**/*.spec.ts', 'playground/**/*.spec.ts'],
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? 'github' : 'html',
  timeout: 30_000,

  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'console',
      testDir: './console',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: process.env.CONSOLE_URL || 'http://localhost:8080',
      },
    },
    {
      name: 'playground',
      testDir: './playground',
      use: {
        ...devices['Desktop Chrome'],
        baseURL: process.env.PLAYGROUND_URL || 'http://localhost:8081',
      },
    },
  ],

  /* Start the full stack before running tests (if not already running) */
  webServer: process.env.CI ? undefined : [
    {
      command: 'docker compose up -d --build && until curl -sf http://localhost:8080/health > /dev/null; do sleep 2; done && echo "ready"',
      url: 'http://localhost:8080/health',
      reuseExistingServer: true,
      timeout: 180_000,
      cwd: '../..',
    },
  ],
});
