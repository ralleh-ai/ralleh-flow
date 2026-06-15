import { defineConfig, devices } from '@playwright/test'

const baseURL = 'http://127.0.0.1:4317'

export default defineConfig({
  testDir: './tests/e2e',
  grepInvert: /@live/,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }]
  ],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
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
