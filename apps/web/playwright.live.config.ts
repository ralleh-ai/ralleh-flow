import { defineConfig, devices } from '@playwright/test'

const baseURL = 'http://127.0.0.1:4317'
const apiBaseURL = 'http://127.0.0.1:4320/v1'

export default defineConfig({
  testDir: './tests/e2e',
  grep: /@live/,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report-live', open: 'never' }]
  ],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    headless: true
  },
  webServer: [
    {
      command: 'cd ../api && go run ./cmd/ralleh-flow-api',
      url: 'http://127.0.0.1:4320/v1/healthz',
      reuseExistingServer: false,
      timeout: 120000,
      env: {
        RALLEH_FLOW_API_ADDR: '127.0.0.1:4320',
        RALLEH_FLOW_DATA_DIR: '../web/.tmp/playwright-live-api-data',
        RALLEH_FLOW_DB_PATH: '../web/.tmp/playwright-live-api-data/ralleh-flow.db',
        RALLEH_FLOW_REPO_ROOT: '../..',
        RALLEH_FLOW_ALLOWED_ORIGINS: 'http://127.0.0.1:4317,http://localhost:4317'
      }
    },
    {
      command: 'pnpm test:ui:server',
      url: baseURL,
      reuseExistingServer: false,
      timeout: 120000,
      env: {
        NUXT_PUBLIC_API_BASE: apiBaseURL
      }
    }
  ],
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] }
    }
  ]
})
