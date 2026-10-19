import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { Button } from '~/components/ui/button';

describe('ui/Button', () => {
  it('emits click', async () => {
    const w = await mountSuspended(Button, { slots: { default: () => 'Сохранить' } });
    expect(w.text()).toContain('Сохранить');
    await w.find('button').trigger('click');
    expect(w.emitted('click')).toHaveLength(1);
  });

  it('loading disables the button, shows a spinner and blocks click', async () => {
    const w = await mountSuspended(Button, { props: { loading: true }, slots: { default: () => 'Ок' } });
    const btn = w.find('button');
    expect(btn.attributes('disabled')).toBeDefined();
    expect(btn.attributes('aria-busy')).toBe('true');
    expect(w.find('[data-testid="spinner"]').exists()).toBe(true);
    await btn.trigger('click');
    expect(w.emitted('click')).toBeUndefined();
  });

  it('disabled does not emit click', async () => {
    const w = await mountSuspended(Button, { props: { disabled: true } });
    await w.find('button').trigger('click');
    expect(w.emitted('click')).toBeUndefined();
  });

  it('renders a link when `to` is set', async () => {
    const w = await mountSuspended(Button, { props: { to: '/listings/new' }, slots: { default: () => 'Создать' } });
    const a = w.find('a');
    expect(a.exists()).toBe(true);
    expect(a.attributes('href')).toBe('/listings/new');
    expect(w.find('button').exists()).toBe(false);
  });
});
