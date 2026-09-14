import { defineConfig, devices } from 'patchright/test'

/**
 * Playwright E2E Configuration
 * Tests site (storefront) and admin portal
 */

/**
 * The port the suite runs on.
 *
 * 8080 is the default, and E2E_PORT moves the whole suite — server, base URL
 * and the store's own domain — so it can run next to something that already
 * holds the default port.
 */
const PORT = Number(process.env.E2E_PORT ?? 8080)
const BASE_URL = `http://localhost:${PORT}`

/**
 * The browser to drive.
 *
 * Nothing is pinned by default: patchright brings its own Chromium, which is
 * the one CI downloads. A machine that cannot use it — the BSDs, where
 * patchright distributes no browser — names its own with E2E_EXECUTABLE_PATH,
 * and `channel: 'chrome'` with the OpenBSD patch installed works too.
 */
const EXECUTABLE_PATH = process.env.E2E_EXECUTABLE_PATH

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false, // Run tests sequentially to avoid conflicts
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker to prevent port conflicts
  reporter: 'html',
  globalSetup: './e2e/global-setup.ts',

  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'on', // Capture a screenshot after every single test execution
    video: 'on', // Record a video for every single test execution
    actionTimeout: 10000,
  },

  projects: [
    {
      name: 'chrome',
      use: {
        ...devices['Desktop Chrome'],
        ...(EXECUTABLE_PATH ? { executablePath: EXECUTABLE_PATH } : {}),
      },
    },
  ],

  // Start Go server (build separately before running tests)
  webServer: {
    command: './scripts/test-server-start.sh',
    url: BASE_URL,
    reuseExistingServer: false, // Always restart to ensure clean database
    timeout: 30000, // 30 seconds for server start
    stdout: 'pipe',
    stderr: 'pipe',
    // Graceful shutdown instead of force-kill
    gracefulShutdown: {
      signal: 'SIGTERM',
      timeout: 1000, // Time in ms to wait before escalation
    },
  },
})
