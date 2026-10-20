import { expect } from '@playwright/test';
import type { Page } from '@playwright/test';

/** Seed accounts (SPEC §6): button label on /login is «Войти как <name>». */
export const DEMO = {
  anna: 'Анна',
  boris: 'Борис',
  vera: 'Вера',
  dmitry: 'Дмитрий',
} as const;

export type DemoUser = keyof typeof DEMO;

/** Signs in through the «Войти как …» button and waits for the feed. */
export async function loginAs(page: Page, user: DemoUser): Promise<void> {
  await page.goto('/login');
  await page.getByRole('button', { name: `Войти как ${DEMO[user]}` }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
}

/** Signs out with the «Выйти» button on the profile page (present at every width). */
export async function logout(page: Page): Promise<void> {
  await page.goto('/me');
  await page.getByRole('main').getByRole('button', { name: 'Выйти' }).click();
  await expect(page).toHaveURL(/\/login$/);
}

/** Unique suffix so data-changing tests survive a reused (stateful) mock server. */
export const uniq = (): string => `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
