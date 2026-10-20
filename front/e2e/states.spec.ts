import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';
import { loginAs } from './helpers';

const MISSING_ID = '00000000-0000-0000-0000-000000000000';

/** «Лента» link in whichever main navigation is visible (header ≥ 768px, tab bar below). */
const feedNavLink = (page: Page) =>
  page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('link', { name: 'Лента', exact: true });

test.describe('unsaved form guard', () => {
  test('leaving /listings/new with changes asks for confirmation', async ({ page }) => {
    await loginAs(page, 'anna');
    await page.goto('/listings/new');
    const titleInput = page.getByTestId('listing-form').getByLabel('Название');
    await titleInput.fill('Черновик, который нельзя потерять');

    await feedNavLink(page).click();
    const dialog = page.getByRole('dialog', { name: 'Уйти без сохранения?' });
    await expect(dialog).toBeVisible();

    await dialog.getByRole('button', { name: 'Остаться' }).click();
    await expect(dialog).toBeHidden();
    await expect(page).toHaveURL(/\/listings\/new$/);
    await expect(titleInput).toHaveValue('Черновик, который нельзя потерять');

    await feedNavLink(page).click();
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Уйти' }).click();
    await expect(page).toHaveURL(/\/$/);
  });

  test('pristine form leaves without confirmation', async ({ page }) => {
    await loginAs(page, 'anna');
    await page.goto('/listings/new');
    await expect(page.getByTestId('listing-form')).toBeVisible();
    await feedNavLink(page).click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole('dialog')).toHaveCount(0);
  });
});

test('401 mid-session: toast «Сессия истекла» and redirect to /login?redirect=…', async ({ page, context }) => {
  await loginAs(page, 'dmitry');
  await context.clearCookies();

  await page.getByRole('navigation', { name: 'Основная навигация' })
    .getByRole('link', { name: /Мои публикации|Публикации/ })
    .click();

  await expect(page.getByText('Сессия истекла', { exact: false }).first()).toBeVisible();
  await expect(page).toHaveURL(/\/login\?redirect=(%2F|\/)me(%2F|\/)listings$/);
  await expect(page.getByRole('button', { name: 'Войти как Дмитрий' })).toBeVisible();
});

test.describe('404 by direct link', () => {
  test.beforeEach(async ({ page }) => {
    await loginAs(page, 'anna');
  });

  const cases = [
    { path: `/listings/${MISSING_ID}`, text: /не найден/i },
    { path: `/listings/${MISSING_ID}/edit`, text: /не найден/i },
    { path: `/listings/${MISSING_ID}/applications`, text: /не найден/i },
    { path: `/users/${MISSING_ID}`, text: /не найден/i },
    { path: '/no-such-page', text: /не найден/i },
  ];

  for (const { path, text } of cases) {
    test(`${path} → HTTP 404 and «не найден»`, async ({ page }) => {
      const response = await page.goto(path);
      expect(response?.status()).toBe(404);
      await expect(page.getByText(text).first()).toBeVisible();
    });
  }
});
