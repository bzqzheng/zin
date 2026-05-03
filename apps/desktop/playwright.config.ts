import { defineConfig, devices } from '@playwright/test'

const appPort = Number(process.env.ZIN_E2E_APP_PORT ?? 5173)
const apiPort = Number(process.env.ZIN_E2E_API_PORT ?? 4174)

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false,
  reporter: process.env.CI ? [['list']] : [['line']],
  use: {
    baseURL: `http://127.0.0.1:${appPort}`,
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: [
    {
      command: `node tests/e2e/mock-api.mjs --port ${apiPort}`,
      url: `http://127.0.0.1:${apiPort}/health`,
      reuseExistingServer: !process.env.CI,
      timeout: 10_000,
    },
    {
      command: `npm run dev -- --host 127.0.0.1 --port ${appPort}`,
      url: `http://127.0.0.1:${appPort}`,
      reuseExistingServer: !process.env.CI,
      timeout: 20_000,
      env: {
        VITE_ZIN_MOCK_DAEMON_BASE_URL: `http://127.0.0.1:${apiPort}`,
      },
    },
  ],
})
