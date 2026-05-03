import { defineConfig, devices } from '@playwright/test'

const port = Number(process.env.ZIN_QA_E2E_PORT ?? 5179)

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: `npm run dev -- --host 127.0.0.1 --port ${port} --strictPort`,
    url: `http://127.0.0.1:${port}/e2e/index.html`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
