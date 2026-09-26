import { defineConfig, devices } from '@playwright/test'

const PORT = 5176
const BASE_URL = `http://localhost:${PORT}`

export default defineConfig({
  testDir: './e2e-backend',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : [['list']],
  outputDir: './test-results/playwright-backend',
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  globalSetup: './e2e-backend/global-setup.ts',
  globalTeardown: './e2e-backend/global-teardown.ts',
})
