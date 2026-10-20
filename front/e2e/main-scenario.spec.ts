import { expect, test } from '@playwright/test';
import { loginAs, logout, uniq } from './helpers';

/**
 * SPEC §1 main scenario through the UI:
 * Анна публикует стартап → Борис находит его в ленте и откликается →
 * Анна видит отклик и принимает → Борис видит «Принят» в кабинете.
 */
test('main scenario: publish → apply → accept → applicant sees «Принят»', async ({ page }) => {
  const title = `E2E стартап ${uniq()}`;
  const message = 'Привет! Я backend-разработчик, готов присоединиться. Telegram: @boris';

  await test.step('Анна создаёт стартап', async () => {
    await loginAs(page, 'anna');
    await page.goto('/listings/new');
    const form = page.getByTestId('listing-form');
    await form.getByLabel('Название').fill(title);
    await form.getByLabel('Стадия').selectOption({ label: 'MVP' });
    await form.getByLabel('Описание').fill('Сервис для поиска сооснователей среди участников клуба.');
    await form.getByLabel('Кого ищете').fill('Backend-разработчика на долю');
    await form.getByTestId('listing-form-submit').click();

    await expect(page).toHaveURL(/\/listings\/[0-9a-f-]{36}$/);
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await logout(page);
  });

  await test.step('Борис находит стартап в ленте и откликается', async () => {
    await loginAs(page, 'boris');
    await page.getByRole('searchbox').fill(title);
    const card = page.getByTestId('listing-card').filter({ hasText: title });
    await expect(card).toHaveCount(1);
    await card.getByRole('link', { name: title }).click();

    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await page.getByTestId('apply').click();
    const sheet = page.getByTestId('apply-sheet');
    await sheet.getByLabel('Сопроводительное сообщение').fill(message);
    await page.getByTestId('apply-submit').click();
    await expect(page.getByText('Отклик отправлен').first()).toBeVisible();
    await expect(sheet).toBeHidden();
    await logout(page);
  });

  await test.step('Анна видит отклик и принимает', async () => {
    await loginAs(page, 'anna');
    await page.goto('/me/applications?tab=received');
    const card = page.getByTestId('application-card').filter({ hasText: title });
    await expect(card).toHaveCount(1);
    await expect(card).toContainText('Борис');
    await expect(card).toContainText(message);
    await card.getByTestId('application-accept').click();
    await expect(page.getByText('Отклик принят').first()).toBeVisible();
    await expect(card).toContainText('Принят');
    await expect(card.getByTestId('application-accept')).toHaveCount(0);
    await logout(page);
  });

  await test.step('Борис видит «Принят» в кабинете', async () => {
    await loginAs(page, 'boris');
    await page.goto('/me/applications');
    await expect(page.getByRole('tab', { name: /Отправленные/ })).toHaveAttribute('aria-selected', 'true');
    const card = page.getByTestId('application-card').filter({ hasText: title });
    await expect(card).toHaveCount(1);
    await expect(card).toContainText('Принят');
  });
});
