import { afterEach, describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { defineComponent, h, nextTick, ref } from 'vue';
import { Modal } from '~/components/ui/modal';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { Dialog } from '~/components/ui/dialog';
import { Sheet } from '~/components/ui/sheet';

const dialog = () => document.body.querySelector<HTMLElement>('[role="dialog"]');

const tab = (shift = false) => {
  const e = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: shift, bubbles: true, cancelable: true });
  (document.activeElement ?? document.body).dispatchEvent(e);
  return e;
};

afterEach(() => {
  document.body.innerHTML = '';
  document.body.style.overflow = '';
});

describe('ui/Modal', () => {
  it('renders an accessible dialog in body, locks scroll and closes on Esc', async () => {
    const w = await mountSuspended(Modal, {
      props: { open: true, title: 'Отклик' },
      slots: { default: () => h('input', { 'data-test': 'first' }) },
      attachTo: document.body,
    });
    await nextTick();
    await nextTick();
    const d = dialog();
    expect(d).not.toBeNull();
    expect(d!.getAttribute('aria-modal')).toBe('true');
    const labelId = d!.getAttribute('aria-labelledby');
    expect(document.getElementById(labelId!)?.textContent).toBe('Отклик');
    expect(document.body.style.overflow).toBe('hidden');
    expect(document.activeElement?.getAttribute('data-test')).toBe('first');

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(w.emitted('update:open')?.[0]).toEqual([false]);
    expect(w.emitted('close')).toHaveLength(1);
    w.unmount();
  });

  it('restores focus and scroll when closed', async () => {
    const trigger = document.createElement('button');
    document.body.appendChild(trigger);
    trigger.focus();

    const open = ref(true);
    const Host = defineComponent({
      render: () => h(Modal, { 'open': open.value, 'title': 'T', 'onUpdate:open': (v: boolean) => (open.value = v) }),
    });
    const w = await mountSuspended(Host);
    await nextTick();
    await nextTick();
    expect(document.activeElement).not.toBe(trigger);
    open.value = false;
    await nextTick();
    await nextTick();
    expect(dialog()).toBeNull();
    expect(document.activeElement).toBe(trigger);
    expect(document.body.style.overflow).toBe('');
    w.unmount();
  });

  it('restores focus to the pressed opener when the click did not focus it (Safari/iOS)', async () => {
    const trigger = document.createElement('button');
    document.body.appendChild(trigger);
    const open = ref(false);
    const Host = defineComponent({
      render: () => h(Modal, { 'open': open.value, 'title': 'T', 'onUpdate:open': (v: boolean) => (open.value = v) }),
    });
    const w = await mountSuspended(Host);
    await nextTick();
    // Press without focus (activeElement stays body), then open.
    trigger.dispatchEvent(new Event('pointerdown', { bubbles: true }));
    expect(document.activeElement).toBe(document.body);
    open.value = true;
    await nextTick();
    await nextTick();
    open.value = false;
    await nextTick();
    await nextTick();
    expect(document.activeElement).toBe(trigger);
    w.unmount();
  });

  it('does not close on Esc when not closable', async () => {
    const w = await mountSuspended(Modal, { props: { open: true, title: 'T', closable: false } });
    await nextTick();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(w.emitted('update:open')).toBeUndefined();
    w.unmount();
  });
});

describe('ui/Modal focus trap', () => {
  const mountWithInputs = async () => {
    const outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.appendChild(outside);
    const w = await mountSuspended(Modal, {
      props: { open: true, title: 'T' },
      slots: {
        default: () => [h('input', { 'data-test': 'a' }), h('input', { 'data-test': 'b' })],
        footer: () => h('button', { 'data-test': 'ok' }, 'OK'),
      },
      attachTo: document.body,
    });
    await nextTick();
    await nextTick();
    return { w, outside };
  };
  const focusedTest = () => document.activeElement?.getAttribute('data-test') ?? document.activeElement?.getAttribute('aria-label');

  it('Tab on the last element wraps to the first (close button)', async () => {
    const { w } = await mountWithInputs();
    document.body.querySelector<HTMLElement>('[data-test="ok"]')!.focus();
    const e = tab();
    expect(e.defaultPrevented).toBe(true);
    expect(focusedTest()).toBe('Закрыть');
    w.unmount();
  });

  it('Shift+Tab on the first element wraps to the last', async () => {
    const { w } = await mountWithInputs();
    document.body.querySelector<HTMLElement>('[data-modal-close]')!.focus();
    const e = tab(true);
    expect(e.defaultPrevented).toBe(true);
    expect(focusedTest()).toBe('ok');
    w.unmount();
  });

  it('Tab between inner elements is left to the browser', async () => {
    const { w } = await mountWithInputs();
    expect(focusedTest()).toBe('a');
    const e = tab();
    expect(e.defaultPrevented).toBe(false);
    w.unmount();
  });

  it('Shift+Tab from the panel itself goes to the last element', async () => {
    const { w } = await mountWithInputs();
    dialog()!.focus();
    tab(true);
    expect(focusedTest()).toBe('ok');
    w.unmount();
  });

  it('keeps focus on the panel when there is nothing focusable', async () => {
    const w = await mountSuspended(Modal, {
      props: { open: true, ariaLabel: 'Пусто', closable: false },
      slots: { default: () => h('p', 'text') },
      attachTo: document.body,
    });
    await nextTick();
    await nextTick();
    const d = dialog()!;
    expect(d.getAttribute('aria-label')).toBe('Пусто');
    expect(document.activeElement).toBe(d);
    const e = tab();
    expect(e.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(d);
    w.unmount();
  });

  it('only the topmost of stacked modals closes on Esc', async () => {
    const lower = await mountSuspended(Modal, { props: { open: true, title: 'Lower' }, attachTo: document.body });
    await nextTick();
    const upper = await mountSuspended(Modal, { props: { open: true, title: 'Upper' }, attachTo: document.body });
    await nextTick();
    await nextTick();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(upper.emitted('close')).toHaveLength(1);
    expect(lower.emitted('close')).toBeUndefined();
    upper.unmount();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(lower.emitted('close')).toHaveLength(1);
    lower.unmount();
  });
});

describe('ui/Dialog and ui/Sheet', () => {
  it.each([['Dialog', Dialog], ['Sheet', Sheet]] as const)('%s is a modal dialog that closes on Esc', async (_n, Cmp) => {
    const w = await mountSuspended(Cmp, { props: { open: true, title: 'Заголовок' }, attachTo: document.body });
    await nextTick();
    await nextTick();
    const d = dialog()!;
    expect(d.getAttribute('aria-modal')).toBe('true');
    expect(d.contains(document.activeElement)).toBe(true);
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(w.emitted('update:open')?.[0]).toEqual([false]);
    expect(w.emitted('close')).toHaveLength(1);
    w.unmount();
  });
});

describe('ui/ConfirmDialog', () => {
  it('closes on Esc (cancel) unless loading', async () => {
    const w = await mountSuspended(ConfirmDialog, { props: { open: true, title: 'Удалить?' }, attachTo: document.body });
    await nextTick();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(w.emitted('update:open')?.[0]).toEqual([false]);
    w.unmount();

    const busy = await mountSuspended(ConfirmDialog, { props: { open: true, title: 'Удалить?', loading: true }, attachTo: document.body });
    await nextTick();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(busy.emitted('update:open')).toBeUndefined();
    busy.unmount();
  });

  it('emits confirm and cancel', async () => {
    const w = await mountSuspended(ConfirmDialog, {
      props: { open: true, title: 'Закрыть публикацию?', tone: 'danger', confirmText: 'Закрыть' },
    });
    await nextTick();
    const buttons = Array.from(dialog()!.querySelectorAll('button'));
    // Initial focus is on «Отмена», not on the destructive action.
    expect(document.activeElement?.textContent).toBe('Отмена');
    buttons.find(b => b.textContent === 'Закрыть')!.click();
    expect(w.emitted('confirm')).toHaveLength(1);
    buttons.find(b => b.textContent === 'Отмена')!.click();
    expect(w.emitted('cancel')).toHaveLength(1);
    expect(w.emitted('update:open')?.[0]).toEqual([false]);
    w.unmount();
  });
});
