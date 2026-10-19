import { afterEach, describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { defineComponent, h, nextTick, ref } from 'vue';
import { Modal } from '~/components/ui/modal';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';

const dialog = () => document.body.querySelector<HTMLElement>('[role="dialog"]');

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

describe('ui/ConfirmDialog', () => {
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
