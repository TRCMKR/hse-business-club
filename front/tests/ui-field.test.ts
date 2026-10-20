import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { Input } from '~/components/ui/input';

describe('ui/Field via Input', () => {
  it('shows the hint without an error', async () => {
    const w = await mountSuspended(Input, { props: { id: 'pwd', label: 'Пароль', hint: 'От 8 до 72 символов', modelValue: '' } });
    expect(w.text()).toContain('От 8 до 72 символов');
    expect(w.find('input').attributes('aria-describedby')).toBe('pwd-hint');
  });

  it('an error replaces the hint', async () => {
    const w = await mountSuspended(Input, {
      props: { id: 'pwd', label: 'Пароль', hint: 'От 8 до 72 символов', error: 'Пароль: от 8 до 72 символов', modelValue: '' },
    });
    expect(w.text()).toContain('Пароль: от 8 до 72 символов');
    expect(w.text()).not.toContain('От 8 до 72 символов');
    expect(w.find('input').attributes('aria-describedby')).toBe('pwd-error');
  });
});
