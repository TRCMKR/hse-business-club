import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { AxiosError, AxiosHeaders } from 'axios';
import { nextTick } from 'vue';
import { ApplySheet, APPLY_SUCCESS_MESSAGE } from '~/components/listing/apply-sheet';

const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('vue-sonner', () => ({ toast: toastMock }));

const DUPLICATE = 'Вы уже откликались на эту публикацию';

function apiError(status: number, message: string): AxiosError {
  const config = { headers: new AxiosHeaders() };
  return new AxiosError('Request failed', AxiosError.ERR_BAD_REQUEST, config, {}, {
    status,
    statusText: '',
    headers: {},
    config,
    data: { code: status === 409 ? 6 : 9, message, details: [] },
  });
}

const flush = async () => {
  for (let i = 0; i < 4; i++) {
    await nextTick();
  }
};

const textarea = () => document.body.querySelector<HTMLTextAreaElement>('textarea')!;
const submitButton = () => document.body.querySelector<HTMLButtonElement>('[data-testid="apply-submit"]')!;
const dialogText = () => document.body.querySelector('[role="dialog"]')?.textContent ?? '';

async function mountSheet(submit: (m: string) => Promise<unknown>) {
  const w = await mountSuspended(ApplySheet, {
    props: { open: true, listingTitle: 'FinTrack', submit },
    attachTo: document.body,
  });
  await flush();
  return w;
}

async function type(value: string) {
  const t = textarea();
  t.value = value;
  t.dispatchEvent(new Event('input'));
  await flush();
}

beforeEach(() => {
  toastMock.success.mockClear();
  toastMock.error.mockClear();
});

afterEach(() => {
  document.body.innerHTML = '';
  document.body.style.overflow = '';
});

describe('listing/ApplySheet', () => {
  it('renders a textarea with a 0 / 2000 counter', async () => {
    const w = await mountSheet(vi.fn());
    expect(textarea()).not.toBeNull();
    expect(dialogText()).toContain('0 / 2000');
    expect(dialogText()).toContain('FinTrack');
    w.unmount();
  });

  it('empty or whitespace message → validation error, no request', async () => {
    const submit = vi.fn();
    const w = await mountSheet(submit);
    submitButton().click();
    await flush();
    expect(dialogText()).toContain('Сообщение: от 1 до 2000 символов');

    await type('   \n  ');
    submitButton().click();
    await flush();
    expect(dialogText()).toContain('Сообщение: от 1 до 2000 символов');
    expect(submit).not.toHaveBeenCalled();
    w.unmount();
  });

  it('message over 2000 characters → validation error, no request', async () => {
    const submit = vi.fn();
    const w = await mountSheet(submit);
    await type('я'.repeat(2001));
    expect(dialogText()).toContain('2001 / 2000');
    submitButton().click();
    await flush();
    expect(dialogText()).toContain('Сообщение: от 1 до 2000 символов');
    expect(submit).not.toHaveBeenCalled();
    w.unmount();
  });

  it('blocks repeated submits while the request is running, then closes on success', async () => {
    let resolve!: () => void;
    const submit = vi.fn(() => new Promise<void>((r) => {
      resolve = r;
    }));
    const w = await mountSheet(submit);
    await type('  Привет! Готов помочь с бэкендом.  ');

    submitButton().click();
    await flush();
    submitButton().click();
    submitButton().click();
    await flush();
    expect(submit).toHaveBeenCalledTimes(1);
    expect(submit).toHaveBeenCalledWith('Привет! Готов помочь с бэкендом.');
    expect(submitButton().disabled).toBe(true);
    expect(submitButton().getAttribute('aria-busy')).toBe('true');

    resolve();
    await flush();
    expect(toastMock.success).toHaveBeenCalledWith(APPLY_SUCCESS_MESSAGE);
    expect(w.emitted('success')).toHaveLength(1);
    expect(w.emitted('update:open')?.at(-1)).toEqual([false]);
    w.unmount();
  });

  it('409 from the server → message under the field and a toast, sheet stays open', async () => {
    const submit = vi.fn(() => Promise.reject(apiError(409, DUPLICATE)));
    const w = await mountSheet(submit);
    await type('Хочу в команду');
    submitButton().click();
    await flush();

    expect(dialogText()).toContain(DUPLICATE);
    expect(textarea().getAttribute('aria-invalid')).toBe('true');
    expect(toastMock.error).toHaveBeenCalledWith(DUPLICATE);
    expect(w.emitted('success')).toBeUndefined();
    expect(w.emitted('update:open')).toBeUndefined();
    expect(w.emitted('failed')).toHaveLength(1);
    // The button is usable again (the user may edit and retry).
    expect(submitButton().disabled).toBe(false);
    w.unmount();
  });

  it('400 «Публикация закрыта» is shown the same way', async () => {
    const w = await mountSheet(() => Promise.reject(apiError(400, 'Публикация закрыта')));
    await type('Хочу в команду');
    submitButton().click();
    await flush();
    expect(dialogText()).toContain('Публикация закрыта');
    expect(toastMock.error).toHaveBeenCalledWith('Публикация закрыта');
    w.unmount();
  });
});
