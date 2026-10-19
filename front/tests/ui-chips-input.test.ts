import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ChipsInput } from '~/components/ui/chips-input';

const mountChips = (modelValue: string[], max = 3) => mountSuspended(ChipsInput, {
  props: {
    'modelValue': modelValue,
    max,
    'onUpdate:modelValue': () => {},
  },
});

const lastEmit = (w: Awaited<ReturnType<typeof mountChips>>) => {
  const e = w.emitted('update:modelValue');
  return e ? (e[e.length - 1]![0] as string[]) : undefined;
};

describe('ui/ChipsInput', () => {
  it('adds on Enter (trimmed)', async () => {
    const w = await mountChips([]);
    const input = w.find('input');
    await input.setValue('  go  ');
    await input.trigger('keydown', { key: 'Enter' });
    expect(lastEmit(w)).toEqual(['go']);
  });

  it('adds on comma', async () => {
    const w = await mountChips(['go']);
    const input = w.find('input');
    await input.setValue('react,');
    expect(lastEmit(w)).toEqual(['go', 'react']);
    expect((input.element as HTMLInputElement).value).toBe('');
  });

  it('ignores case-insensitive duplicates and empty values', async () => {
    const w = await mountChips(['Go']);
    const input = w.find('input');
    await input.setValue('go');
    await input.trigger('keydown', { key: 'Enter' });
    await input.setValue('   ');
    await input.trigger('keydown', { key: 'Enter' });
    expect(w.emitted('update:modelValue')).toBeUndefined();
  });

  it('never exceeds max and blocks input when full', async () => {
    const w = await mountChips(['a', 'b'], 3);
    const input = w.find('input');
    await input.setValue('c,d,e,');
    expect(lastEmit(w)).toEqual(['a', 'b', 'c']);

    const full = await mountChips(['a', 'b', 'c'], 3);
    expect(full.find('input').attributes('disabled')).toBeDefined();
    expect(full.text()).toContain('Не больше 3');
  });

  it('Backspace on empty input removes the last item', async () => {
    const w = await mountChips(['a', 'b']);
    await w.find('input').trigger('keydown', { key: 'Backspace' });
    expect(lastEmit(w)).toEqual(['a']);
  });

  it('remove button emits without the item', async () => {
    const w = await mountChips(['a', 'b']);
    await w.find('button[aria-label="Удалить a"]').trigger('click');
    expect(lastEmit(w)).toEqual(['b']);
  });
});
