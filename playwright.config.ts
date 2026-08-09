import { defineConfig, devices } from 'patchright/test'

/**
 * Playwright E2E Configuration
 * Tests site (storefront) and admin portal
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false, // Run tests sequentially to avoid conflicts
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker to prevent port conflicts
  reporter: 'html',
  globalSetup: './e2e/global-setup.ts',

  use: {
    baseURL: 'http://localhost:8080',
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
        channel: 'chrome',
        executablePath: '/usr/local/bin/chrome',
      },
    },
  ],

  // Start Go server (build separately before running tests)
  webServer: {
    command: './scripts/test-server-start.sh',
    url: 'http://localhost:8080',
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
