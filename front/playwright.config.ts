import { defineConfig } from '@playwright/test';

/**
 * E2E against the production build in mock-API mode (in-memory state, SPEC §6 fixtures).
 * Uses the system Chrome (`channel: 'chrome'`), no Playwright browser download.
 * Tests that change data use unique titles, so they survive `reuseExistingServer`.
 * The mock issues access tokens for 1 s (`NUXT_MOCK_ACCESS_TTL`), so the whole suite
 * runs with the proxy/SSR refreshing the session all the time (e2e/session.spec.ts
 * checks it explicitly).
 */
const PORT = Number(process.env.E2E_PORT ?? 13100);
const baseURL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL,
    channel: 'chrome',
    locale: 'ru-RU',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'desktop',
      use: { viewport: { width: 1280, height: 800 } },
    },
    {
      name: 'mobile',
      use: { viewport: { width: 375, height: 812 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
    },
  ],
  webServer: {
    command: `npm run build && node .output/server/index.mjs`,
    url: `${baseURL}/login`,
    env: { NUXT_MOCK_API: 'true', NUXT_MOCK_ACCESS_TTL: '1s', PORT: String(PORT), HOST: '127.0.0.1' },
    reuseExistingServer: !process.env.CI,
    timeout: 300_000,
    stdout: 'ignore',
    stderr: process.env.E2E_SERVER_LOG ? 'pipe' : 'ignore',
  },
});
