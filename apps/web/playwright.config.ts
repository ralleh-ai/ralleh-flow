import { defineConfig, devices } from '@playwright/test'

const baseURL = 'http://127.0.0.1:4317'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
    headless: true
  },
  webServer: {
    command: 'pnpm test:ui:server',
    url: baseURL,
    reuseExistingServer: false,
    timeout: 120000
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] }
    }
  ]
})
