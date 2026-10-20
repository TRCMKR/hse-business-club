import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import type { Page, TestInfo } from '@playwright/test';
import { loginAs } from './helpers';

/**
 * axe (wcag2a + wcag2aa) on the key screens, light and dark theme; the viewport comes from
 * the project (desktop 1280 / mobile 375). Every violation is logged and attached;
 * the test fails on `serious` / `critical` ones.
 */
const BLOCKING = new Set(['serious', 'critical']);
const THEMES = ['light', 'dark'] as const;

async function audit(page: Page, testInfo: TestInfo, label: string): Promise<void> {
  // Let entrance transitions finish so colours are final.
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(400);
  const { violations, passes, incomplete } = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze();

  const counts: Record<string, number> = { minor: 0, moderate: 0, serious: 0, critical: 0 };
  for (const v of violations) {
    counts[v.impact ?? 'minor'] = (counts[v.impact ?? 'minor'] ?? 0) + v.nodes.length;
  }
  const summary = `[axe] ${testInfo.project.name} ${label}: ${JSON.stringify(counts)}`
    + ` (rules passed: ${passes.length}, needs review: ${incomplete.map(r => `${r.id}×${r.nodes.length}`).join(', ') || 0})`;
  console.log(summary);
  for (const v of violations) {
    console.log(`  ${v.impact} ${v.id}: ${v.help} (${v.nodes.length})`);
    for (const n of v.nodes.slice(0, 5)) {
      console.log(`    ${n.target.join(' ')} — ${n.failureSummary?.split('\n').slice(1).join(' ').trim()}`);
    }
  }
  for (const r of incomplete) {
    console.log(`  needs review ${r.id}: ${r.nodes.map(n => n.target.join(' ')).join(' | ')}`);
  }
  await testInfo.attach(`axe-${label}`, { body: JSON.stringify(violations, null, 2), contentType: 'application/json' });

  const blocking = violations.filter(v => BLOCKING.has(v.impact ?? ''));
  expect.soft(blocking.map(v => `${v.impact} ${v.id} ×${v.nodes.length}`), summary).toEqual([]);
}

for (const theme of THEMES) {
  test.describe(`a11y, ${theme} theme`, () => {
    test.use({ colorScheme: theme });

    test('/login', async ({ page }, testInfo) => {
      await page.goto('/login');
      await audit(page, testInfo, `/login ${theme}`);
    });

    test('signed-in pages', async ({ page }, testInfo) => {
      await loginAs(page, 'anna');
      await audit(page, testInfo, `/ ${theme}`);

      // Someone else's listing (with «Откликнуться») — opened from the feed.
      const foreign = page.getByTestId('listing-card').filter({ hasNotText: 'Анна' }).first();
      await foreign.getByRole('link').first().click();
      await expect(page).toHaveURL(/\/listings\/[0-9a-f-]{36}$/);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await audit(page, testInfo, `/listings/:id ${theme}`);

      await page.goto('/me/applications?tab=received');
      await expect(page.getByRole('tab', { name: /Полученные/ })).toHaveAttribute('aria-selected', 'true');
      await audit(page, testInfo, `/me/applications?tab=received ${theme}`);

      await page.goto('/me');
      await expect(page.getByRole('heading', { level: 1, name: 'Профиль' })).toBeVisible();
      await audit(page, testInfo, `/me ${theme}`);
    });
  });
}
