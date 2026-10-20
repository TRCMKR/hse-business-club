import { expect, test } from '@playwright/test';
import type { BrowserContext, Page } from '@playwright/test';
import { loginAs } from './helpers';

/**
 * The e2e server runs the mock with NUXT_MOCK_ACCESS_TTL=1s (playwright.config.ts), so
 * a 1.5 s pause is enough for the access token to expire.
 */
const ACCESS_EXPIRY_PAUSE_MS = 1_500;

const nav = (page: Page) => page.getByRole('navigation', { name: 'Основная навигация' });

const sessionCookies = async (context: BrowserContext) => {
  const cookies = await context.cookies();
  return {
    access: cookies.find(c => c.name === 'hbc_access'),
    refresh: cookies.find(c => c.name === 'hbc_refresh'),
  };
};

test('tokens are invisible to JS and both cookies are httpOnly', async ({ page, context }) => {
  await loginAs(page, 'anna');
  expect(await page.evaluate(() => document.cookie)).toBe('');
  expect(await page.evaluate(() => JSON.stringify(localStorage) + JSON.stringify(sessionStorage))).toBe('{}{}');
  const { access, refresh } = await sessionCookies(context);
  for (const cookie of [access, refresh]) {
    expect(cookie?.httpOnly).toBe(true);
    expect(cookie?.sameSite).toBe('Lax');
    expect(cookie?.path).toBe('/');
  }
});

test('access expires mid-session → actions keep working without a redirect', async ({ page, context }) => {
  await loginAs(page, 'boris');
  const before = await sessionCookies(context);

  await page.waitForTimeout(ACCESS_EXPIRY_PAUSE_MS);
  // Client-side navigation: the page's API calls go through the proxy, which refreshes.
  await nav(page).getByRole('link', { name: /Мои публикации|Публикации/ }).click();
  await expect(page).toHaveURL(/\/me\/listings$/);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(page.getByText('Сессия истекла')).toHaveCount(0);

  const after = await sessionCookies(context);
  expect(after.access?.value).toBeTruthy();
  expect(after.access?.value).not.toBe(before.access?.value);
  expect(after.refresh?.value).not.toBe(before.refresh?.value);

  // SSR with an expired access token: refreshed during render, no trip to /login.
  await page.waitForTimeout(ACCESS_EXPIRY_PAUSE_MS);
  const response = await page.goto('/me/applications');
  expect(response?.status()).toBe(200);
  await expect(page).toHaveURL(/\/me\/applications$/);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

  // A mutation after another expiry: profile save goes through as well.
  await page.waitForTimeout(ACCESS_EXPIRY_PAUSE_MS);
  await page.goto('/me');
  await page.getByRole('button', { name: /Редактировать/ }).first().click();
  const headline = page.getByTestId('profile-form').getByLabel('Кратко о себе');
  await headline.fill('Сессия переживает истечение access');
  await page.waitForTimeout(ACCESS_EXPIRY_PAUSE_MS);
  await page.getByTestId('profile-form').getByRole('button', { name: /Сохранить/ }).click();
  await expect(page.getByText('Сессия переживает истечение access').first()).toBeVisible();
  await expect(page).toHaveURL(/\/me$/);
  await expect(page.getByText('Сессия истекла')).toHaveCount(0);
});

test('revoked refresh → «Сессия истекла» → /login, and the old cookies stay dead', async ({ page, context }) => {
  await loginAs(page, 'vera');
  const stale = await context.cookies();

  // Log out «in another tab» (revokes the refresh token), then put the old cookies back,
  // as if this tab still held them.
  const logout = await page.request.post('/api/auth/logout');
  expect(logout.status()).toBe(200);
  await context.addCookies(stale);

  await page.waitForTimeout(ACCESS_EXPIRY_PAUSE_MS);
  await nav(page).getByRole('link', { name: /Мои публикации|Публикации/ }).click();

  await expect(page.getByText('Сессия истекла', { exact: false }).first()).toBeVisible();
  await expect(page).toHaveURL(/\/login\?redirect=(%2F|\/)me(%2F|\/)listings$/);
  expect((await sessionCookies(context)).refresh).toBeUndefined();

  // Even re-planted, the revoked refresh token does not revive the session on SSR.
  await context.addCookies(stale);
  await page.goto('/me');
  await expect(page).toHaveURL(/\/login\?redirect=(%2F|\/)me$/);
  expect((await sessionCookies(context)).refresh).toBeUndefined();
});
