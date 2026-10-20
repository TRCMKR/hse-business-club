import { afterEach, describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { defineComponent, h, nextTick, ref } from 'vue';
import { Tabs } from '~/components/ui/tabs';

const TABS = [
  { value: 'sent', label: 'Отправленные' },
  { value: 'received', label: 'Полученные', count: 2 },
  { value: 'archive', label: 'Архив' },
];

const mountHost = async (initial = 'sent', idPrefix: string | null = 'apps') => {
  const model = ref(initial);
  const Host = defineComponent({
    render: () => h(Tabs, {
      'modelValue': model.value,
      'tabs': TABS,
      'idPrefix': idPrefix ?? undefined,
      'ariaLabel': 'Отклики',
      'onUpdate:modelValue': (v: string) => (model.value = v),
    }),
  });
  const w = await mountSuspended(Host, { attachTo: document.body });
  const tabs = () => w.findAll('[role="tab"]');
  return { w, model, tabs };
};

afterEach(() => {
  document.body.innerHTML = '';
});

describe('ui/Tabs', () => {
  it('renders a labelled tablist with roving tabindex', async () => {
    const { w, tabs } = await mountHost('received');
    const list = w.find('[role="tablist"]');
    expect(list.attributes('aria-label')).toBe('Отклики');
    expect(tabs().map(t => t.attributes('aria-selected'))).toEqual(['false', 'true', 'false']);
    expect(tabs().map(t => t.attributes('tabindex'))).toEqual(['-1', '0', '-1']);
    expect(tabs()[1]!.text()).toContain('2');
    w.unmount();
  });

  it('sets aria-controls only on the selected tab', async () => {
    const { w, model, tabs } = await mountHost('sent');
    expect(tabs().map(t => t.attributes('id'))).toEqual(['apps-tab-sent', 'apps-tab-received', 'apps-tab-archive']);
    expect(tabs().map(t => t.attributes('aria-controls'))).toEqual(['apps-panel-sent', undefined, undefined]);
    model.value = 'archive';
    await nextTick();
    expect(tabs().map(t => t.attributes('aria-controls'))).toEqual([undefined, undefined, 'apps-panel-archive']);
    w.unmount();
  });

  it('omits ids and aria-controls without idPrefix', async () => {
    const { w, tabs } = await mountHost('sent', null);
    for (const t of tabs()) {
      expect(t.attributes('id')).toBeUndefined();
      expect(t.attributes('aria-controls')).toBeUndefined();
    }
    w.unmount();
  });

  it('moves selection and focus with ArrowRight/ArrowLeft (wrapping), Home and End', async () => {
    const { w, model, tabs } = await mountHost('sent');
    const press = async (key: string) => {
      await tabs().find(t => t.attributes('aria-selected') === 'true')!.trigger('keydown', { key });
      await nextTick();
    };
    const focused = () => (document.activeElement as HTMLElement | null)?.textContent;

    await press('ArrowRight');
    expect(model.value).toBe('received');
    expect(focused()).toContain('Полученные');

    await press('ArrowRight');
    await press('ArrowRight');
    expect(model.value).toBe('sent');

    await press('ArrowLeft');
    expect(model.value).toBe('archive');
    expect(focused()).toBe('Архив');

    await press('Home');
    expect(model.value).toBe('sent');
    expect(focused()).toBe('Отправленные');

    await press('End');
    expect(model.value).toBe('archive');

    await press('Enter');
    expect(model.value).toBe('archive');
    w.unmount();
  });

  it('click selects a tab; clicking the selected tab does not re-emit', async () => {
    const model = ref('sent');
    const w = await mountSuspended(Tabs, { props: { modelValue: model.value, tabs: TABS } });
    await w.findAll('[role="tab"]')[0]!.trigger('click');
    expect(w.emitted('update:modelValue')).toBeUndefined();
    await w.findAll('[role="tab"]')[2]!.trigger('click');
    expect(w.emitted('update:modelValue')?.[0]).toEqual(['archive']);
    w.unmount();
  });
});
