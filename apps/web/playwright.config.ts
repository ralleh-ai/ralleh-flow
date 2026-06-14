import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:4300',
    trace: 'on-first-retry',
    headless: true
  },
  webServer: {
    command: 'NUXT_UI_TEST_MODE=1 NUXT_APP_BASE_URL=/ NUXT_PUBLIC_API_BASE=/api/flow/v1 pnpm dev',
    port: 4300,
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
