import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:3001',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3001',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
    env: {
      // Enables app/e2e/* harness routes, which 404 everywhere else.
      NEXT_PUBLIC_E2E_HARNESS: '1',
      // Points the client at a socket URL the tests intercept with
      // page.routeWebSocket — no real hub needs to be running.
      NEXT_PUBLIC_WS_URL: 'ws://localhost:3001/ws',
    },
  },
});
