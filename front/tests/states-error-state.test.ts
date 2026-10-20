import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ErrorState } from '~/components/states/error-state';
import ErrorPage from '~/error';

describe('states/ErrorState', () => {
  it('title is h2 by default', async () => {
    const w = await mountSuspended(ErrorState, { props: { title: 'Не удалось' } });
    expect(w.find('h2').text()).toBe('Не удалось');
    expect(w.find('h1').exists()).toBe(false);
  });

  it('headingLevel 1 renders the title as the page h1', async () => {
    const w = await mountSuspended(ErrorState, { props: { title: 'Публикация не найдена', headingLevel: 1 } });
    expect(w.find('h1').text()).toBe('Публикация не найдена');
  });
});

describe('error page', () => {
  it('503 asks to reload in a minute', async () => {
    const w = await mountSuspended(ErrorPage, {
      props: { error: { statusCode: 503, statusMessage: 'Сервер временно недоступен' } as never },
    });
    expect(w.find('h1').text()).toBe('Сервер временно недоступен');
    expect(w.text()).toContain('Попробуйте обновить страницу через минуту.');
    expect(w.text()).not.toContain('Мы уже знаем о проблеме');
  });

  it('500 keeps the generic text', async () => {
    const w = await mountSuspended(ErrorPage, { props: { error: { statusCode: 500 } as never } });
    expect(w.text()).toContain('Мы уже знаем о проблеме');
  });
});
